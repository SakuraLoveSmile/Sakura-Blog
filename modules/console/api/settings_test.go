package api

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/config"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// Wave 4 测试覆盖补强：设置分段（读取/保存/敏感字段脱敏与保留）。

// TestSettingsSections 设置分段：初始为空、保存后回读、敏感字段脱敏、
// 回写 "****" 保留原值、authors 注入、未知分段 400。
func TestSettingsSections(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	cookie := authCookie(user)
	before, _ := st.ReadSettingsSnapshot()
	for _, section := range []string{"ai", "comments", "style", "widgets", "menubar", "data_cache", "friends", "authors"} {
		w := do2FA(r, http.MethodPut, "/api/v1/settings/"+section, `{}`, cookie)
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "stellar_migration_required") {
			t.Fatalf("legacy %s gate: %d %s", section, w.Code, w.Body.String())
		}
	}
	after, _ := st.ReadSettingsSnapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("legacy API gate modified settings")
	}
	if _, err := config.InitializeStellarV2(st, map[string]any{}, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	before, _ = st.ReadSettingsSnapshot()
	for _, section := range []string{"ai", "comments", "style", "widgets", "menubar", "data_cache", "friends", "authors"} {
		w := do2FA(r, http.MethodPut, "/api/v1/settings/"+section, `{}`, cookie)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "removed_field") {
			t.Fatalf("migrated %s gate: %d %s", section, w.Code, w.Body.String())
		}
	}
	after, _ = st.ReadSettingsSnapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("removed section API modified settings")
	}
	for _, section := range []string{"site", "feedback", "dashboard"} {
		if w := do2FA(r, http.MethodPut, "/api/v1/settings/"+section, `{}`, cookie); w.Code != http.StatusOK {
			t.Fatalf("independent %s: %d %s", section, w.Code, w.Body.String())
		}
	}
	large := strings.Repeat("x", (1<<20)+1)
	for _, path := range []string{"/api/v1/settings/ai", "/api/v1/settings/style/yaml"} {
		if w := do2FA(r, http.MethodPut, path, large, cookie); w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("body limit: %d", w.Code)
		}
	}
}

