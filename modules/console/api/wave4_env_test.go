package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	backuppkg "blog/internal/backup"
	"blog/internal/comments"
	"blog/internal/config"
	"blog/internal/content"
	"blog/internal/post"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// Wave 4 测试覆盖补强：共享完整环境助手。
// 与 authEnhanceEnv 的区别：额外装配文章服务（Render 依赖）与评论/备份服务，
// 供文章 CRUD、评论管理、设置、备份导出等 handler 测试使用。

// newFullEnv 完整测试环境：密码站点 + users 表 + 文章/评论/备份服务。
// 返回 engine、store、admin 用户与 handler。
func newFullEnv(t *testing.T) (*gin.Engine, *store.Store, *store.User, *Handler) {
	return newFullEnvWithStellar(t, false)
}
func newFullStellarEnv(t *testing.T) (*gin.Engine, *store.Store, *store.User, *Handler) {
	return newFullEnvWithStellar(t, true)
}
func newFullEnvWithStellar(t *testing.T, stellar bool) (*gin.Engine, *store.Store, *store.User, *Handler) {
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
	for _, username := range []string{"alice", "bob"} {
		if err := st.CreateUser(&store.User{
			Username: username, PasswordHash: "x", AuthSecret: store.GenerateAuthSecret(), Role: store.RoleAuthor,
		}); err != nil {
			t.Fatalf("create author %s: %v", username, err)
		}
	}
	site := &tmpl.Site{}
	if stellar {
		strict, err := config.InitializeStellarV2(st, map[string]any{}, map[string]any{})
		if err != nil {
			t.Fatalf("initialize isolated Stellar fixture: %v", err)
		}
		site.StellarV2 = strict
	}
	enableAdmin(t, st, site, "secret")

	posts := post.NewService(t.TempDir())
	commentH := comments.New(st, sitePtr(site))
	backups := backuppkg.New(backuppkg.Config{
		Store:     st,
		Posts:     posts,
		UploadDir: t.TempDir(),
		SitePath:  filepath.Join(t.TempDir(), "site.yml"),
		BackupDir: t.TempDir(),
	})

	snapshot := sitePtr(site)
	r := gin.New()
	mountPublicIdentityForTest(r, st, site)
	var h *Handler
	var mountErr error
	h, mountErr = MountConsole(r, MountDeps{Config: Config{
		Store: st,
		RenderStellarMarkdown: func(input content.StellarBodyInput) (string, error) {
			configuration := snapshot.Load().StellarV2
			if configuration == nil {
				var err error
				configuration, err = config.LoadStellarV2(st)
				if err != nil {
					return "", err
				}
			}
			language := input.Language
			if language == "" {
				language = content.ResolveStellarLanguage(input.Page, configuration.Blog)
			}
			data, _ := configuration.Blog["data"].(map[string]any)
			return posts.RenderStellarV2WithContext(input.Markdown, post.StellarRenderContext{Theme: configuration.Theme, Data: data, Page: input.Page, Language: language, BlogExtensions: true})
		},
		RenderStellarContentPreview: func(input content.StellarPreviewInput) (content.StellarPreviewOutput, error) {
			body := input.HTML
			if input.Markdown != nil {
				parsed, err := validateEditorMetadata(StellarContentMetadata{input.StellarVersion, input.StellarConfig, input.BlogExtensions}, input.Layout, false)
				if err != nil {
					return content.StellarPreviewOutput{}, err
				}
				body, err = h.renderStellarBody(content.StellarBodyInput{Markdown: *input.Markdown, Page: parsed.Value, Language: input.Language})
				if err != nil {
					return content.StellarPreviewOutput{}, err
				}
			}
			return content.StellarPreviewOutput{HTML: body, Document: "<!doctype html><html><body>" + body + "</body></html>"}, nil
		},
		Posts:    posts,
		Site:     snapshot,
		Comments: commentH,
		Backups:  backups,
	}})
	if mountErr != nil {
		t.Fatalf("mount: %v", mountErr)
	}
	return r, st, user, h
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
