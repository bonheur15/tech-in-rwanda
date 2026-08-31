package auth

import (
	"context"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rwandafreespace.com/blog/backend/internal/platform/database"
)

func TestTokenFormat(t *testing.T) {
	for _, kind := range []string{"staff", "reader"} {
		token, err := GenerateToken(kind)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ParseToken(token)
		if err != nil || got != kind {
			t.Fatalf("%s: %s %v", token, got, err)
		}
		if strings.Contains(token, "=") {
			t.Fatal("token must be raw base64url")
		}
	}
}

type captureMailer struct{ code string }

func (m *captureMailer) SendOTP(_ context.Context, _ string, code string) error {
	m.code = code
	return nil
}

type failingMailer struct{}

func (failingMailer) SendOTP(context.Context, string, string) error {
	return errors.New("SMTP offline")
}

func TestStaffOTPCreatesUsableOpaqueSession(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "auth.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 8, 31, 1, 0, 0, 0, time.UTC)
	stamp := now.Format(time.RFC3339Nano)
	_, err = db.Exec("INSERT INTO identities(id,email,created_at,updated_at)VALUES('staff-1','editor@example.com',?,?); INSERT INTO staff_profiles(identity_id,handle,display_name,role,publish_mode,status,created_at,updated_at)VALUES('staff-1','editor','Editor','superadmin','direct_publish','active',?,?)", stamp, stamp, stamp, stamp)
	if err != nil {
		t.Fatal(err)
	}
	mailer := &captureMailer{}
	service := &Service{DB: db, SessionPepper: "session-test-pepper", OTPPepper: "otp-test-pepper", Mailer: mailer, Now: func() time.Time { return now }}
	if err = service.RequestOTP(ctx, "staff", " Editor@Example.com ", "127.0.0.1", ""); err != nil {
		t.Fatal(err)
	}
	if len(mailer.code) != 6 {
		t.Fatalf("code=%q", mailer.code)
	}
	token, csrf, actor, err := service.VerifyOTP(ctx, "staff", "editor@example.com", mailer.code, "test browser", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if actor.IdentityID != "staff-1" || csrf == "" {
		t.Fatalf("actor=%+v csrf=%q", actor, csrf)
	}
	if kind, err := ParseToken(token); err != nil || kind != "staff" {
		t.Fatalf("token=%q kind=%q err=%v", token, kind, err)
	}
	authenticated, err := service.Authenticate(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if authenticated.Role != "superadmin" || !service.ValidateCSRF(authenticated, csrf) {
		t.Fatalf("authenticated=%+v", authenticated)
	}
}
func TestRejectMalformedToken(t *testing.T) {
	for _, v := range []string{"", "hub_short.v1", "hub_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.reader.v2", "ey.jwt.token"} {
		if _, err := ParseToken(v); err == nil {
			t.Fatalf("accepted %q", v)
		}
	}
}

func TestSessionCookiesAreScopedByCapability(t *testing.T) {
	staff := httptest.NewRecorder()
	SetSessionCookie(staff, "staff", "hub_staff.v1", "staff-csrf", true)
	reader := httptest.NewRecorder()
	SetSessionCookie(reader, "reader", "hub_reader.reader.v1", "reader-csrf", true)
	staffCookies := strings.Join(staff.Header().Values("Set-Cookie"), "\n")
	readerCookies := strings.Join(reader.Header().Values("Set-Cookie"), "\n")
	if !strings.Contains(staffCookies, StaffCSRFCookie+"=") || strings.Contains(staffCookies, ReaderCSRFCookie+"=") {
		t.Fatalf("staff cookie scope is wrong: %s", staffCookies)
	}
	if !strings.Contains(readerCookies, ReaderCSRFCookie+"=") {
		t.Fatalf("reader CSRF cookie missing: %s", readerCookies)
	}
}

func TestDevelopmentTurnstileIsExplicit(t *testing.T) {
	verifier := DevelopmentVerifier{}
	if err := verifier.Verify(context.Background(), "rfs-development-turnstile", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if err := verifier.Verify(context.Background(), "anything-else", "127.0.0.1"); err == nil {
		t.Fatal("unexpected development token accepted")
	}
}

func TestDeliveryFailureRemovesUnusableChallenge(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "delivery.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 8, 31, 2, 0, 0, 0, time.UTC)
	service := &Service{DB: db, OTPPepper: "otp", SessionPepper: "session", Mailer: failingMailer{}, Turnstile: DevelopmentVerifier{}, Now: func() time.Time { return now }}
	if err = service.RequestOTP(ctx, "reader", "reader@example.com", "127.0.0.1", "rfs-development-turnstile"); !errors.Is(err, ErrDelivery) {
		t.Fatalf("err=%v", err)
	}
	var count int
	if err = db.QueryRow("SELECT count(*) FROM otp_challenges").Scan(&count); err != nil || count != 0 {
		t.Fatalf("challenges=%d err=%v", count, err)
	}
}

func TestOTPExpiryAttemptsAndPersistentRateLimits(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "limits.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 8, 31, 3, 0, 0, 0, time.UTC)
	mailer := &captureMailer{}
	service := &Service{DB: db, OTPPepper: "otp", SessionPepper: "session", Mailer: mailer, Turnstile: DevelopmentVerifier{}, Now: func() time.Time { return now }}
	if err = service.RequestOTP(ctx, "reader", "limits@example.com", "127.0.0.1", "rfs-development-turnstile"); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		_, _, _, _ = service.VerifyOTP(ctx, "reader", "limits@example.com", "000000", "browser", "127.0.0.1")
	}
	if _, _, _, err = service.VerifyOTP(ctx, "reader", "limits@example.com", mailer.code, "browser", "127.0.0.1"); err == nil {
		t.Fatal("accepted code after five failed attempts")
	}
	now = now.Add(time.Hour)
	for i := 0; i < 3; i++ {
		if err = service.RequestOTP(ctx, "reader", "rate@example.com", "127.0.0.2", "rfs-development-turnstile"); err != nil {
			t.Fatal(err)
		}
		now = now.Add(time.Minute)
	}
	if err = service.RequestOTP(ctx, "reader", "rate@example.com", "127.0.0.2", "rfs-development-turnstile"); err == nil {
		t.Fatal("email hourly limit did not persist")
	}
}
func TestUsername(t *testing.T) {
	if !ValidUsername("umucyo_24") || ValidUsername("Admin") || ValidUsername("ab") || ValidUsername("rwanda") {
		t.Fatal("username rules failed")
	}
}
