// Package identity owns shared authentication and reader identity primitives.
package identity

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Audience prevents a reader cookie from being replayed as an admin cookie.
type Audience string

const (
	AdminAudience  Audience = "admin"
	ReaderAudience Audience = "user"
)

// SignCookie creates the stable four-part session cookie payload.
func SignCookie(audience Audience, expiry, userID int64, secret string) string {
	message := fmt.Sprintf("%s.%d.%d", audience, expiry, userID)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(message))
	return message + "." + hex.EncodeToString(mac.Sum(nil))
}

// VerifyCookie validates audience, expiry and HMAC, returning the user ID and
// expiry when valid. Callers still own the user lookup and role policy.
func VerifyCookie(value string, audience Audience, secret string, now time.Time) (userID, expiry int64, ok bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 4 || parts[0] != string(audience) {
		return 0, 0, false
	}
	expiry, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || now.Unix() > expiry {
		return 0, 0, false
	}
	userID, err = strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	expected := SignCookie(audience, expiry, userID, secret)
	partsExpected := strings.Split(expected, ".")
	if subtle.ConstantTimeCompare([]byte(parts[3]), []byte(partsExpected[3])) != 1 {
		return 0, 0, false
	}
	return userID, expiry, true
}
