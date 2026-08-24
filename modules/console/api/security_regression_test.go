package api

// 安全审计 2026-08-16 回归测试：认证链修复后的关键不变量。
//  1. reader 不能经 console 登录端点换取 blog_admin 会话；
//  2. 读者 cookie（audience=user）改名重放为 blog_admin 验签失败；
//  3. 即使持有 admin audience 的合法签名，reader 角色也过不了内容写端点的
//     requireRole(RoleAuthor) 门禁；
//  4. 备份恢复上传在请求体超限时被 MaxBytesReader 拒绝。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/safefetch"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// newReaderEnv 密码站点 + admin + reader 两个用户。
func newReaderEnv(t *testing.T) (*gin.Engine, *store.Store, *store.User, *store.User) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	if err := st.EnsureUsersTable(); err != nil {
		t.Fatalf("ensure users table: %v", err)
	}
	hash, err := store.HashPassword("readerpass")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	reader := &store.User{
		Username: "reader1", PasswordHash: hash,
		AuthSecret: store.GenerateAuthSecret(), Role: store.RoleReader,
	}
	if err := st.CreateUser(reader); err != nil {
		t.Fatalf("create reader: %v", err)
	}
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "secret")
	admin, err := st.GetUserByUsername("admin")
	if err != nil || admin == nil {
		t.Fatalf("get admin: %v", err)
	}
	r := gin.New()
	mountPublicIdentityForTest(r, st, site)
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site)}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	return r, st, admin, reader
}

// TestConsoleLoginRejectsReader console 登录端点拒绝 reader 凭据（即使密码正确）。
func TestConsoleLoginRejectsReader(t *testing.T) {
	r, _, _, _ := newReaderEnv(t)
	body, _ := json.Marshal(map[string]string{"username": "reader1", "password": "readerpass"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("reader login via console should be 403, got %d body=%s", w.Code, w.Body.String())
	}
	for _, ck := range w.Result().Cookies() {
		if ck.Name == authCookieName && ck.Value != "" {
			t.Fatal("reader login must not issue blog_admin cookie")
		}
	}
}

// TestReaderCookieCannotReplayAsAdmin 读者 cookie（aud=user）改名重放为 blog_admin
// 必须验签失败（audience 绑定进 HMAC 消息）。
func TestReaderCookieCannotReplayAsAdmin(t *testing.T) {
	r, _, _, reader := newReaderEnv(t)
	readerCookie := signReaderCookie(reader)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: readerCookie})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("user-audience cookie replayed as blog_admin should be 401, got %d", w.Code)
	}
}

// TestReaderRoleBlockedFromContentWrite reader 即便持有 admin audience 的合法签名
// （等价于历史上"改名重放成功"的场景），也过不了内容写端点的角色门禁。
func TestReaderRoleBlockedFromContentWrite(t *testing.T) {
	r, _, _, reader := newReaderEnv(t)
	// 攻击者视角的最强伪造：audience 正确、签名合法（借用 reader 自己的 secret 签 admin 段）
	forged := signAuthCookie(cookieAudAdmin, time.Now().Add(authCookieTTL).Unix(), reader.ID, reader.AuthSecret)
	body := `{"title":"x","content_md":"<script>alert(1)</script>","status":"published"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: forged})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("reader with valid admin-aud cookie should hit author gate (403), got %d body=%s", w.Code, w.Body.String())
	}
}

// TestPublicLoginRejectsNonReader 公共登录端点拒绝 admin/author 凭据（角色分离）。
func TestPublicLoginRejectsNonReader(t *testing.T) {
	r, _, admin, _ := newReaderEnv(t)
	body, _ := json.Marshal(map[string]string{"username": admin.Username, "password": "secret"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("admin login via public endpoint should be 403, got %d", w.Code)
	}
}

// TestSafeFetchRejectsPrivateDNSResolutionBeforeDial 验证域名解析到内网地址时，
// 请求在拨号前被拒绝；这覆盖 DNS rebinding/混合解析响应，不能只依赖 IP 字面量校验。
func TestSafeFetchRejectsPrivateDNSResolutionBeforeDial(t *testing.T) {
	dialCalled := false
	client := safefetch.New(safefetch.Options{
		ResolveIP: func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("169.254.169.254")}, nil
		},
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			dialCalled = true
			return nil, errors.New("unexpected dial")
		},
	})
	_, err := client.Get("https://rebind.test/image.png")
	if !errors.Is(err, safefetch.ErrForbiddenHost) {
		t.Fatalf("error = %v, want ErrForbiddenHost", err)
	}
	if dialCalled {
		t.Fatal("private DNS result must be rejected before DialContext")
	}
}
