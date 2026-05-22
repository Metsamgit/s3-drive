package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/Metsamgit/s3-drive/internal/validation"
)

// PostBucket change le bucket courant après check d'accès.
func (h *Handler) PostBucket(w http.ResponseWriter, r *http.Request) {
	if !h.verifyCSRF(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form invalide", http.StatusBadRequest)
		return
	}
	name, err := validation.BucketName(r.FormValue("bucket"))
	if err != nil {
		http.Error(w, "nom de bucket invalide", http.StatusBadRequest)
		return
	}
	sess := sessionFrom(r.Context())

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	if err := h.s3(r).HeadBucket(ctx, name); err != nil {
		http.Error(w, "bucket inaccessible", http.StatusForbidden)
		return
	}

	h.Store.SetBucket(sess.ID, name)
	http.Redirect(w, r, "/files", http.StatusSeeOther)
}
