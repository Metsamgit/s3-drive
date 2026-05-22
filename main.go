// s3-drive: petite app web pour gérer des fichiers dans un bucket S3.
package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Metsamgit/s3-drive/internal/auth"
	"github.com/Metsamgit/s3-drive/internal/config"
	"github.com/Metsamgit/s3-drive/internal/handlers"
	"github.com/Metsamgit/s3-drive/internal/middleware"
)

//go:embed web/templates web/static
var assets embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})))

	store, err := auth.NewStore(cfg.SessionKey, cfg.SessionIdleTTL, cfg.SessionAbsTTL)
	if err != nil {
		slog.Error("session store", "err", err)
		os.Exit(1)
	}

	h, err := handlers.New(cfg, store, assets)
	if err != nil {
		slog.Error("handlers", "err", err)
		os.Exit(1)
	}

	staticFS, err := fs.Sub(assets, "web/static")
	if err != nil {
		slog.Error("static fs", "err", err)
		os.Exit(1)
	}
	static := http.FileServer(http.FS(staticFS))

	mux := h.Routes(static)

	// /login: limite stricte pour ralentir le brute force.
	authLimit := middleware.NewIPLimiter(0.1, 5, 30*time.Minute) // ~6/min
	apiLimit := middleware.NewIPLimiter(2.0, 30, 30*time.Minute) // 2/s, burst 30

	root := http.NewServeMux()
	root.Handle("/login", authLimit.Middleware(mux))
	root.Handle("/", apiLimit.Middleware(mux))

	chain := middleware.Recover(
		middleware.Logging(
			middleware.SecurityHeaders(
				middleware.NoCache(root),
			),
		),
	)

	srv := &http.Server{
		Addr:              cfg.BindAddr,
		Handler:           chain,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       5 * time.Minute, // uploads longs
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 16, // 64 KB
	}

	// Shutdown propre: laisse les uploads/downloads en cours finir.
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs
		slog.Info("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	slog.Info("listening", "addr", cfg.BindAddr, "tls", cfg.TLSCert != "")
	if cfg.TLSCert != "" {
		err = srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
	} else {
		err = srv.ListenAndServe()
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}
