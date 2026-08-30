package auth

import (
	"strings"
	"testing"
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
func TestRejectMalformedToken(t *testing.T) {
	for _, v := range []string{"", "hub_short.v1", "hub_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.reader.v2", "ey.jwt.token"} {
		if _, err := ParseToken(v); err == nil {
			t.Fatalf("accepted %q", v)
		}
	}
}
func TestUsername(t *testing.T) {
	if !ValidUsername("umucyo_24") || ValidUsername("Admin") || ValidUsername("ab") || ValidUsername("rwanda") {
		t.Fatal("username rules failed")
	}
}
