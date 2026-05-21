package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Metsamgit/s3-drive/internal/auth"
	"github.com/Metsamgit/s3-drive/internal/awsclient"
	"github.com/Metsamgit/s3-drive/internal/validation"
)

var awsRegions = []string{
	"eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1", "eu-north-1", "eu-south-1",
	"us-east-1", "us-east-2", "us-west-1", "us-west-2",
	"ap-northeast-1", "ap-northeast-2", "ap-northeast-3",
	"ap-southeast-1", "ap-southeast-2", "ap-south-1",
	"sa-east-1", "ca-central-1", "me-south-1", "af-south-1",
}

type loginData struct {
	Error          string
	Regions        []string
	SelectedRegion string
	CSRF           string
}

func (h *Handler) GetLogin(w http.ResponseWriter, r *http.Request) {
	// Already-logged-in users skip the form.
	if c, err := r.Cookie(auth.CookieName); err == nil {
		if _, ok := h.Store.Get(c.Value); ok {
			http.Redirect(w, r, "/files", http.StatusSeeOther)
			return
		}
	}
	// We render with a fresh CSRF that we'll bind to the new session
	// created on POST. This means the login form has a one-shot token.
	csrf := newPreLoginCSRF()
	setPreLoginCSRFCookie(w, csrf, h.secure(r))

	_ = h.tmpl.renderLogin(w, loginData{
		Regions:        awsRegions,
		SelectedRegion: "eu-west-3",
		CSRF:           csrf,
	})
}

const preLoginCookieName = "pre_csrf"

func (h *Handler) PostLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form invalide", http.StatusBadRequest)
		return
	}

	// Pre-login CSRF: the token submitted must match the cookie we set
	// on the GET. Defends against an attacker submitting a login form
	// from another origin.
	cookie, err := r.Cookie(preLoginCookieName)
	if err != nil || cookie.Value == "" || cookie.Value != r.FormValue("csrf") {
		http.Error(w, "jeton CSRF invalide", http.StatusForbidden)
		return
	}

	ak := r.FormValue("access_key")
	sk := r.FormValue("secret_key")
	region := r.FormValue("region")
	bucket := r.FormValue("bucket")

	renderErr := func(msg string) {
		csrf := newPreLoginCSRF()
		setPreLoginCSRFCookie(w, csrf, h.secure(r))
		w.WriteHeader(http.StatusBadRequest)
		_ = h.tmpl.renderLogin(w, loginData{
			Error:          msg,
			Regions:        awsRegions,
			SelectedRegion: region,
			CSRF:           csrf,
		})
	}

	if !validation.AWSAccessKey(ak) || !validation.AWSSecretKey(sk) {
		renderErr("Identifiants invalides.")
		return
	}
	cleanRegion, err := validation.Region(region)
	if err != nil {
		renderErr("Région invalide.")
		return
	}
	var cleanBucket string
	if bucket != "" {
		cleanBucket, err = validation.BucketName(bucket)
		if err != nil {
			renderErr("Nom de bucket invalide.")
			return
		}
	}

	creds := auth.Creds{
		AccessKeyID:     ak,
		SecretAccessKey: sk,
		Region:          cleanRegion,
	}

	// Probe AWS to validate creds before creating a session. We hit
	// HeadBucket if a bucket was provided, otherwise rely on ListBuckets
	// — both fail loudly on bad creds.
	probeCtx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	cli := awsclient.New(creds)
	if cleanBucket != "" {
		if err := cli.HeadBucket(probeCtx, cleanBucket); err != nil {
			renderErr("Bucket inaccessible. Vérifie le nom, les droits IAM ou la région.")
			return
		}
	} else {
		if _, err := cli.ListBuckets(probeCtx); err != nil {
			renderErr("Identifiants AWS refusés.")
			return
		}
	}

	sess, err := h.Store.Create(creds)
	if err != nil {
		slog.Error("session create", "err", err)
		http.Error(w, "erreur interne", http.StatusInternalServerError)
		return
	}
	if cleanBucket != "" {
		h.Store.SetBucket(sess.ID, cleanBucket)
	}

	clearPreLoginCSRFCookie(w, h.secure(r))
	auth.SetCookie(w, sess.ID, h.secure(r), h.Cfg.SessionIdleTTL)

	slog.Info("login ok", "ip", r.RemoteAddr, "region", cleanRegion)
	http.Redirect(w, r, "/files", http.StatusSeeOther)
}

func (h *Handler) PostLogout(w http.ResponseWriter, r *http.Request) {
	if !h.verifyCSRF(w, r) {
		return
	}
	sess := sessionFrom(r.Context())
	if sess != nil {
		h.Store.Destroy(sess.ID)
	}
	auth.ClearCookie(w, h.secure(r))
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) GetRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/files", http.StatusSeeOther)
}

// --- pre-login CSRF helpers ---------------------------------------------

func newPreLoginCSRF() string {
	// Reuse the same source of entropy as session IDs.
	tok, _ := auth.NewRandomToken(32)
	return tok
}

func setPreLoginCSRFCookie(w http.ResponseWriter, val string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     preLoginCookieName,
		Value:    val,
		Path:     "/login",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   600, // 10 min: just long enough to fill the form
	})
}

func clearPreLoginCSRFCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     preLoginCookieName,
		Value:    "",
		Path:     "/login",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}
