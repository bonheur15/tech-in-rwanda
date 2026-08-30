// Package config reads and validates runtime configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Address          string
	StaticDir        string
	AllowedOrigins   map[string]struct{}
	ShutdownTimeout  time.Duration
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	IdleTimeout      time.Duration
	ReadHeaderTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Address:           envOr("APP_ADDR", ":8080"),
		StaticDir:         strings.TrimSpace(os.Getenv("STATIC_DIR")),
		AllowedOrigins:    parseSet(envOr("APP_ALLOWED_ORIGINS", "http://localhost:4321,http://127.0.0.1:4321")),
		ShutdownTimeout:   10 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if !strings.Contains(cfg.Address, ":") {
		return Config{}, fmt.Errorf("APP_ADDR must contain a port: %q", cfg.Address)
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func parseSet(value string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result[strings.TrimRight(item, "/")] = struct{}{}
		}
	}
	return result
}
