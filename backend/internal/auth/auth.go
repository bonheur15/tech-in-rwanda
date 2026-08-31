package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
)

const StaffCookie = "__Host-rfs_staff_session"
const ReaderCookie = "__Host-rfs_reader_session"
const StaffCSRFCookie = "__Host-rfs_staff_csrf"
const ReaderCSRFCookie = "__Host-rfs_reader_csrf"

var ErrUnauthorized = errors.New("unauthorized")
var ErrDelivery = errors.New("OTP delivery failed")

type Actor struct{ IdentityID, Kind, Role, PublishMode, Status, CSRF string }
type Mailer interface {
	SendOTP(context.Context, string, string) error
}
type TerminalMailer struct{}

func (TerminalMailer) SendOTP(_ context.Context, email, code string) error {
	fmt.Printf("DEV OTP for %s: %s\n", email, code)
	return nil
}

type SMTPMailer struct{ Address, Username, Password, From string }

func (m SMTPMailer) SendOTP(_ context.Context, to, code string) error {
	host, _, err := net.SplitHostPort(m.Address)
	if err != nil {
		return err
	}
	msg := []byte("To: " + to + "\r\nFrom: " + m.From + "\r\nSubject: Rwanda Free Space sign-in code\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nYour sign-in code is " + code + ". It expires in 10 minutes.\r\n")
	return smtp.SendMail(m.Address, smtp.PlainAuth("", m.Username, m.Password, host), m.From, []string{to}, msg)
}

type Turnstile interface {
	Verify(context.Context, string, string) error
}
type SiteVerifier struct {
	Secret string
	Client *http.Client
}
type DevelopmentVerifier struct{}

func (DevelopmentVerifier) Verify(_ context.Context, token, _ string) error {
	if token != "rfs-development-turnstile" {
		return errors.New("development challenge failed")
	}
	return nil
}

func (v SiteVerifier) Verify(ctx context.Context, token, ip string) error {
	if token == "" {
		return errors.New("missing turnstile token")
	}
	form := url.Values{"secret": {v.Secret}, "response": {token}, "remoteip": {ip}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://challenges.cloudflare.com/turnstile/v0/siteverify", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := v.Client.Do(req)
	if err != nil {
		return fmt.Errorf("turnstile unavailable: %w", err)
	}
	defer res.Body.Close()
	var body struct {
		Success bool `json:"success"`
	}
	if err := decodeJSON(res.Body, &body); err != nil || !body.Success {
		return errors.New("turnstile verification failed")
	}
	return nil
}

type Service struct {
	DB                       *sql.DB
	SessionPepper, OTPPepper string
	Mailer                   Mailer
	Turnstile                Turnstile
	Now                      func() time.Time
	SecureCookies            bool
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func NormalizeEmail(value string) (string, error) {
	value = strings.ToLower(norm.NFKC.String(strings.TrimSpace(value)))
	if len(value) > 254 || !strings.Contains(value, "@") {
		return "", errors.New("invalid email")
	}
	return value, nil
}
func Digest(secret, value string) []byte {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(value))
	return h.Sum(nil)
}
func GenerateToken(kind string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	secret := base64.RawURLEncoding.EncodeToString(raw)
	if kind == "reader" {
		return "hub_" + secret + ".reader.v1", nil
	}
	return "hub_" + secret + ".v1", nil
}
func ParseToken(token string) (string, error) {
	if strings.HasPrefix(token, "hub_") && strings.HasSuffix(token, ".reader.v1") && len(strings.TrimSuffix(strings.TrimPrefix(token, "hub_"), ".reader.v1")) == 43 {
		return "reader", nil
	}
	if strings.HasPrefix(token, "hub_") && strings.HasSuffix(token, ".v1") && len(strings.TrimSuffix(strings.TrimPrefix(token, "hub_"), ".v1")) == 43 {
		return "staff", nil
	}
	return "", ErrUnauthorized
}
func generateCode() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	n := (uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])) % 1000000
	return fmt.Sprintf("%06d", n), nil
}

