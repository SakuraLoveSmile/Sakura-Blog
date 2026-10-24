package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"net/url"

	"blog/internal/identity"
)

const (
	totpPeriod = 30
	totpDigits = 6
)

// GenerateSecret 生成 TOTP secret（随机 20 字节 → base32 编码，无 padding）。
func GenerateSecret() string {
	b := make([]byte, 20)
	rand.Read(b)
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
}

// GenerateRecoveryCodes 生成 n 个 8 字符随机恢复码。
func GenerateRecoveryCodes(n int) []string {
	codes := make([]string, n)
	for i := range codes {
		b := make([]byte, 4)
		rand.Read(b)
		codes[i] = fmt.Sprintf("%08x", b)
	}
	return codes
}

// ValidateTOTP 验证 TOTP code（HMAC-SHA1、30s 窗口 ±1）。
func ValidateTOTP(secret, code string) bool {
	return identity.ValidateTOTP(secret, code)
}

// generateTOTP 对指定时间计数器生成 6 位 TOTP code。
func generateTOTP(key []byte, counter int64) string {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(counter))

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	hash := mac.Sum(nil)

	// Dynamic truncation
	offset := hash[len(hash)-1] & 0x0F
	code := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7FFFFFFF
	code = code % uint32(math.Pow10(totpDigits))

	return fmt.Sprintf("%06d", code)
}

// TOTPAuthURL 生成 otpauth://totp/... URL（用于二维码）。
func TOTPAuthURL(secret, account, issuer string) string {
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&digits=%d&period=%d",
		url.PathEscape(issuer), url.PathEscape(account),
		secret, url.QueryEscape(issuer), totpDigits, totpPeriod)
}
