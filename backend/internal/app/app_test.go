package app

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"rwandafreespace.com/blog/backend/internal/auth"
	"rwandafreespace.com/blog/backend/internal/platform/config"
	"rwandafreespace.com/blog/backend/internal/platform/database"
	"testing"
	"time"
)

func TestAPIHealthSecurityAndCORS(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "test.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := config.Config{Address: ":0", DatabasePath: "unused", SessionPepper: "test-session-pepper", OTPPepper: "test-otp-pepper", PublicOrigin: "http://localhost:4321", AllowedOrigins: map[string]struct{}{"http://localhost:4321": {}}, ShutdownTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second, IdleTimeout: time.Second, ReadHeaderTimeout: time.Second}
	handler := New(cfg, db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	request.Header.Set("Origin", "http://localhost:4321")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Access-Control-Allow-Origin") != "http://localhost:4321" {
		t.Fatal("missing CORS")
	}
	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing security header")
	}
}

func TestLastSuperadminCannotBeRemoved(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "admin.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	stamp := now.Format(time.RFC3339Nano)
	_, err = db.Exec("INSERT INTO identities(id,email,created_at,updated_at)VALUES('admin','admin@example.com',?,?); INSERT INTO staff_profiles(identity_id,handle,display_name,role,publish_mode,status,created_at,updated_at)VALUES('admin','admin','Admin','superadmin','direct_publish','active',?,?)", stamp, stamp, stamp, stamp)
	if err != nil {
		t.Fatal(err)
	}
	token, _ := auth.GenerateToken("staff")
	csrf := "test-csrf"
	pepper := "test-session-pepper"
	_, err = db.Exec("INSERT INTO sessions(id,identity_id,kind,token_digest,csrf_digest,user_agent,ip_address,created_at,last_activity_at,expires_at)VALUES('session','admin','staff',?,?, 'test','127.0.0.1',?,?,?)", auth.Digest(pepper, token), auth.Digest(pepper, csrf), stamp, stamp, now.Add(time.Hour).Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{SessionPepper: pepper, OTPPepper: "test-otp", PublicOrigin: "http://localhost:4321", AllowedOrigins: map[string]struct{}{"http://localhost:4321": {}}, Environment: "development"}
	handler := New(cfg, db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	body := bytes.NewBufferString(`{"role":"author","publishMode":"direct_publish","status":"active","reassignTo":""}`)
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/staff/admin", body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://localhost:4321")
	request.Header.Set("X-CSRF-Token", csrf)
	request.AddCookie(&http.Cookie{Name: auth.StaffCookie, Value: token})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
