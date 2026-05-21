// Package handlers wires HTTP routes for the S3 Drive backend.
package handlers

import (
	"context"
	"embed"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Metsamgit/s3-drive/internal/auth"
	"github.com/Metsamgit/s3-drive/internal/awsclient"
	"github.com/Metsamgit/s3-drive/internal/config"
)

type Handler struct {
	Cfg   *config.Config
	Store *auth.Store
	tmpl  *templates
}

func New(cfg *config.Config, store *auth.Store, assets embed.FS) (*Handler, error) {
	tmpl, err := loadTemplates(assets)
	if err != nil {
		return nil, err
	}
	return &Handler{Cfg: cfg, Store: store, tmpl: tmpl}, nil
}

// ctxKey is a private type for context keys to avoid collisions across pkgs.
type ctxKey int

const (
	ctxSession ctxKey = iota
)

func sessionFrom(ctx context.Context) *auth.Session {
	s, _ := ctx.Value(ctxSession).(*auth.Session)
	return s
}

// requireSession is a middleware that 302s to /login if no valid session.
func (h *Handler) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(auth.CookieName)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		sess, ok := h.Store.Get(c.Value)
		if !ok {
			auth.ClearCookie(w, h.secure(r))
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		h.Store.Touch(sess.ID)
		ctx := context.WithValue(r.Context(), ctxSession, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// verifyCSRF returns false and writes 403 if the CSRF token is missing or
// doesn't match the session's. Called on every state-changing handler.
func (h *Handler) verifyCSRF(w http.ResponseWriter, r *http.Request) bool {
	sess := sessionFrom(r.Context())
	if sess == nil {
		http.Error(w, "session manquante", http.StatusForbidden)
		return false
	}
	token := r.FormValue("csrf")
	if !h.Store.VerifyCSRF(sess.ID, token) {
		http.Error(w, "jeton CSRF invalide", http.StatusForbidden)
		return false
	}
	return true
}

// secure reports whether the cookie should have the Secure flag.
// True for direct TLS or when behind a trusted proxy forwarding HTTPS.
func (h *Handler) secure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if h.Cfg.BehindProxy && r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}
	return false
}

// s3 returns a fresh AWS client built from the request's session creds.
// We build per request to keep credential exposure as short-lived as
// possible.
func (h *Handler) s3(r *http.Request) *awsclient.Client {
	sess := sessionFrom(r.Context())
	if sess == nil {
		return nil
	}
	return awsclient.New(sess.Creds)
}

// Routes wires every endpoint into a chi router. The unauthenticated set
// is small on purpose (login + static) so it's easy to reason about.
func (h *Handler) Routes(static http.Handler) chi.Router {
	r := chi.NewRouter()

	// public
	r.Get("/login", h.GetLogin)
	r.Post("/login", h.PostLogin)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Handle("/static/*", http.StripPrefix("/static/", static))

	// authenticated
	r.Group(func(r chi.Router) {
		r.Use(h.requireSession)
		r.Get("/", h.GetRoot)
		r.Get("/files", h.GetFiles)
		r.Get("/files/list", h.GetFilesList)
		r.Post("/bucket", h.PostBucket)
		r.Post("/upload", h.PostUpload)
		r.Get("/download", h.GetDownload)
		r.Post("/delete", h.PostDelete)
		r.Post("/folder", h.PostFolder)
		r.Post("/logout", h.PostLogout)
	})

	return r
}
