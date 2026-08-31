package app

import (
	"database/sql"
	"log/slog"
	"net/http"
	"rwandafreespace.com/blog/backend/internal/api"
	"rwandafreespace.com/blog/backend/internal/auth"
	"rwandafreespace.com/blog/backend/internal/editorial"
	"rwandafreespace.com/blog/backend/internal/platform/config"
	"rwandafreespace.com/blog/backend/internal/platform/httpx"
)

func New(cfg config.Config, db *sql.DB, logger *slog.Logger) http.Handler {
	mailer := auth.Mailer(auth.TerminalMailer{})
	if cfg.MailMode == "smtp" {
		mailer = auth.SMTPMailer{Address: cfg.SMTPAddress, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword, From: cfg.SMTPFrom}
	}
	authService := &auth.Service{DB: db, SessionPepper: cfg.SessionPepper, OTPPepper: cfg.OTPPepper, Mailer: mailer, Turnstile: auth.SiteVerifier{Secret: cfg.TurnstileSecret, Client: &http.Client{Timeout: 5e9}}, SecureCookies: cfg.Environment == "production"}
	handler := (&api.API{DB: db, Auth: authService, Editorial: &editorial.Service{DB: db}, Logger: logger, Origin: cfg.PublicOrigin, MediaDir: cfg.MediaDir, MailMode: cfg.MailMode, Development: cfg.Environment != "production"}).Routes()
	return httpx.Chain(handler, httpx.RequestID, httpx.SecurityHeaders, httpx.CORS(cfg.AllowedOrigins), httpx.Recover(logger), httpx.Log(logger))
}
func NewServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{Addr: cfg.Address, Handler: handler, ReadHeaderTimeout: cfg.ReadHeaderTimeout, ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout, IdleTimeout: cfg.IdleTimeout}
}
