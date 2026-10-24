package identity

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// ValidateTOTP 校验六位 HMAC-SHA1 TOTP，允许相邻两个 30 秒时间窗口。
// Console 和公开读者登录使用同一校验，畸形密钥或验证码一律拒绝。
func ValidateTOTP(secret, code string) bool {
	return validateTOTPAt(secret, code, time.Now())
}

func validateTOTPAt(secret, code string, now time.Time) bool {
	if len(code) != 6 {
		return false
	}
	for _, digit := range code {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	secret = strings.ToUpper(secret)
	for _, char := range secret {
		if !(char >= 'A' && char <= 'Z' || char >= '2' && char <= '7') {
			return false
		}
	}
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil || len(key) == 0 {
		return false
	}
	counter := now.Unix() / 30
	matched := 0
	for offset := int64(-1); offset <= 1; offset++ {
		if counter+offset < 0 {
			continue
		}
		var input [8]byte
		binary.BigEndian.PutUint64(input[:], uint64(counter+offset))
		mac := hmac.New(sha1.New, key)
		_, _ = mac.Write(input[:])
		digest := mac.Sum(nil)
		position := digest[len(digest)-1] & 15
		value := (binary.BigEndian.Uint32(digest[position:position+4]) & 0x7fffffff) % 1000000
		expected := fmt.Sprintf("%06d", value)
		matched |= subtle.ConstantTimeCompare([]byte(expected), []byte(code))
	}
	return matched == 1
}
