package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Wave 4 测试覆盖补强：内容 CRUD（页面全生命周期 + 瞬间发布/编辑/删除）。

// TestPageCRUDFlow 页面生命周期：创建（slug 冲突 409）→ 列表 → 详情 →
// 更新（改 slug 冲突 409）→ 删除 → 回收站 → 恢复 → 再删除 → 彻底删除。
func TestPageCRUDFlow(t *testing.T) {
	r, _, user, _ := newFullEnv(t)
	cookie := authCookie(user)

	// 创建
	w := do2FA(r, http.MethodPost, "/api/v1/pages",
		`{"slug":"about","title":"关于","content_md":"# 关于我","status":"published"}`, cookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("create page: status = %d, body = %s", w.Code, w.Body.String())
	}
	var page pageJSON
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode page: %v", err)
	}
	if page.Slug != "about" || page.Status != "published" || page.Title != "关于" {
		t.Errorf("created page mismatch: %+v", page)
	}
	createdAt := page.CreatedAt
	if createdAt == "" || strings.HasPrefix(createdAt, "0001-") {
		t.Fatalf("created page timestamp is empty: %+v", page)
	}

	// 创建：slug 空 → 400；slug 冲突 → 409
	w = do2FA(r, http.MethodPost, "/api/v1/pages", `{"slug":"","title":"x"}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("create page empty slug: status = %d, want 400", w.Code)
	}
	w = do2FA(r, http.MethodPost, "/api/v1/pages", `{"slug":"about","title":"dup"}`, cookie)
	if w.Code != http.StatusConflict {
		t.Errorf("create page dup slug: status = %d, want 409", w.Code)
	}

	// 列表
	w = do2FA(r, http.MethodGet, "/api/v1/pages", "", cookie)
	var list struct {
		Items []pageJSON `json:"items"`
		Total int        `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || list.Total != 1 {
		t.Fatalf("pages list: %+v (err %v), want 1", list, err)
	}

	// 详情 + 404
	w = do2FA(r, http.MethodGet, "/api/v1/pages/about", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("get page: status = %d", w.Code)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/pages/no-such", "", cookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("missing page: status = %d, want 404", w.Code)
	}

	// 更新：改标题与内容
	w = do2FA(r, http.MethodPut, "/api/v1/pages/about",
		`{"slug":"about","title":"关于我","content_md":"# 新内容","status":"published"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("update page: status = %d, body = %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &page)
	if page.Title != "关于我" {
		t.Errorf("updated title = %q, want 关于我", page.Title)
	}
	if page.CreatedAt != createdAt || strings.HasPrefix(page.CreatedAt, "0001-") {
		t.Errorf("update must preserve created_at: before=%q after=%q", createdAt, page.CreatedAt)
	}

	// 更新：slug 被占用 → 409（先建第二页）
	do2FA(r, http.MethodPost, "/api/v1/pages", `{"slug":"links","title":"友链"}`, cookie)
	w = do2FA(r, http.MethodPut, "/api/v1/pages/about", `{"slug":"links","title":"x"}`, cookie)
	if w.Code != http.StatusConflict {
		t.Errorf("update page occupied slug: status = %d, want 409", w.Code)
	}
	// 更新：不存在 → 404
	w = do2FA(r, http.MethodPut, "/api/v1/pages/no-such", `{"title":"x"}`, cookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("update missing page: status = %d, want 404", w.Code)
	}

	// 删除 → 回收站
	w = do2FA(r, http.MethodDelete, "/api/v1/pages/about", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("delete page: status = %d", w.Code)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/pages?status=trashed", "", cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 {
		t.Fatalf("trashed pages = %d, want 1", list.Total)
	}
	// 普通列表不再包含
	w = do2FA(r, http.MethodGet, "/api/v1/pages", "", cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 {
		t.Errorf("normal pages list should exclude trashed, total = %d", list.Total)
	}

	// 恢复
	w = do2FA(r, http.MethodPost, "/api/v1/pages/about/restore", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("restore page: status = %d", w.Code)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/pages/about", "", cookie)
	if w.Code != http.StatusOK {
		t.Errorf("restored page should be fetchable, got %d", w.Code)
	}

	// 再删除 → 彻底删除 → 404
	do2FA(r, http.MethodDelete, "/api/v1/pages/about", "", cookie)
	w = do2FA(r, http.MethodDelete, "/api/v1/pages/about/purge", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("purge page: status = %d", w.Code)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/pages/about", "", cookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("purged page should 404, got %d", w.Code)
	}
}

// TestMomentCRUDFlow 瞬间：发布（纯图允许）→ 列表 → 编辑 → 删除 → 参数校验。
func TestMomentCRUDFlow(t *testing.T) {
	r, _, user, _ := newFullEnv(t)
	cookie := authCookie(user)

	// 发布
	w := do2FA(r, http.MethodPost, "/api/v1/moments",
		`{"content_md":"今天的瞬间","visibility":"public"}`, cookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("create moment: status = %d, body = %s", w.Code, w.Body.String())
	}
	var m momentJSON
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode moment: %v", err)
	}
	if m.ID == 0 || strings.TrimSpace(m.Content) != "<p>今天的瞬间</p>" {
		t.Errorf("created moment mismatch: %+v", m)
	}

	// 空内容 → 400
	w = do2FA(r, http.MethodPost, "/api/v1/moments", `{"content_md":""}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("empty moment: status = %d, want 400", w.Code)
	}
	// 非法 JSON → 400
	w = do2FA(r, http.MethodPost, "/api/v1/moments", `{bad`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("bad json moment: status = %d, want 400", w.Code)
	}

	// 列表
	w = do2FA(r, http.MethodGet, "/api/v1/moments", "", cookie)
	var list struct {
		Items []momentJSON `json:"items"`
		Total int          `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || list.Total != 1 {
		t.Fatalf("moments list: %+v (err %v), want 1", list, err)
	}

	// 编辑
	w = do2FA(r, http.MethodPut, "/api/v1/moments/"+itoa(m.ID),
		`{"content_md":"已编辑","visibility":"private"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("update moment: status = %d, body = %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	if m.Visibility != "private" || strings.TrimSpace(m.Content) != "<p>已编辑</p>" {
		t.Errorf("updated moment mismatch: %+v", m)
	}
	// 编辑：不存在 → 404；非法 id → 400；空内容 → 400
	w = do2FA(r, http.MethodPut, "/api/v1/moments/99999", `{"content_md":"x"}`, cookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("update missing moment: status = %d, want 404", w.Code)
	}
	w = do2FA(r, http.MethodPut, "/api/v1/moments/abc", `{"content_md":"x"}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("update bad id: status = %d, want 400", w.Code)
	}
	w = do2FA(r, http.MethodPut, "/api/v1/moments/"+itoa(m.ID), `{"content_md":""}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("update empty moment: status = %d, want 400", w.Code)
	}

	// 删除
	w = do2FA(r, http.MethodDelete, "/api/v1/moments/"+itoa(m.ID), "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("delete moment: status = %d", w.Code)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/moments", "", cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 0 {
		t.Errorf("moments after delete = %d, want 0", list.Total)
	}
	// 删除：非法 id → 400
	w = do2FA(r, http.MethodDelete, "/api/v1/moments/abc", "", cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("delete bad id: status = %d, want 400", w.Code)
	}
}
