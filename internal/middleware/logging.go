package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder wraps ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Logging emits a structured access log entry per request.
// We deliberately do NOT log the body, query string, or any cookie. The
// path + method + status + duration is enough to spot anomalies without
// leaking sensitive data into logs.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"dur_ms", time.Since(start).Milliseconds(),
			"ip", clientIP(r))
	})
}

// clientIP returns the best-effort client IP. We don't trust XFF unless
// we're behind a proxy we control; the caller sets that flag.
func clientIP(r *http.Request) string {
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}
