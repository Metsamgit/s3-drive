package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover attrape les panics des handlers et renvoie un 500 générique.
// La stack reste dans les logs serveur.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered",
					"path", r.URL.Path,
					"method", r.Method,
					"err", rec,
					"stack", string(debug.Stack()))
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
