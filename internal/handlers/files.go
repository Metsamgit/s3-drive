package handlers

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/Metsamgit/s3-drive/internal/awsclient"
	"github.com/Metsamgit/s3-drive/internal/validation"
)

type viewFolder struct {
	Name string
	Path string
}

type viewFile struct {
	Key      string
	Name     string
	Kind     string
	Size     string
	Modified string
}

type filesData struct {
	Bucket  string
	Prefix  string
	Crumbs  []viewFolder
	Folders []viewFolder
	Files   []viewFile
	Flash   string
	Error   string
	CSRF    string

	// Bonus 1: if ListBuckets succeeded at probe time, the dropdown is
	// populated. If the IAM lacks s3:ListAllMyBuckets we just hide it.
	CanList bool
	Buckets []awsclient.Bucket
}

func (h *Handler) GetFiles(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	bucket := sess.Bucket
	prefix := r.URL.Query().Get("prefix")

	if cleaned, err := validation.Prefix(prefix); err != nil {
		http.Error(w, "préfixe invalide", http.StatusBadRequest)
		return
	} else {
		prefix = cleaned
	}

	data := filesData{
		Bucket: bucket,
		Prefix: prefix,
		CSRF:   sess.CSRFToken,
	}

	// Best-effort list of accessible buckets; tolerate AccessDenied.
	cli := h.s3(r)
	listCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if buckets, err := cli.ListBuckets(listCtx); err == nil {
		data.CanList = true
		data.Buckets = buckets
	}

	if bucket != "" {
		listCtx2, cancel2 := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel2()
		res, err := cli.ListPrefix(listCtx2, bucket, prefix)
		if err != nil {
			data.Error = err.Error()
		} else {
			data.Folders = renderFolders(prefix, res.Folders)
			data.Files = renderFiles(prefix, res.Files)
		}
		data.Crumbs = renderCrumbs(prefix)
	}

	_ = h.tmpl.renderFiles(w, data)
}

// GetFilesList serves the table-only HTML for HTMX swaps (e.g. after a
// delete that triggers `hx-target="#file-list"`).
func (h *Handler) GetFilesList(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	if sess.Bucket == "" {
		http.Error(w, "aucun bucket", http.StatusBadRequest)
		return
	}
	prefix, err := validation.Prefix(r.URL.Query().Get("prefix"))
	if err != nil {
		http.Error(w, "préfixe invalide", http.StatusBadRequest)
		return
	}

	cli := h.s3(r)
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	res, err := cli.ListPrefix(ctx, sess.Bucket, prefix)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	data := filesData{
		Prefix:  prefix,
		Folders: renderFolders(prefix, res.Folders),
		Files:   renderFiles(prefix, res.Files),
		CSRF:    sess.CSRFToken,
	}
	_ = h.tmpl.renderFileList(w, data)
}

func renderCrumbs(prefix string) []viewFolder {
	if prefix == "" {
		return nil
	}
	parts := strings.Split(strings.TrimSuffix(prefix, "/"), "/")
	out := make([]viewFolder, 0, len(parts))
	acc := ""
	for _, p := range parts {
		acc += p + "/"
		out = append(out, viewFolder{Name: p, Path: acc})
	}
	return out
}

func renderFolders(prefix string, prefs []string) []viewFolder {
	out := make([]viewFolder, 0, len(prefs))
	for _, p := range prefs {
		name := strings.TrimSuffix(strings.TrimPrefix(p, prefix), "/")
		out = append(out, viewFolder{Name: name, Path: p})
	}
	return out
}

func renderFiles(prefix string, files []awsclient.Object) []viewFile {
	out := make([]viewFile, 0, len(files))
	for _, f := range files {
		out = append(out, viewFile{
			Key:      f.Key,
			Name:     strings.TrimPrefix(f.Key, prefix),
			Kind:     kindForName(f.Key),
			Size:     fmtSize(f.Size),
			Modified: f.LastModified.Format("02/01/2006 15:04"),
		})
	}
	return out
}

func kindForName(name string) string {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))
	switch ext {
	case "jpg", "jpeg", "png", "gif", "webp", "svg", "bmp":
		return "IMG"
	case "pdf":
		return "PDF"
	case "doc", "docx", "txt", "md", "rtf":
		return "DOC"
	case "xls", "xlsx", "csv":
		return "XLS"
	case "zip", "tar", "gz", "rar", "7z":
		return "ZIP"
	case "mp4", "mov", "avi", "mkv", "webm":
		return "VID"
	case "mp3", "wav", "ogg", "flac":
		return "MP3"
	case "js", "ts", "py", "go", "rs", "java", "c", "cpp", "h", "json", "html", "css":
		return "COD"
	}
	return "FIL"
}

func fmtSize(n int64) string {
	const k = 1024
	switch {
	case n < k:
		return fmt.Sprintf("%d B", n)
	case n < k*k:
		return fmt.Sprintf("%.1f KB", float64(n)/k)
	case n < k*k*k:
		return fmt.Sprintf("%.1f MB", float64(n)/k/k)
	default:
		return fmt.Sprintf("%.1f GB", float64(n)/k/k/k)
	}
}
