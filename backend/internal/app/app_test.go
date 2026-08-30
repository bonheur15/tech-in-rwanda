package app

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rwandafreespace.com/blog/backend/internal/platform/config"
)

func TestAPIHealthSecurityAndCORS(t *testing.T) {
	handler := New(testConfig(""), slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	request.Header.Set("Origin", "http://localhost:4321")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:4321" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
	if got := recorder.Header().Get("X-Request-ID"); len(got) != 24 {
		t.Errorf("X-Request-ID = %q, want 24 characters", got)
	}
}

func TestRejectsUnknownCrossOriginRequests(t *testing.T) {
	handler := New(testConfig(""), slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/server-time", nil)
	request.Header.Set("Origin", "https://example.invalid")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestServesBuiltAstroFiles(t *testing.T) {
	directory := t.TempDir()
	pageDirectory := filepath.Join(directory, "server-time")
	if err := os.MkdirAll(pageDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pageDirectory, "index.html"), []byte("<h1>API test</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := New(testConfig(directory), slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodGet, "/server-time/", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "API test") {
		t.Fatalf("body = %q", recorder.Body.String())
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q", got)
	}
}

func TestRedirectsStaticPageDirectoryToTrailingSlash(t *testing.T) {
	directory := t.TempDir()
	pageDirectory := filepath.Join(directory, "server-time")
	if err := os.MkdirAll(pageDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pageDirectory, "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := New(testConfig(directory), slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodGet, "/server-time", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMovedPermanently)
	}
	if got := recorder.Header().Get("Location"); got != "server-time/" {
		t.Fatalf("Location = %q, want server-time/", got)
	}
}

func testConfig(staticDirectory string) config.Config {
	return config.Config{
		Address:           ":0",
		StaticDir:         staticDirectory,
		AllowedOrigins:    map[string]struct{}{"http://localhost:4321": {}},
		ShutdownTimeout:   time.Second,
		ReadTimeout:       time.Second,
		WriteTimeout:      time.Second,
		IdleTimeout:       time.Second,
		ReadHeaderTimeout: time.Second,
	}
}
