// Package config reads and validates runtime configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Address, DatabasePath, MediaDir, TempDir          string
	PublicOrigin, SessionPepper, OTPPepper            string
	TurnstileSecret, TurnstileSiteKey                 string
	Environment, MailMode                             string
	SMTPAddress, SMTPUsername, SMTPPassword, SMTPFrom string
	AllowedOrigins                                    map[string]struct{}
	ShutdownTimeout                                   time.Duration
	ReadTimeout                                       time.Duration
	WriteTimeout                                      time.Duration
	IdleTimeout                                       time.Duration
	ReadHeaderTimeout                                 time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Address: envOr("APP_ADDR", "127.0.0.1:8081"), DatabasePath: envOr("DATABASE_PATH", ".data/blog.sqlite3"),
		MediaDir: envOr("MEDIA_DIR", ".data/media"), TempDir: envOr("UPLOAD_TEMP_DIR", ".data/tmp"),
		PublicOrigin:  strings.TrimRight(envOr("PUBLIC_ORIGIN", "http://127.0.0.1:4321"), "/"),
		SessionPepper: os.Getenv("SESSION_PEPPER"), OTPPepper: os.Getenv("OTP_PEPPER"),
		TurnstileSecret: envOr("TURNSTILE_SECRET", "1x0000000000000000000000000000000AA"), TurnstileSiteKey: envOr("PUBLIC_TURNSTILE_SITE_KEY", "1x00000000000000000000AA"),
		Environment: envOr("APP_ENV", "development"), MailMode: envOr("MAIL_MODE", "terminal"),
		SMTPAddress: os.Getenv("SMTP_ADDRESS"), SMTPUsername: os.Getenv("SMTP_USERNAME"), SMTPPassword: os.Getenv("SMTP_PASSWORD"), SMTPFrom: os.Getenv("SMTP_FROM"),
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
	if cfg.Environment == "production" {
		if len(cfg.SessionPepper) < 32 || len(cfg.OTPPepper) < 32 {
			return Config{}, fmt.Errorf("production peppers must each be at least 32 characters")
		}
		if cfg.MailMode != "smtp" {
			return Config{}, fmt.Errorf("production requires MAIL_MODE=smtp")
		}
		if cfg.SMTPAddress == "" || cfg.SMTPFrom == "" || cfg.SMTPUsername == "" || cfg.SMTPPassword == "" {
			return Config{}, fmt.Errorf("production requires complete SMTP configuration")
		}
		if !strings.HasPrefix(cfg.PublicOrigin, "https://") {
			return Config{}, fmt.Errorf("production PUBLIC_ORIGIN must use HTTPS")
		}
		if cfg.TurnstileSecret == "" || cfg.TurnstileSecret == "1x0000000000000000000000000000000AA" {
			return Config{}, fmt.Errorf("production requires a non-test TURNSTILE_SECRET")
		}
	}
	if cfg.SessionPepper == "" {
		cfg.SessionPepper = "development-session-pepper-not-for-production"
	}
	if cfg.OTPPepper == "" {
		cfg.OTPPepper = "development-otp-pepper-not-for-production"
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
