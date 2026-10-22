package identity

import (
	"strings"
	"testing"
	"time"
)

func TestValidateTOTPRFC6238AndWindow(t *testing.T) {
	for _, vector := range []struct {
		at   int64
		code string
	}{
		{59, "287082"}, {1111111109, "081804"}, {1111111111, "050471"},
		{1234567890, "005924"}, {2000000000, "279037"},
	} {
		if !validateTOTPAt(publicTestTOTPSecret, vector.code, time.Unix(vector.at, 0)) {
			t.Fatalf("RFC6238 time=%d rejected", vector.at)
		}
	}
	now := time.Unix(1234567890, 0)
	for offset := int64(-2); offset <= 2; offset++ {
		code := publicTestTOTPCode(now.Unix()/30 + offset)
		want := offset >= -1 && offset <= 1
		if got := validateTOTPAt(publicTestTOTPSecret, code, now); got != want {
			t.Fatalf("window offset=%d got=%v want=%v", offset, got, want)
		}
	}
	if !validateTOTPAt(strings.ToLower(publicTestTOTPSecret), "005924", now) {
		t.Fatal("existing lowercase secret compatibility lost")
	}
}

func TestValidateTOTPMalformedInputsFailClosed(t *testing.T) {
	now := time.Unix(1234567890, 0)
	for _, secret := range []string{"", "!", "A", "89INVALID", publicTestTOTPSecret + "=", publicTestTOTPSecret + "\n", " " + publicTestTOTPSecret} {
		if validateTOTPAt(secret, "005924", now) {
			t.Fatalf("malformed secret accepted: %q", secret)
		}
	}
	for _, code := range []string{"", "00592", "0059240", "00592x", " 005924", "005924\n", "００５９２４", "000000"} {
		if validateTOTPAt(publicTestTOTPSecret, code, now) {
			t.Fatalf("invalid code accepted: %q", code)
		}
	}
}
