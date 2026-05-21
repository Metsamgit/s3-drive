package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Metsamgit/s3-drive/internal/validation"
)

func (h *Handler) PostDelete(w http.ResponseWriter, r *http.Request) {
	if !h.verifyCSRF(w, r) {
		return
	}
	sess := sessionFrom(r.Context())
	if sess.Bucket == "" {
		http.Error(w, "aucun bucket", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form invalide", http.StatusBadRequest)
		return
	}
	key, err := validation.S3Key(r.FormValue("key"))
	if err != nil {
		http.Error(w, "clé invalide", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := h.s3(r).DeleteObject(ctx, sess.Bucket, key); err != nil {
		slog.Warn("delete", "key", key, "err", err)
		http.Error(w, "erreur S3", http.StatusBadGateway)
		return
	}

	// HTMX vs normal: HTMX expects the new partial back, browsers want a
	// redirect to refresh the page.
	if r.Header.Get("HX-Request") == "true" {
		prefix, _ := validation.Prefix(r.FormValue("prefix"))
		res, err := h.s3(r).ListPrefix(ctx, sess.Bucket, prefix)
		if err != nil {
			http.Error(w, "erreur S3", http.StatusBadGateway)
			return
		}
		_ = h.tmpl.renderFileList(w, filesData{
			Prefix:  prefix,
			Folders: renderFolders(prefix, res.Folders),
			Files:   renderFiles(prefix, res.Files),
			CSRF:    sess.CSRFToken,
		})
		return
	}

	prefix := r.FormValue("prefix")
	http.Redirect(w, r, "/files?prefix="+strings.TrimPrefix(prefix, "/"), http.StatusSeeOther)
}
