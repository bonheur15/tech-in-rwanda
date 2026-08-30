package servertime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rwandafreespace.com/blog/backend/internal/platform/requestmeta"
)

func TestHandlerReturnsKigaliTimeAndRequestMetadata(t *testing.T) {
	fixed := time.Date(2026, time.August, 30, 20, 15, 23, 47_000_000, time.UTC)
	handler := NewHandler(func() time.Time { return fixed })
	request := httptest.NewRequest(http.MethodGet, Route, nil)
	request = request.WithContext(requestmeta.WithID(request.Context(), "request-test"))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}

	var response Response
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.ISO != "2026-08-30T22:15:23.047+02:00" {
		t.Errorf("ISO = %q", response.Data.ISO)
	}
	if response.Data.Display != "Sunday, 30 August 2026 at 22:15:23 CAT" {
		t.Errorf("display = %q", response.Data.Display)
	}
	if response.Data.TimeZone != TimeZone {
		t.Errorf("time zone = %q", response.Data.TimeZone)
	}
	if response.Meta.RequestID != "request-test" {
		t.Errorf("request ID = %q", response.Meta.RequestID)
	}
}

func TestHandlerRejectsUnsupportedMethods(t *testing.T) {
	handler := NewHandler(time.Now)
	request := httptest.NewRequest(http.MethodPost, Route, nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
	if got := recorder.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("Allow = %q, want GET", got)
	}
}
