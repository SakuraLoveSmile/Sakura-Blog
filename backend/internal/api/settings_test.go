package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
	tmpl "blog/internal/template"
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

// TestSettingsAuthorsInject site.yml authors 注入 settings 响应。
func TestSettingsAuthorsInject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	if err := st.EnsureSchema(); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	site := &tmpl.Site{}
	site.Admin.Password = "secret"
	site.Authors = map[string]tmpl.AuthorDef{"admin": {Name: "管理员"}}
	r := gin.New()
	if _, err := Mount(r, MountDeps{Config: Config{Store: st, Site: site}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	// 无密码站点时走无密码逻辑：直接造 admin 用户
	user := &store.User{Username: "admin", PasswordHash: "x", AuthSecret: store.GenerateAuthSecret()}
	if err := st.EnsureUsersTable(); err != nil || st.CreateUser(user) != nil {
		t.Fatalf("seed user: %v", err)
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
