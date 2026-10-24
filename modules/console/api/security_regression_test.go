package api

// 安全审计 2026-08-16 回归测试：认证链修复后的关键不变量。
//  1. reader 不能经 console 登录端点换取 blog_admin 会话；
//  2. 读者 cookie（audience=user）改名重放为 blog_admin 验签失败；
//  3. 即使持有 admin audience 的合法签名，reader 角色也过不了内容写端点的
//     content:write 权限门禁；
//  4. 备份恢复上传在请求体超限时被 MaxBytesReader 拒绝。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
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
	body := `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","title":"x","content_md":"<script>alert(1)</script>","status":"published"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: forged})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("reader with valid admin-aud cookie should hit author gate (403), got %d body=%s", w.Code, w.Body.String())
	}
}

// TestAuthorCannotWriteAnotherAuthorsPost 验证文章所有写入口都以
// post_authors.users.id 做对象级授权，而不是仅依赖 author 角色门禁。
func TestAuthorCannotWriteAnotherAuthorsPost(t *testing.T) {
	r, st, admin, _ := newFullStellarEnv(t)
	authorA, err := st.GetUserByUsername("alice")
	if err != nil || authorA == nil {
		t.Fatalf("get author A: %v", err)
	}
	authorB, err := st.GetUserByUsername("bob")
	if err != nil || authorB == nil {
		t.Fatalf("get author B: %v", err)
	}
	post := &store.Post{Slug: "author-a-post", Title: "A 的文章", Status: store.StatusPublished, AuthorIDs: []int64{authorA.ID}}
	if _, err := st.Create(post); err != nil {
		t.Fatalf("create owned post: %v", err)
	}
	revisionID, err := st.CreateRevision(&store.Revision{PostSlug: post.Slug, Title: post.Title, ContentMD: "旧正文"})
	if err != nil {
		t.Fatalf("create revision: %v", err)
	}
	shareID, err := st.CreateShare(&store.SharedPost{PostSlug: post.Slug, ShareType: "published"})
	if err != nil {
		t.Fatalf("create share: %v", err)
	}

	cases := []struct {
		name, method, path, body string
	}{
		{"update", http.MethodPut, "/api/v1/posts/author-a-post", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","title":"越权"}`},
		{"save draft", http.MethodPut, "/api/v1/posts/author-a-post/draft", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","title":"越权草稿"}`},
		{"publish", http.MethodPost, "/api/v1/posts/author-a-post/publish", `{}`},
		{"discard draft", http.MethodDelete, "/api/v1/posts/author-a-post/draft", ""},
		{"ai summary", http.MethodPut, "/api/v1/posts/author-a-post/ai-summary", `{"summary":"越权摘要"}`},
		{"delete", http.MethodDelete, "/api/v1/posts/author-a-post", ""},
		{"restore", http.MethodPost, "/api/v1/posts/author-a-post/restore", ""},
		{"purge", http.MethodDelete, "/api/v1/posts/author-a-post/purge", ""},
		{"restore revision", http.MethodPost, "/api/v1/revisions/" + strconv.FormatInt(revisionID, 10) + "/restore", ""},
		{"create share", http.MethodPost, "/api/v1/posts/author-a-post/shares", `{}`},
		{"delete share", http.MethodDelete, "/api/v1/shares/" + strconv.FormatInt(shareID, 10), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := do2FA(r, tc.method, tc.path, tc.body, authCookie(authorB))
			if w.Code != http.StatusForbidden {
				t.Fatalf("status=%d body=%s, want 403", w.Code, w.Body.String())
			}
		})
	}

	// author 新建文章时，客户端提交的 bob 不能替换服务端确定的 alice 归属。
	w := do2FA(r, http.MethodPost, "/api/v1/posts", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","slug":"author-created","title":"A 新建","authors":["bob"],"status":"published"}`, authCookie(authorA))
	if w.Code != http.StatusCreated {
		t.Fatalf("author create status=%d body=%s, want 201", w.Code, w.Body.String())
	}
	created, err := st.GetBySlug("author-created")
	if err != nil || created == nil {
		t.Fatalf("get author-created: %v", err)
	}
	if len(created.AuthorIDs) != 1 || created.AuthorIDs[0] != authorA.ID {
		t.Fatalf("author-created ids=%v, want [%d]", created.AuthorIDs, authorA.ID)
	}

	// 归属作者可以写；管理员仍保留全通语义。
	w = do2FA(r, http.MethodPut, "/api/v1/posts/author-a-post", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","title":"A 更新","authors":["bob"]}`, authCookie(authorA))
	if w.Code != http.StatusOK {
		t.Fatalf("owner update status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	w = do2FA(r, http.MethodPut, "/api/v1/posts/author-a-post", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","title":"管理员更新"}`, authCookie(admin))
	if w.Code != http.StatusOK {
		t.Fatalf("admin update status=%d body=%s, want 200", w.Code, w.Body.String())
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
