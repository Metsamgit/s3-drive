// Package config charge la config depuis les variables d'environnement.
// En dev, SESSION_KEY est généré au boot si absent (warning).
package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

type Config struct {
	BindAddr        string
	SessionKey      []byte // 32 bytes for AES-256-GCM
	SessionIdleTTL  time.Duration
	SessionAbsTTL   time.Duration
	MaxUploadBytes  int64
	TLSCert         string // optional
	TLSKey          string // optional
	BehindProxy     bool   // trust X-Forwarded-For
	LogLevel        slog.Level
}

func Load() (*Config, error) {
	c := &Config{
		BindAddr:       env("BIND_ADDR", "127.0.0.1:8080"),
		SessionIdleTTL: 30 * time.Minute,
		SessionAbsTTL:  8 * time.Hour,
		MaxUploadBytes: 100 * 1024 * 1024, // 100 MB
		TLSCert:        os.Getenv("TLS_CERT"),
		TLSKey:         os.Getenv("TLS_KEY"),
		BehindProxy:    env("BEHIND_PROXY", "false") == "true",
		LogLevel:       slog.LevelInfo,
	}

	if v := os.Getenv("MAX_UPLOAD_MB"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("MAX_UPLOAD_MB: invalid value %q", v)
		}
		c.MaxUploadBytes = int64(n) * 1024 * 1024
	}

	if v := os.Getenv("LOG_LEVEL"); v != "" {
		switch v {
		case "debug":
			c.LogLevel = slog.LevelDebug
		case "info":
			c.LogLevel = slog.LevelInfo
		case "warn":
			c.LogLevel = slog.LevelWarn
		case "error":
			c.LogLevel = slog.LevelError
		default:
			return nil, fmt.Errorf("LOG_LEVEL: unknown level %q", v)
		}
	}

	key, err := loadOrGenerateSessionKey()
	if err != nil {
		return nil, err
	}
	c.SessionKey = key

	if (c.TLSCert == "") != (c.TLSKey == "") {
		return nil, errors.New("TLS_CERT and TLS_KEY must both be set or both empty")
	}

	return c, nil
}

func loadOrGenerateSessionKey() ([]byte, error) {
	raw := os.Getenv("SESSION_KEY")
	if raw == "" {
		k := make([]byte, 32)
		if _, err := rand.Read(k); err != nil {
			return nil, fmt.Errorf("generate session key: %w", err)
		}
		// Fallback dev: sans clé stable, les sessions sautent au restart.
		slog.Warn("SESSION_KEY absent, clé éphémère générée. Les sessions ne survivront pas à un restart.",
			"hint", "export SESSION_KEY="+base64.StdEncoding.EncodeToString(k))
		return k, nil
	}
	k, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("SESSION_KEY: must be base64: %w", err)
	}
	if len(k) != 32 {
		return nil, fmt.Errorf("SESSION_KEY: must decode to 32 bytes (got %d)", len(k))
	}
	return k, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
