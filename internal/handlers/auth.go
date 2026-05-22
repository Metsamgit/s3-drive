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
	// Déjà connecté: skip le form.
	if c, err := r.Cookie(auth.CookieName); err == nil {
		if _, ok := h.Store.Get(c.Value); ok {
			http.Redirect(w, r, "/files", http.StatusSeeOther)
			return
		}
	}
	// CSRF pré-login: token jetable, vérifié au POST.
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

	// Le token POSTé doit matcher le cookie pré-login posé sur le GET.
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

	// Message générique pour tout ce qui touche aux credentials, pour ne
	// pas révéler si la forme est bonne ou si c'est AWS qui rejette.
	const genericAuthErr = "Identifiants, région ou compte refusés."

	if !validation.AWSAccessKey(ak) || !validation.AWSSecretKey(sk) {
		renderErr(genericAuthErr)
		return
	}
	cleanRegion, err := validation.Region(region)
	if err != nil {
		renderErr(genericAuthErr)
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

	// Vérifie les creds côté AWS avant de créer la session.
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
			renderErr(genericAuthErr)
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

// --- CSRF pré-login ---

func newPreLoginCSRF() string {
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
		MaxAge:   600, // 10 min
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
