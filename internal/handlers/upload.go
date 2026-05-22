package handlers

import (
	"context"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Metsamgit/s3-drive/internal/validation"
)

// PostUpload reçoit un upload multipart. Le body est borné à
// MaxUploadBytes via MaxBytesReader pour éviter qu'un client envoie
// un body infini.
func (h *Handler) PostUpload(w http.ResponseWriter, r *http.Request) {
	if !h.verifyCSRF(w, r) {
		return
	}
	sess := sessionFrom(r.Context())
	if sess.Bucket == "" {
		http.Error(w, "aucun bucket sélectionné", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.Cfg.MaxUploadBytes)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		http.Error(w, "upload trop volumineux ou invalide", http.StatusRequestEntityTooLarge)
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	prefix, err := validation.Prefix(r.FormValue("prefix"))
	if err != nil {
		http.Error(w, "préfixe invalide", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		http.Error(w, "aucun fichier", http.StatusBadRequest)
		return
	}

	cli := h.s3(r)
	bucket := sess.Bucket
	uploadCtx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	for _, fh := range files {
		// Le filename d'une part multipart est contrôlé par le client.
		// On garde uniquement le base name et on revalide.
		base := filepath.Base(fh.Filename)
		key := prefix + base
		if _, err := validation.S3Key(key); err != nil {
			http.Error(w, "nom de fichier invalide: "+base, http.StatusBadRequest)
			return
		}

		f, err := fh.Open()
		if err != nil {
			http.Error(w, "lecture du fichier impossible", http.StatusInternalServerError)
			return
		}

		ct := guessContentType(base, fh.Header.Get("Content-Type"))
		if err := cli.Upload(uploadCtx, bucket, key, ct, f); err != nil {
			_ = f.Close()
			slog.Error("upload", "key", key, "err", err)
			http.Error(w, "erreur S3", http.StatusBadGateway)
			return
		}
		_ = f.Close()
	}

	http.Redirect(w, r, "/files?prefix="+prefix, http.StatusSeeOther)
}

// guessContentType déduit le content-type de l'extension. On n'utilise
// pas celui fourni par le client (un attaquant pourrait mettre text/html).
func guessContentType(name, clientCT string) string {
	if ct := mime.TypeByExtension(filepath.Ext(name)); ct != "" {
		return ct
	}
	if clientCT != "" && !strings.Contains(clientCT, "html") && !strings.Contains(clientCT, "script") {
		return clientCT
	}
	return "application/octet-stream"
}
