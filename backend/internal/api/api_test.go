package api

import (
	"database/sql"
	"errors"
	"net/http/httptest"
	"testing"

	"rwandafreespace.com/blog/backend/internal/auth"
)

func TestActorKindForRequest(t *testing.T) {
	tests := map[string]string{
		"/api/v1/admin/posts":              "staff",
		"/api/v1/posts/one/draft":          "staff",
		"/api/v1/articles/one/comments":    "reader",
		"/api/v1/comments/one/reports":     "reader",
		"/api/v1/bookmarks":                "reader",
		"/api/v1/auth/me?kind=reader":      "reader",
		"/api/v1/sessions/one?kind=reader": "reader",
	}
	for path, want := range tests {
		r := httptest.NewRequest("GET", path, nil)
		if got := actorKindForRequest(r); got != want {
			t.Errorf("actorKindForRequest(%q)=%q want %q", path, got, want)
		}
	}
}

func TestPublicErrorDoesNotExposeInternals(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{"missing", sql.ErrNoRows, 404, "not_found", "The requested resource was not found"},
		{"forbidden", auth.ErrUnauthorized, 403, "forbidden", "You do not have permission to perform this action"},
		{"validation", errors.New("invalid cursor"), 400, "invalid_request", "invalid cursor"},
		{"delivery", auth.ErrDelivery, 503, "delivery_unavailable", "The verification code could not be delivered. Please try again"},
		{"internal", errors.New("SQL secret table detail"), 500, "internal_error", "The request could not be completed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, code, message := publicError(test.err)
			if status != test.status || code != test.code || message != test.message {
				t.Fatalf("got (%d, %q, %q), want (%d, %q, %q)", status, code, message, test.status, test.code, test.message)
			}
		})
	}
}

func TestMutationOriginAllowsConfiguredDevelopmentAliases(t *testing.T) {
	a := &API{Origin: "http://127.0.0.1:4321", AllowedOrigins: map[string]struct{}{"http://localhost:4321": {}}}
	if !a.originAllowed("http://127.0.0.1:4321") || a.originAllowed("http://localhost:4321") || a.originAllowed("https://attacker.example") || a.originAllowed("") {
		t.Fatal("production origin policy is not same-origin")
	}
	a.Development = true
	if !a.originAllowed("") || !a.originAllowed("http://localhost:4321") {
		t.Fatal("development origin aliases were rejected")
	}
}
