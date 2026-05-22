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

// GetDownload streame l'objet via le serveur (pas d'URL pré-signée).
// On force Content-Disposition: attachment pour qu'un .html dans le
// bucket ne soit jamais rendu inline sous notre origine.
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

	// Force le download: rien n'est jamais rendu inline.
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

// sanitizeFilename retire les guillemets et bytes de contrôle pour
// éviter d'injecter des tokens supplémentaires dans Content-Disposition.
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