func (s *Service) RequestOTP(ctx context.Context, kind, email, ip, turnstile string) error {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return nil
	}
	if kind == "reader" {
		if s.Turnstile == nil || s.Turnstile.Verify(ctx, turnstile, ip) != nil {
			return errors.New("verification failed")
		}
		if err = s.applyReaderLimits(ctx, normalized, ip); err != nil {
			return err
		}
	} else {
		var n int
		if err = s.DB.QueryRowContext(ctx, "SELECT count(*) FROM identities i JOIN staff_profiles s ON s.identity_id=i.id WHERE i.email=? AND s.status='active'", normalized).Scan(&n); err != nil || n == 0 {
			return nil
		}
	}
	now := s.now()
	var last string
	_ = s.DB.QueryRowContext(ctx, "SELECT created_at FROM otp_challenges WHERE identity_kind=? AND email=? ORDER BY created_at DESC LIMIT 1", kind, normalized).Scan(&last)
	if last != "" {
		if t, e := time.Parse(time.RFC3339Nano, last); e == nil && now.Sub(t) < time.Minute {
			return errors.New("wait before requesting another code")
		}
	}
	code, err := generateCode()
	if err != nil {
		return err
	}
	challengeID := uuid.NewString()
	_, err = s.DB.ExecContext(ctx, "INSERT INTO otp_challenges(id,identity_kind,email,code_digest,expires_at,created_at) VALUES(?,?,?,?,?,?)", challengeID, kind, normalized, Digest(s.OTPPepper, code), now.Add(10*time.Minute).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	if err = s.Mailer.SendOTP(ctx, normalized, code); err != nil {
		_, _ = s.DB.ExecContext(ctx, "DELETE FROM otp_challenges WHERE id=?", challengeID)
		return fmt.Errorf("%w: %v", ErrDelivery, err)
	}
	return nil
}
func (s *Service) applyReaderLimits(ctx context.Context, email, ip string) error {
	now := s.now()
	checks := []struct {
		key    string
		window time.Duration
		limit  int
	}{{"email-hour:" + email, time.Hour, 3}, {"email-day:" + email, 24 * time.Hour, 8}, {"ip-hour:" + ip, time.Hour, 30}, {"ip-day:" + ip, 24 * time.Hour, 100}}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range checks {
		start := now.Truncate(c.window).Format(time.RFC3339Nano)
		var count int
		_ = tx.QueryRowContext(ctx, "SELECT count FROM rate_limit_buckets WHERE bucket_key=? AND window_start=?", c.key, start).Scan(&count)
		if count >= c.limit {
			return errors.New("too many requests")
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO rate_limit_buckets(bucket_key,window_start,count) VALUES(?,?,1) ON CONFLICT(bucket_key,window_start) DO UPDATE SET count=count+1", c.key, start); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Service) VerifyOTP(ctx context.Context, kind, email, code, userAgent, ip string) (token, csrf string, actor Actor, err error) {
	normalized, e := NormalizeEmail(email)
	if e != nil {
		return "", "", actor, ErrUnauthorized
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return "", "", actor, e
	}
	defer tx.Rollback()
	var id, digest, expires string
	var attempts int
	e = tx.QueryRowContext(ctx, "SELECT id,hex(code_digest),attempts,expires_at FROM otp_challenges WHERE identity_kind=? AND email=? AND consumed_at IS NULL ORDER BY created_at DESC LIMIT 1", kind, normalized).Scan(&id, &digest, &attempts, &expires)
	if e != nil {
		return "", "", actor, ErrUnauthorized
	}
	now := s.now()
	expiry, _ := time.Parse(time.RFC3339Nano, expires)
	if attempts >= 5 || !now.Before(expiry) {
		return "", "", actor, ErrUnauthorized
	}
	if !hmac.Equal([]byte(strings.ToUpper(digest)), []byte(strings.ToUpper(fmt.Sprintf("%x", Digest(s.OTPPepper, code))))) {
		tx.ExecContext(ctx, "UPDATE otp_challenges SET attempts=attempts+1 WHERE id=?", id)
		tx.Commit()
		return "", "", actor, ErrUnauthorized
	}
	var identityID string
	if kind == "staff" {
		e = tx.QueryRowContext(ctx, "SELECT i.id FROM identities i JOIN staff_profiles s ON s.identity_id=i.id WHERE i.email=? AND s.status='active'", normalized).Scan(&identityID)
		if e != nil {
			return "", "", actor, ErrUnauthorized
		}
	} else {
		e = tx.QueryRowContext(ctx, "SELECT id FROM identities WHERE email=?", normalized).Scan(&identityID)
		if errors.Is(e, sql.ErrNoRows) {
			identityID = uuid.NewString()
			stamp := now.Format(time.RFC3339Nano)
			_, e = tx.ExecContext(ctx, "INSERT INTO identities(id,email,created_at,updated_at)VALUES(?,?,?,?)", identityID, normalized, stamp, stamp)
		}
		if e != nil {
			return "", "", actor, e
		}
		_, _ = tx.ExecContext(ctx, "UPDATE reader_profiles SET status='active' WHERE identity_id=? AND status='suspended' AND NOT EXISTS(SELECT 1 FROM account_suspensions WHERE identity_id=? AND (ends_at IS NULL OR ends_at>?))", identityID, identityID, now.Format(time.RFC3339Nano))
		var profileStatus string
		e = tx.QueryRowContext(ctx, "SELECT status FROM reader_profiles WHERE identity_id=?", identityID).Scan(&profileStatus)
		if e == nil && profileStatus != "active" {
			return "", "", actor, ErrUnauthorized
		}
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return "", "", actor, e
		}
	}
	token, e = GenerateToken(kind)
	if e != nil {
		return "", "", actor, e
	}
	csrfSecret, e := GenerateToken("staff")
	if e != nil {
		return "", "", actor, e
	}
	csrf = strings.TrimSuffix(strings.TrimPrefix(csrfSecret, "hub_"), ".v1")
	duration := 3 * 24 * time.Hour
	if kind == "reader" {
		duration = 30 * 24 * time.Hour
	}
	sessionID := uuid.NewString()
	_, e = tx.ExecContext(ctx, "INSERT INTO sessions(id,identity_id,kind,token_digest,csrf_digest,user_agent,ip_address,created_at,last_activity_at,expires_at) VALUES(?,?,?,?,?,?,?,?,?,?)", sessionID, identityID, kind, Digest(s.SessionPepper, token), Digest(s.SessionPepper, csrf), userAgent, ip, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), now.Add(duration).Format(time.RFC3339Nano))
	if e != nil {
		return "", "", actor, e
	}
	tx.ExecContext(ctx, "UPDATE otp_challenges SET consumed_at=? WHERE id=?", now.Format(time.RFC3339Nano), id)
	if e = tx.Commit(); e != nil {
		return "", "", actor, e
	}
	actor = Actor{IdentityID: identityID, Kind: kind, CSRF: csrf}
	return
}
func (s *Service) Authenticate(ctx context.Context, token string) (Actor, error) {
	kind, err := ParseToken(token)
	if err != nil {
		return Actor{}, err
	}
	var a Actor
	var digest []byte
	var last string
	query := "SELECT se.identity_id,se.kind,COALESCE(sp.role,''),COALESCE(sp.publish_mode,''),COALESCE(sp.status,rp.status,''),se.csrf_digest,se.last_activity_at FROM sessions se LEFT JOIN staff_profiles sp ON sp.identity_id=se.identity_id LEFT JOIN reader_profiles rp ON rp.identity_id=se.identity_id WHERE se.token_digest=? AND se.kind=? AND se.revoked_at IS NULL AND se.expires_at>?"
	err = s.DB.QueryRowContext(ctx, query, Digest(s.SessionPepper, token), kind, s.now().Format(time.RFC3339Nano)).Scan(&a.IdentityID, &a.Kind, &a.Role, &a.PublishMode, &a.Status, &digest, &last)
	if err != nil {
		return Actor{}, ErrUnauthorized
	}
	a.CSRF = base64.RawURLEncoding.EncodeToString(digest)
	if t, e := time.Parse(time.RFC3339Nano, last); e == nil && s.now().Sub(t) > 5*time.Minute {
		s.DB.ExecContext(ctx, "UPDATE sessions SET last_activity_at=? WHERE token_digest=?", s.now().Format(time.RFC3339Nano), Digest(s.SessionPepper, token))
	}
	return a, nil
}
func (s *Service) ValidateCSRF(actor Actor, header string) bool {
	return hmac.Equal(Digest(s.SessionPepper, header), mustDecode(actor.CSRF))
}
func mustDecode(v string) []byte { b, _ := base64.RawURLEncoding.DecodeString(v); return b }
func SetSessionCookie(w http.ResponseWriter, kind, token, csrf string, secure bool) {
	name := StaffCookie
	age := 3 * 24 * 3600
	if kind == "reader" {
		name = ReaderCookie
		age = 30 * 24 * 3600
	}
	csrfName := StaffCSRFCookie
	if kind == "reader" {
		csrfName = ReaderCSRFCookie
	}
	if !secure {
		if kind == "reader" {
			name = "rfs_reader_session"
		} else {
			name = "rfs_staff_session"
		}
		csrfName = "rfs_staff_csrf"
		if kind == "reader" {
			csrfName = "rfs_reader_csrf"
		}
	}
	http.SetCookie(w, &http.Cookie{Name: name, Value: token, Path: "/", MaxAge: age, Secure: secure, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: csrfName, Value: csrf, Path: "/", MaxAge: age, Secure: secure, HttpOnly: false, SameSite: http.SameSiteLaxMode})
}
func ClientIP(r *http.Request) string {
	for _, name := range []string{"CF-Connecting-IP", "X-Forwarded-For"} {
		if v := r.Header.Get(name); v != "" {
			return strings.TrimSpace(strings.Split(v, ",")[0])
		}
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}
func ValidUsername(v string) bool {
	if len(v) < 3 || len(v) > 24 {
		return false
	}
	for _, r := range v {
		if !(r >= 'a' && r <= 'z' || unicode.IsDigit(r) || r == '_') {
			return false
		}
	}
	reserved := map[string]bool{"admin": true, "administrator": true, "rwanda": true, "rfs": true, "support": true, "deleted": true}
	return !reserved[v]
}
