package api

import (
	"net/http/httptest"
	"testing"
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

func TestMutationOriginAllowsConfiguredDevelopmentAliases(t *testing.T) {
	a := &API{Origin: "http://127.0.0.1:4321", AllowedOrigins: map[string]struct{}{"http://localhost:4321": {}}}
	if !a.originAllowed("http://127.0.0.1:4321") || !a.originAllowed("http://localhost:4321") || a.originAllowed("https://attacker.example") {
		t.Fatal("origin allowlist is inconsistent")
	}
}