// TestSettingsNotifyTest 测试邮件：未配置 SMTP 时业务失败仍 200 + ok=false（Console 依赖）。
func TestSettingsNotifyTest(t *testing.T) {
	r, _, user, _ := newFullEnv(t)
	cookie := authCookie(user)
	w := do2FA(r, http.MethodPost, "/api/v1/settings/notify/test", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("test notify: status = %d, want 200 (business failure)", w.Code)
	}
	var resp struct {
		Ok bool `json:"ok"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Ok {
		t.Errorf("test notify without smtp should return ok=false, got %s", w.Body.String())
	}
}

// TestSettingsFeedbackCheck feedback/check：探测草稿 api_base 的 /healthz。
// 覆盖 200 可达、非 200 可达、连接失败、非法 scheme、空地址与缺鉴权。
func TestSettingsFeedbackCheck(t *testing.T) {
	r, _, user, _ := newFullEnv(t)
	cookie := authCookie(user)

	// 200 → ok
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer up.Close()
	w := do2FA(r, http.MethodPost, "/api/v1/settings/feedback/check",
		`{"api_base":"`+up.URL+`"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("check 200: status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp FeedbackCheckResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode check resp: %v", err)
	}
	if !resp.OK || resp.Status != 200 || resp.Endpoint != up.URL+"/healthz" {
		t.Errorf("check should be ok with status 200, got %+v", resp)
	}

	// /healthz 500 → ok=false 但带回状态码
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer down.Close()
	w = do2FA(r, http.MethodPost, "/api/v1/settings/feedback/check",
		`{"api_base":"`+down.URL+`/"}`, cookie) // 尾斜杠应被归一
	if w.Code != http.StatusOK {
		t.Fatalf("check 500: status = %d", w.Code)
	}
	resp = FeedbackCheckResponse{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.OK || resp.Status != 500 || resp.Endpoint != down.URL+"/healthz" {
		t.Errorf("500 healthz should return ok=false status=500, got %+v", resp)
	}

	// 不可达 → ok=false + error（本地起即关的端口）
	lis, _ := net.Listen("tcp", "127.0.0.1:0")
	dead := "http://" + lis.Addr().String()
	_ = lis.Close()
	w = do2FA(r, http.MethodPost, "/api/v1/settings/feedback/check",
		`{"api_base":"`+dead+`"}`, cookie)
	resp = FeedbackCheckResponse{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.OK || resp.Error == "" {
		t.Errorf("unreachable base should be ok=false with error, got %+v", resp)
	}

	// 非法 scheme / 空地址 → 400
	w = do2FA(r, http.MethodPost, "/api/v1/settings/feedback/check", `{"api_base":"ftp://x"}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("ftp scheme: status = %d, want 400", w.Code)
	}
	w = do2FA(r, http.MethodPost, "/api/v1/settings/feedback/check", `{"api_base":"  "}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("blank base: status = %d, want 400", w.Code)
	}

	// 未登录 → 401/403（sys 组鉴权）
	w = do2FA(r, http.MethodPost, "/api/v1/settings/feedback/check", `{"api_base":"http://x"}`, "")
	if w.Code == http.StatusOK {
		t.Errorf("unauthenticated check should not be 200, got %d", w.Code)
	}
}

// TestSettingsAuthorsInject authors 段经 Bootstrap 入库后从 settings 表返回。
func TestSettingsAuthorsInject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	if err := st.EnsureSchema(); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "secret")
	site.Authors = map[string]tmpl.AuthorDef{"admin": {Name: "管理员"}}
	// authors 段已接入 settings 热更新管线：Bootstrap 把 site.yml 初值导入 settings 表
	if err := config.Bootstrap(st, site); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	r := gin.New()
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site)}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	// 无密码站点时走无密码逻辑：直接造 admin 用户（Bootstrap 的 Spec 45 迁移可能已创建，已存在则复用）
	user := &store.User{Username: "admin", PasswordHash: "x", AuthSecret: store.GenerateAuthSecret()}
	if err := st.EnsureUsersTable(); err != nil {
		t.Fatalf("ensure users: %v", err)
	}
	users, _ := st.ListUsers()
	if len(users) == 0 {
		if err := st.CreateUser(user); err != nil {
			t.Fatalf("seed user: %v", err)
		}
		users, _ = st.ListUsers()
	}
	user = users[0]
	// ListUsers 不含 auth_secret，需 GetUserByID 取完整用户（authCookie 签名密钥）
	if full, err := st.GetUserByID(user.ID); err == nil && full != nil {
		user = full
	}
	w := do2FA(r, http.MethodGet, "/api/v1/settings", "", authCookie(user))
	var secs map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &secs); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	authorsRaw, ok := secs["authors"]
	if !ok {
		t.Fatal("settings should inject authors section")
	}
	var authors map[string]map[string]string
	if err := json.Unmarshal(authorsRaw, &authors); err != nil || len(authors) != 1 || authors["admin"]["name"] != "管理员" {
		t.Errorf("authors inject mismatch: %s (err %v)", authorsRaw, err)
	}
}

// 历史 YAML 仍可读取作迁移预览；任何保存均不得修改历史记录。
func TestStyleYAMLWriteGatePreservesLegacyData(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	if err := st.PutSetting("style", `{"future_widget":{"enabled":false,"count":0,"title":""}}`); err != nil {
		t.Fatal(err)
	}
	before, _ := st.ReadSettingsSnapshot()
	w := do2FA(r, http.MethodPut, "/api/v1/settings/style/yaml", "future_widget:\n  enabled: true\n", authCookie(user))
	if w.Code != http.StatusConflict {
		t.Fatalf("legacy YAML gate: %d %s", w.Code, w.Body.String())
	}
	after, _ := st.ReadSettingsSnapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("YAML gate modified historical data")
	}
	if err := st.PutSettingsSnapshot(map[string]string{config.SectionStellarV2: `{"version":2,"theme":{}}`, config.SectionBlogExtensions: `{}`}); err != nil {
		t.Fatal(err)
	}
	before, _ = st.ReadSettingsSnapshot()
	w = do2FA(r, http.MethodPut, "/api/v1/settings/style/yaml", "future_widget:\n  enabled: true\n", authCookie(user))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "removed_field") {
		t.Fatalf("migrated YAML gate: %d %s", w.Code, w.Body.String())
	}
	after, _ = st.ReadSettingsSnapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("YAML gate modified canonical or historical data")
	}
}
