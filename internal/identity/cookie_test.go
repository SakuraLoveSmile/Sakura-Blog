package identity

import (
	"testing"
	"time"
)

func TestCookieAudienceAndExpiry(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	cookie := SignCookie(AdminAudience, now.Add(time.Hour).Unix(), 42, "secret")
	if got, _, ok := VerifyCookie(cookie, AdminAudience, "secret", now); !ok || got != 42 {
		t.Fatalf("VerifyCookie() = (%d, %v), want (42, true)", got, ok)
	}
	if _, _, ok := VerifyCookie(cookie, ReaderAudience, "secret", now); ok {
		t.Fatal("admin cookie must not verify for reader audience")
	}
	if _, _, ok := VerifyCookie(cookie, AdminAudience, "wrong", now); ok {
		t.Fatal("cookie must reject a different signing secret")
	}
	if _, _, ok := VerifyCookie(cookie, AdminAudience, "secret", now.Add(2*time.Hour)); ok {
		t.Fatal("expired cookie must be rejected")
	}
}
