// Package handlers: routes HTTP de l'app.
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

// ctxKey évite les collisions de clés de context entre packages.
type ctxKey int

const (
	ctxSession ctxKey = iota
)

func sessionFrom(ctx context.Context) *auth.Session {
	s, _ := ctx.Value(ctxSession).(*auth.Session)
	return s
}

// requireSession redirige vers /login si pas de session valide.
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

// verifyCSRF renvoie false (+403) si le token est absent ou faux.
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

// secure indique si on doit poser le flag Secure sur les cookies.
func (h *Handler) secure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if h.Cfg.BehindProxy && r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}
	return false
}

// s3 renvoie un client AWS construit à partir des credentials de session.
// Construit par requête pour limiter la durée de vie des creds en mémoire.
func (h *Handler) s3(r *http.Request) *awsclient.Client {
	sess := sessionFrom(r.Context())
	if sess == nil {
		return nil
	}
	return awsclient.New(sess.Creds)
}

// Routes monte les routes dans chi. Le set non-authentifié est volontairement
// court (login + static).
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
