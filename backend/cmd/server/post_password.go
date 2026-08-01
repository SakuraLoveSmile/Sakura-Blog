package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

const postPWCookieTTL = 7 * 24 * time.Hour

func postPWCookieName(slug string) string {
	sum := sha256.Sum256([]byte("post_pw|" + slug))
	return "ppw_" + hex.EncodeToString(sum[:8])
}

func signPostPWCookie(slug, password string, expiry int64) string {
	mac := hmac.New(sha256.New, []byte(password+"|"+slug))
	mac.Write([]byte(strconv.FormatInt(expiry, 10)))
	return strconv.FormatInt(expiry, 10) + "." + hex.EncodeToString(mac.Sum(nil))
}

func validPostPWCookie(value, slug, password string) bool {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return false
	}
	expiry, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return false
	}
	expect := signPostPWCookie(slug, password, expiry)
	return subtle.ConstantTimeCompare([]byte(value), []byte(expect)) == 1
}
