package api

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
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

	// 初始为空对象
	w := do2FA(r, http.MethodGet, "/api/v1/settings", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("get settings: status = %d", w.Code)
	}
	var secs map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &secs); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if raw, ok := secs["comments"]; ok && string(raw) != "null" {
		t.Errorf("empty settings should not contain comments section: %s", w.Body.String())
	}

	// 保存 ai 段（含 api_key）
	w = do2FA(r, http.MethodPut, "/api/v1/settings/ai",
		`{"api_key":"sk-secret-123","model":"gpt-4o"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("save ai section: status = %d, body = %s", w.Code, w.Body.String())
	}

	// 保存 comments 段（含 notify.password）
	w = do2FA(r, http.MethodPut, "/api/v1/settings/comments",
		`{"notify":{"password":"smtp-pass","enabled":true},"moderation":true}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("save comments section: status = %d, body = %s", w.Code, w.Body.String())
	}

	// 回读：api_key / password 脱敏为 ****
	w = do2FA(r, http.MethodGet, "/api/v1/settings", "", cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &secs)
	var ai map[string]any
	if err := json.Unmarshal(secs["ai"], &ai); err != nil || ai["api_key"] != "****" {
		t.Errorf("ai section should mask api_key, got %v (err %v)", ai["api_key"], err)
	}
	var cm map[string]any
	if err := json.Unmarshal(secs["comments"], &cm); err != nil {
		t.Fatalf("decode comments section: %v", err)
	}
	notify, _ := cm["notify"].(map[string]any)
	if notify == nil || notify["password"] != "****" {
		t.Errorf("comments.notify.password should be masked, got %v", notify)
	}

	// 回写 "****" → 原值保留（脱敏不回写）
	w = do2FA(r, http.MethodPut, "/api/v1/settings/ai",
		`{"api_key":"****","model":"gpt-4o-mini"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("resave ai with masked key: status = %d", w.Code)
	}
	stored, err := st.GetSetting("ai")
	if err != nil || stored == "" {
		t.Fatalf("get stored ai: %v", err)
	}
	var storedAI map[string]any
	_ = json.Unmarshal([]byte(stored), &storedAI)
	if storedAI["api_key"] != "sk-secret-123" {
		t.Errorf("masked key should preserve original, got %v", storedAI["api_key"])
	}
	if storedAI["model"] != "gpt-4o-mini" {
		t.Errorf("model should update, got %v", storedAI["model"])
	}

	// 回写空 password → 原值保留
	w = do2FA(r, http.MethodPut, "/api/v1/settings/comments",
		`{"notify":{"password":"","enabled":false}}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("resave comments empty pw: status = %d", w.Code)
	}
	stored, _ = st.GetSetting("comments")
	var storedCM map[string]any
	_ = json.Unmarshal([]byte(stored), &storedCM)
	storedNotify, _ := storedCM["notify"].(map[string]any)
	if storedNotify == nil || storedNotify["password"] != "smtp-pass" {
		t.Errorf("empty password should preserve original, got %v", storedNotify)
	}

	// 非法 JSON → 400
	w = do2FA(r, http.MethodPut, "/api/v1/settings/ai", `{bad`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid settings json: status = %d, want 400", w.Code)
	}
	// JSON settings over the request limit return 413, while bounded malformed
	// JSON remains a 400 validation error.
	large := strings.Repeat("x", (1<<20)+1)
	w = do2FA(r, http.MethodPut, "/api/v1/settings/ai", large, cookie)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized JSON settings: status = %d, want 413", w.Code)
	}
	w = do2FA(r, http.MethodPut, "/api/v1/settings/style/yaml", strings.Repeat("x", (1<<20)+1), cookie)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized YAML settings: status = %d, want 413", w.Code)
	}
	w = do2FA(r, http.MethodPut, "/api/v1/settings/style/yaml", "[", cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("malformed YAML settings: status = %d, want 400", w.Code)
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

// 合并 B 的原子设置保存与 D 的 YAML 未知字段契约，不能因规范化而丢字段。
func TestStyleYAMLPreservesUnknownFieldsAfterAtomicSave(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	w := do2FA(r, http.MethodPut, "/api/v1/settings/style/yaml", "future_widget:\n  enabled: false\n  count: 0\n  title: \"\"\n", authCookie(user))
	if w.Code != http.StatusOK {
		t.Fatalf("save YAML: %d %s", w.Code, w.Body.String())
	}
	var response StyleYAMLUpdateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || !response.OK {
		t.Fatalf("typed response: %v", err)
	}
	raw, err := st.GetSetting("style")
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &saved); err != nil {
		t.Fatal(err)
	}
	var extra map[string]any
	if err := json.Unmarshal(saved["future_widget"], &extra); err != nil {
		t.Fatal(err)
	}
	if extra["enabled"] != false || extra["count"] != float64(0) || extra["title"] != "" {
		t.Fatalf("unknown YAML fields lost: %v", extra)
	}
}
