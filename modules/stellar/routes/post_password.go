package routes

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"time"
)

const postPWCookieTTL = 7 * 24 * time.Hour

func postPWCookieName(slug string) string {
	sum := sha256.Sum256([]byte("post_pw|" + slug))
	return "ppw_" + hex.EncodeToString(sum[:8])
}

// postPWServerSecret 每次进程启动随机生成（安全审计 2026-08-16）：
// 此前 cookie 的 HMAC 密钥 = 口令本身（password+"|"+slug），拿到一枚有效 cookie
// （浏览器导出/截图/共享设备）即可离线字典爆破文章口令；现密钥 =
// serverSecret + sha256(password|slug)，离线爆破必须先攻破 256 位随机密钥。
// 副作用：进程重启后已解锁 cookie 失效，读者需重新输入口令（低频可接受）。
var postPWServerSecret = func() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("post password server secret: " + err.Error())
	}
	return b
}()

// postPWKey 派生 HMAC 密钥：serverSecret 拼接口令哈希（不直接用明文口令，
// 口令长度不影响密钥强度）。
func postPWKey(slug, password string) []byte {
	h := sha256.Sum256([]byte(password + "|" + slug))
	key := make([]byte, 0, len(postPWServerSecret)+sha256.Size)
	key = append(key, postPWServerSecret...)
	key = append(key, h[:]...)
	return key
}

func signPostPWCookie(slug, password string, expiry int64) string {
	mac := hmac.New(sha256.New, postPWKey(slug, password))
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

// --- unlock 限流（安全审计 2026-08-16）---
// POST /post/:slug/unlock 此前无任何限流，可在线无限爆破口令
// （ConstantTimeCompare 只保证时序安全，不抗频率攻击）。与 API 登录限流同模式：
// IP+slug 维度 5 次失败/5 分钟，触发后封锁 5 分钟。

const (
	postPWRateWindow = 5 * time.Minute
	postPWMaxFails   = 5
	postPWBlockTime  = 5 * time.Minute
)

type postPWFail struct {
	n int
	t time.Time
}

var postPWFails = struct {
	sync.Mutex
	m map[string]postPWFail
}{m: make(map[string]postPWFail)}

// postPWBlocked 检查 IP+slug 是否被限流，顺带惰性清理过期条目。
func postPWBlocked(key string) bool {
	postPWFails.Lock()
	defer postPWFails.Unlock()
	now := time.Now()
	for k, v := range postPWFails.m {
		if now.Sub(v.t) > postPWBlockTime {
			delete(postPWFails.m, k)
		}
	}
	v, ok := postPWFails.m[key]
	return ok && v.n >= postPWMaxFails && now.Sub(v.t) < postPWBlockTime
}

// postPWRecordFail 记录一次解锁失败。
func postPWRecordFail(key string) {
	postPWFails.Lock()
	defer postPWFails.Unlock()
	v := postPWFails.m[key]
	if time.Since(v.t) > postPWRateWindow {
		v.n = 0
	}
	v.n++
	v.t = time.Now()
	postPWFails.m[key] = v
}

// postPWClear 解锁成功后清零计数。
func postPWClear(key string) {
	postPWFails.Lock()
	defer postPWFails.Unlock()
	delete(postPWFails.m, key)
}
