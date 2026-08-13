package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	backuppkg "blog/internal/backup"
	"blog/internal/comment"
	"blog/internal/post"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

// Wave 4 测试覆盖补强：共享完整环境助手。
// 与 authEnhanceEnv 的区别：额外装配文章服务（Render 依赖）与评论/备份服务，
// 供文章 CRUD、评论管理、设置、备份导出等 handler 测试使用。

// newFullEnv 完整测试环境：密码站点 + users 表 + 文章/评论/备份服务。
// 返回 engine、store、admin 用户与 handler。
func newFullEnv(t *testing.T) (*gin.Engine, *store.Store, *store.User, *Handler) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	if err := st.EnsureSchema(); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if err := st.EnsureUsersTable(); err != nil {
		t.Fatalf("ensure users table: %v", err)
	}
	user := &store.User{
		Username:     "admin",
		PasswordHash: "x",
		AuthSecret:   store.GenerateAuthSecret(),
		Role:         store.RoleAdmin,
	}
	if err := st.CreateUser(user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	site := &tmpl.Site{}
	site.Admin.Password = "secret"

	posts := post.NewService(t.TempDir())
	comments := comment.New(st, sitePtr(site))
	backups := backuppkg.New(backuppkg.Config{
		Store:     st,
		Posts:     posts,
		UploadDir: t.TempDir(),
		SitePath:  filepath.Join(t.TempDir(), "site.yml"),
		BackupDir: t.TempDir(),
	})

	r := gin.New()
	h, err := Mount(r, MountDeps{Config: Config{
		Store:    st,
		Posts:    posts,
		Site: sitePtr(site),
		Comments: comments,
		Backups:  backups,
	}})
	if err != nil {
		t.Fatalf("mount: %v", err)
	}
	return r, st, user, h
}

// do2FAReq 发起自定义请求（multipart 等非 JSON body 场景），自动附加 auth cookie。
func do2FAReq(r *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// do2FARaw 发起带 cookie 的原始 body 请求（非 JSON Content-Type）。
func do2FARaw(r *gin.Engine, method, path string, body *bytes.Buffer, contentType, cookie string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", contentType)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: authCookieName, Value: cookie})
	}
	r.ServeHTTP(w, req)
	return w
}
