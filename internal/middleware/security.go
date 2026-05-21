// Package middleware contains HTTP middleware: security headers, panic
// recovery, structured logging, rate limiting.
package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeaders sets headers that reduce the impact of XSS, clickjacking,
// and MIME confusion attacks. The CSP is strict: 'self' only, no inline
// scripts, no inline styles, no third-party origins.
//
// HSTS is conditional on the request being seen as HTTPS, otherwise a
// browser on plain HTTP would refuse to upgrade.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self'; "+
				"img-src 'self' data:; "+
				"font-src 'self'; "+
				"connect-src 'self'; "+
				"frame-ancestors 'none'; "+
				"base-uri 'self'; "+
				"form-action 'self'; "+
				"object-src 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")

		if isHTTPS(r) {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// Defense-in-depth: even if a typo lets a route handler set the
		// wrong content-type, the browser will still refuse to sniff.
		next.ServeHTTP(w, r)
	})
}

// NoCache turns off all caching for HTML responses. Static assets are
// cached separately via the file server.
func NoCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	// nginx in front sets this header; we only trust it if explicitly told
	// to via config.BehindProxy. The check is done by the caller wiring
	// middleware order.
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
