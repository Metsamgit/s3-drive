package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Metsamgit/s3-drive/internal/validation"
)

// PostFolder creates a zero-byte "folder" object.
func (h *Handler) PostFolder(w http.ResponseWriter, r *http.Request) {
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

	prefix, err := validation.Prefix(r.FormValue("prefix"))
	if err != nil {
		http.Error(w, "préfixe invalide", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	// Don't let users smuggle slashes through the folder name — they
	// could create arbitrary nesting and confuse the breadcrumb code.
	if name == "" || strings.ContainsAny(name, "/\\") {
		http.Error(w, "nom de dossier invalide", http.StatusBadRequest)
		return
	}
	key := prefix + name + "/"
	if _, err := validation.S3Key(key); err != nil {
		http.Error(w, "nom de dossier invalide", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := h.s3(r).CreateEmptyFolder(ctx, sess.Bucket, key); err != nil {
		http.Error(w, "erreur S3", http.StatusBadGateway)
		return
	}
	http.Redirect(w, r, "/files?prefix="+prefix, http.StatusSeeOther)
}
