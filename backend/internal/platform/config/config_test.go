package config

import "testing"

func TestProductionFailsClosed(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SESSION_PEPPER", "12345678901234567890123456789012")
	t.Setenv("OTP_PEPPER", "12345678901234567890123456789012")
	t.Setenv("MAIL_MODE", "smtp")
	t.Setenv("TURNSTILE_SECRET", "real-secret")
	t.Setenv("PUBLIC_ORIGIN", "http://example.com")
	if _, err := Load(); err == nil {
		t.Fatal("production accepted HTTP and missing SMTP credentials")
	}
	t.Setenv("PUBLIC_ORIGIN", "https://example.com")
	t.Setenv("SMTP_ADDRESS", "smtp.example.com:587")
	t.Setenv("SMTP_FROM", "noreply@example.com")
	t.Setenv("SMTP_USERNAME", "user")
	t.Setenv("SMTP_PASSWORD", "secret")
	if _, err := Load(); err != nil {
		t.Fatalf("valid production configuration rejected: %v", err)
	}
}
