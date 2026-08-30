package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
