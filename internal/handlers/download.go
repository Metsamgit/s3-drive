package handlers

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/Metsamgit/s3-drive/internal/validation"
)

// GetDownload streams the requested object back through the server. This
// is intentionally more conservative than emitting a presigned URL: it
// keeps the bucket hostname (and indirectly the AWS account) hidden, and
// it lets us force a Content-Disposition: attachment so a HTML file from
// a user's bucket can never be rendered as if it lived on our origin.
func (h *Handler) GetDownload(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	if sess.Bucket == "" {
		http.Error(w, "aucun bucket", http.StatusBadRequest)
		return
	}
	key, err := validation.S3Key(r.URL.Query().Get("key"))
	if err != nil {
		http.Error(w, "clé invalide", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	body, meta, err := h.s3(r).DownloadStream(ctx, sess.Bucket, key)
	if err != nil {
		slog.Warn("download", "key", key, "err", err)
		http.Error(w, "objet introuvable", http.StatusNotFound)
		return
	}
	defer body.Close()

	// Force download: never render directly in the browser, even for
	// images or PDFs. The CSP would block most active content anyway,
	// but a forced attachment is one less thing to worry about.
	w.Header().Set("Content-Disposition", `attachment; filename="`+sanitizeFilename(path.Base(key))+`"`)
	w.Header().Set("Content-Type", "application/octet-stream")
	if meta.ContentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(meta.ContentLength, 10))
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if _, err := io.Copy(w, body); err != nil {
		slog.Warn("download copy", "err", err)
	}
}

// sanitizeFilename strips any quote or control char from the filename
// before putting it into Content-Disposition. Without this an attacker
// who controlled an object name could inject extra header tokens.
func sanitizeFilename(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c == '"' || c == '\\' {
			continue
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return "fichier"
	}
	return string(out)
}
