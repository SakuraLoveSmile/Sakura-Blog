package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"blog/internal/store"
)

// Wave 4 测试覆盖补强：杂项管理接口（分类/标签/专题/仪表盘/统计/版本/模块开关）。

// TestTaxonomyFlow 分类与标签：创建文章 → 计数列表 → 重命名 → 删除 → 合并。
func TestTaxonomyFlow(t *testing.T) {
	r, _, user, _ := newFullStellarEnv(t)
	cookie := authCookie(user)

	// 两篇文章：一篇 Go/后端，一篇 Go/前端
	do2FA(r, http.MethodPost, "/api/v1/posts",
		`{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","title":"Go 后端","slug":"go-backend","category":"后端","tags":"go,后端","status":"published"}`, cookie)
	do2FA(r, http.MethodPost, "/api/v1/posts",
		`{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","title":"Go 前端","slug":"go-front","category":"前端","tags":"go,前端","status":"published"}`, cookie)

	// 分类计数：按 count 降序
	w := do2FA(r, http.MethodGet, "/api/v1/categories", "", cookie)
	var cats struct {
		Items []taxonomyItem `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &cats); err != nil {
		t.Fatalf("decode categories: %v", err)
	}
	if len(cats.Items) != 2 {
		t.Fatalf("categories = %+v, want 2", cats.Items)
	}
	for _, it := range cats.Items {
		if it.Count != 1 {
			t.Errorf("category %q count = %d, want 1", it.Name, it.Count)
		}
	}

	// 标签计数
	w = do2FA(r, http.MethodGet, "/api/v1/tags", "", cookie)
	var tags struct {
		Items []taxonomyItem `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &tags)
	if len(tags.Items) != 3 {
		t.Errorf("tags = %+v, want 3 (go,后端,前端)", tags.Items)
	}

	// 重命名分类
	w = do2FA(r, http.MethodPut, "/api/v1/categories/后端", `{"name":"Backend"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("rename category: status = %d", w.Code)
	}
	var aff struct {
		Affected int `json:"affected"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &aff)
	if aff.Affected != 1 {
		t.Errorf("rename category affected = %d, want 1", aff.Affected)
	}
	// 重命名：空名 → 400；非法 JSON → 400
	w = do2FA(r, http.MethodPut, "/api/v1/categories/x", `{"name":""}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("rename empty: status = %d, want 400", w.Code)
	}
	w = do2FA(r, http.MethodPut, "/api/v1/categories/x", `{bad`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("rename bad json: status = %d, want 400", w.Code)
	}

	// 删除分类（清空文章分类）
	w = do2FA(r, http.MethodDelete, "/api/v1/categories/前端", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("delete category: status = %d", w.Code)
	}

	// 重命名标签
	w = do2FA(r, http.MethodPut, "/api/v1/tags/后端", `{"name":"backend"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("rename tag: status = %d", w.Code)
	}
	w = do2FA(r, http.MethodPut, "/api/v1/tags/前端", `{"name":""}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("rename tag empty: status = %d, want 400", w.Code)
	}

	// 合并标签：go+backend → golang
	w = do2FA(r, http.MethodPost, "/api/v1/tags/merge", `{"sources":["go","backend"],"target":"golang"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("merge tags: status = %d, body = %s", w.Code, w.Body.String())
	}
	// 合并：缺参数 → 400
	w = do2FA(r, http.MethodPost, "/api/v1/tags/merge", `{"sources":[],"target":""}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("merge empty: status = %d, want 400", w.Code)
	}
	w = do2FA(r, http.MethodPost, "/api/v1/tags/merge", `{bad`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("merge bad json: status = %d, want 400", w.Code)
	}

	// 删除标签
	w = do2FA(r, http.MethodDelete, "/api/v1/tags/前端", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("delete tag: status = %d", w.Code)
	}
	// 最终标签应只剩 golang
	w = do2FA(r, http.MethodGet, "/api/v1/tags", "", cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &tags)
	if len(tags.Items) != 1 || tags.Items[0].Name != "golang" {
		t.Errorf("final tags = %+v, want [golang]", tags.Items)
	}
}

// TestTopicFlow 专题：创建（slug 自动生成/冲突 409）→ 列表（含 post_count）→ 更新 → 删除。
func TestTopicFlow(t *testing.T) {
	r, _, user, _ := newFullStellarEnv(t)
	cookie := authCookie(user)

	// 创建
	w := do2FA(r, http.MethodPost, "/api/v1/topics",
		`{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","name":"Go 专题","description":"Go 语言","icon":"go","sort":1}`, cookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("create topic: status = %d, body = %s", w.Code, w.Body.String())
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || created.ID == 0 {
		t.Fatalf("decode created topic: %+v (err %v)", created, err)
	}
	topicID := created.ID
	// 重复 name → 409
	w = do2FA(r, http.MethodPost, "/api/v1/topics", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","name":"Go 专题"}`, cookie)
	if w.Code != http.StatusConflict {
		t.Errorf("dup topic name: status = %d, want 409", w.Code)
	}
	// 空 name → 400
	w = do2FA(r, http.MethodPost, "/api/v1/topics", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","name":""}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("empty topic name: status = %d, want 400", w.Code)
	}

	// 列表（无文章，post_count=0）
	w = do2FA(r, http.MethodGet, "/api/v1/topics", "", cookie)
	var list struct {
		Items []struct {
			ID        int64 `json:"id"`
			PostCount int   `json:"post_count"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || list.Total != 1 {
		t.Fatalf("topics list: %+v (err %v)", list, err)
	}
	if list.Items[0].PostCount != 0 {
		t.Errorf("topic post_count = %d, want 0", list.Items[0].PostCount)
	}

	// 更新
	w = do2FA(r, http.MethodPut, "/api/v1/topics/"+itoa(topicID),
		`{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","name":"Go 合集","description":"更新","sort":2}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("update topic: status = %d", w.Code)
	}
	// 更新不存在 → 404
	w = do2FA(r, http.MethodPut, "/api/v1/topics/99999", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","name":"x"}`, cookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("update missing topic: status = %d, want 404", w.Code)
	}

	// 删除
	w = do2FA(r, http.MethodDelete, "/api/v1/topics/"+itoa(topicID), "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("delete topic: status = %d", w.Code)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/topics", "", cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 0 {
		t.Errorf("topics after delete = %d, want 0", list.Total)
	}
}

// TestDashboardStats 仪表盘：统计、最近评论、快捷操作读/写。
func TestDashboardStats(t *testing.T) {
	r, st, user, _ := newFullStellarEnv(t)
	cookie := authCookie(user)

	// 先造数据：1 篇文章 + 1 条已审评论（最近评论接口只查 approved）
	do2FA(r, http.MethodPost, "/api/v1/posts",
		`{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","title":"统计","slug":"stats-post","status":"published"}`, cookie)
	seedComment(t, st, &store.Comment{PageKey: "/post/stats-post", Nick: "n", ContentMD: "c", Status: store.CommentApproved})

	// 仪表盘统计
	w := do2FA(r, http.MethodGet, "/api/v1/dashboard/stats", "", cookie)
	var ds struct {
		PostTotal      int `json:"post_total"`
		PostPublished  int `json:"post_published"`
		CommentTotal   int `json:"comment_total"`
		CommentPending int `json:"comment_pending"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &ds); err != nil {
		t.Fatalf("decode dashboard stats: %v", err)
	}
	if ds.PostTotal != 1 || ds.PostPublished != 1 || ds.CommentTotal != 1 || ds.CommentPending != 0 {
		t.Errorf("dashboard stats = %+v, want 1/1/1/0", ds)
	}

	// 最近评论
	w = do2FA(r, http.MethodGet, "/api/v1/dashboard/recent-comments", "", cookie)
	var rc struct {
		Items []commentJSON `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &rc); err != nil || len(rc.Items) != 1 {
		t.Errorf("recent comments: %+v (err %v)", rc, err)
	}

	// 快捷操作：默认列表
	w = do2FA(r, http.MethodGet, "/api/v1/dashboard/quick-actions", "", cookie)
	var qa struct {
		Items []string `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &qa); err != nil || len(qa.Items) != len(defaultQuickActions) {
		t.Errorf("default quick actions: %+v (err %v)", qa, err)
	}
	// 保存（去重去空 + 超量截断）
	w = do2FA(r, http.MethodPut, "/api/v1/dashboard/quick-actions",
		`{"items":["a","a","","b","c","d","e","f","g","h","i"]}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("save quick actions: status = %d", w.Code)
	}
	_ = json.Unmarshal(w.Body.Bytes(), &qa)
	if len(qa.Items) != maxQuickActions || qa.Items[0] != "a" {
		t.Errorf("saved quick actions = %+v, want %d items starting with a", qa.Items, maxQuickActions)
	}
	// 回读一致
	w = do2FA(r, http.MethodGet, "/api/v1/dashboard/quick-actions", "", cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &qa)
	if len(qa.Items) != maxQuickActions {
		t.Errorf("reload quick actions = %d items, want %d", len(qa.Items), maxQuickActions)
	}
	// 非法 JSON → 400
	w = do2FA(r, http.MethodPut, "/api/v1/dashboard/quick-actions", `{bad`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("save quick actions bad json: status = %d, want 400", w.Code)
	}
}

// TestStatsEndpoints 统计四接口：trend/popular/referrers/hourly + days 参数解析。
func TestStatsEndpoints(t *testing.T) {
	r, _, user, _ := newFullEnv(t)
	cookie := authCookie(user)

	for _, path := range []string{
		"/api/v1/stats/trend?days=30",
		"/api/v1/stats/popular?days=30&limit=5",
		"/api/v1/stats/referrers?days=7",
		"/api/v1/stats/hourly?days=7",
	} {
		w := do2FA(r, http.MethodGet, path, "", cookie)
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s: status = %d, body = %s", path, w.Code, w.Body.String())
		}
	}
	// 非法 days 回退 7
	w := do2FA(r, http.MethodGet, "/api/v1/stats/trend?days=999", "", cookie)
	var trend struct {
		Days int `json:"days"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &trend)
	if trend.Days != 7 {
		t.Errorf("trend invalid days fallback = %d, want 7", trend.Days)
	}
	// popular limit 越界回退 10
	w = do2FA(r, http.MethodGet, "/api/v1/stats/popular?limit=999", "", cookie)
	if w.Code != http.StatusOK {
		t.Errorf("popular: status = %d", w.Code)
	}
}

// TestRevisionEndpoints 版本：列表（空）/非法 id/不存在。
func TestRevisionEndpoints(t *testing.T) {
	r, _, user, _ := newFullEnv(t)
	cookie := authCookie(user)

	// 列表：无版本 → 空数组
	w := do2FA(r, http.MethodGet, "/api/v1/posts/no-such/revisions", "", cookie)
	var list struct {
		Items []json.RawMessage `json:"items"`
		Total int               `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || list.Items == nil {
		t.Errorf("revisions empty list: %+v (err %v), want []", list, err)
	}
	// 单版本：不存在 → 404；非法 id → 400
	w = do2FA(r, http.MethodGet, "/api/v1/revisions/99999", "", cookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("get missing revision: status = %d, want 404", w.Code)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/revisions/abc", "", cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("get bad revision id: status = %d, want 400", w.Code)
	}
	// 回滚：不存在 → 404；非法 id → 400
	w = do2FA(r, http.MethodPost, "/api/v1/revisions/99999/restore", "", cookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("restore missing revision: status = %d, want 404", w.Code)
	}
	w = do2FA(r, http.MethodPost, "/api/v1/revisions/abc/restore", "", cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("restore bad revision id: status = %d, want 400", w.Code)
	}
}

// TestModuleEndpoints 模块：未装配时列表空数组、开关 503。
func TestModuleEndpoints(t *testing.T) {
	r, _, user, _ := newFullEnv(t)
	cookie := authCookie(user)

	w := do2FA(r, http.MethodGet, "/api/v1/modules", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("list modules: status = %d", w.Code)
	}
	var list struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || list.Items == nil {
		t.Errorf("modules list = %+v (err %v), want []", list, err)
	}
	w = do2FA(r, http.MethodPut, "/api/v1/modules/seo", `{"enabled":true}`, cookie)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("toggle module without manager: status = %d, want 503", w.Code)
	}
}
