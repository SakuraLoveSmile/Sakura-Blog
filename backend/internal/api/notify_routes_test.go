package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/notify"
	"blog/internal/store"
)

// Wave 3b 通知中心路由测试：认证 401 / 分页列表 / 未读数 / 单条与全部已读。

// notifyRoutesEnv 带密码站点 + admin 用户 + 注入通知服务的测试环境。
func notifyRoutesEnv(t *testing.T) (*gin.Engine, *store.Store, *store.User) {
	t.Helper()
	r, st, user, h := authEnhanceEnv(t)
	if err := st.EnsureNotificationsTable(); err != nil {
		t.Fatalf("ensure notifications: %v", err)
	}
	h.notify = notify.New(st)
	return r, st, user
}

// TestNotificationsUnauthenticated 未认证访问通知 API 一律 401。
func TestNotificationsUnauthenticated(t *testing.T) {
	r, _, _ := notifyRoutesEnv(t)
	cases := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/v1/notifications"},
		{http.MethodGet, "/api/v1/notifications/unread-count"},
		{http.MethodPut, "/api/v1/notifications/read-all"},
	}
	for _, tc := range cases {
		w := do2FA(r, tc.method, tc.path, "", "")
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without auth: status = %d, want 401", tc.method, tc.path, w.Code)
		}
	}
}

// TestNotificationsFlow 列表/未读数/单条已读/全部已读 全链路。
func TestNotificationsFlow(t *testing.T) {
	r, st, user := notifyRoutesEnv(t)
	cookie := authCookie(user)

	// 初始空列表
	w := do2FA(r, http.MethodGet, "/api/v1/notifications", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("list notifications: status = %d, body = %s", w.Code, w.Body.String())
	}
	var empty struct {
		Items  []*store.Notification `json:"items"`
		Total  int64                 `json:"total"`
		Unread int64                 `json:"unread"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &empty); err != nil {
		t.Fatalf("decode empty list: %v", err)
	}
	if empty.Total != 0 || empty.Unread != 0 || empty.Items == nil {
		t.Fatalf("empty list should be items=[] total=0 unread=0, got %+v", empty)
	}

	// 制造 2 条通知（1 条已读 1 条未读）
	for i, read := range []int{0, 1} {
		n := &store.Notification{
			UserID:    user.ID,
			Type:      "comment_new",
			Title:     "新评论：测试",
			Content:   "内容",
			Link:      "/posts/hello",
			Read:      read,
			CreatedAt: time.Now().UTC(),
		}
		if err := st.CreateNotification(n); err != nil {
			t.Fatalf("create notification %d: %v", i, err)
		}
	}

	// 未读数 = 1
	wc := do2FA(r, http.MethodGet, "/api/v1/notifications/unread-count", "", cookie)
	var uc struct {
		Count int64 `json:"count"`
	}
	if err := json.Unmarshal(wc.Body.Bytes(), &uc); err != nil || uc.Count != 1 {
		t.Fatalf("unread-count = %+v (err %v), want 1", uc, err)
	}

	// 列表：未读优先 → 第一条未读
	wl := do2FA(r, http.MethodGet, "/api/v1/notifications", "", cookie)
	var list struct {
		Items  []*store.Notification `json:"items"`
		Total  int64                 `json:"total"`
		Unread int64                 `json:"unread"`
	}
	if err := json.Unmarshal(wl.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if list.Total != 2 || len(list.Items) != 2 || list.Unread != 1 {
		t.Fatalf("list total=%d len=%d unread=%d, want 2/2/1", list.Total, len(list.Items), list.Unread)
	}
	if list.Items[0].Read != 0 {
		t.Errorf("list should be unread-first, got first read=%d", list.Items[0].Read)
	}
	firstID := list.Items[0].ID

	// 单条已读
	wm := do2FA(r, http.MethodPut, "/api/v1/notifications/"+strconv.FormatInt(firstID, 10)+"/read", "", cookie)
	if wm.Code != http.StatusOK {
		t.Fatalf("mark read: status = %d, body = %s", wm.Code, wm.Body.String())
	}
	uc2 := do2FA(r, http.MethodGet, "/api/v1/notifications/unread-count", "", cookie)
	var uc2Body struct {
		Count int64 `json:"count"`
	}
	if err := json.Unmarshal(uc2.Body.Bytes(), &uc2Body); err != nil || uc2Body.Count != 0 {
		t.Errorf("unread after mark one = %s (err %v), want 0", uc2.Body.String(), err)
	}

	// 全部已读：此时已无未读（上一步已读），更新数应为 0
	wa := do2FA(r, http.MethodPut, "/api/v1/notifications/read-all", "", cookie)
	if wa.Code != http.StatusOK {
		t.Fatalf("mark all read: status = %d", wa.Code)
	}
	var all struct {
		Updated int64 `json:"updated"`
	}
	if err := json.Unmarshal(wa.Body.Bytes(), &all); err != nil || all.Updated != 0 {
		t.Fatalf("mark all read updated = %+v (err %v), want 0 (already all read)", all, err)
	}

	// 再补 1 条未读后 read-all 应更新 1 条
	if err := st.CreateNotification(&store.Notification{
		UserID: user.ID, Type: "comment_reply", Title: "回复", Content: "c", Link: "/p",
		Read: 0, CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create notification: %v", err)
	}
	wa2 := do2FA(r, http.MethodPut, "/api/v1/notifications/read-all", "", cookie)
	var all2 struct {
		Updated int64 `json:"updated"`
	}
	if err := json.Unmarshal(wa2.Body.Bytes(), &all2); err != nil || all2.Updated != 1 {
		t.Fatalf("mark all read (with 1 unread) updated = %+v (err %v), want 1", all2, err)
	}

	// 标记不存在的通知 → 404
	wn := do2FA(r, http.MethodPut, "/api/v1/notifications/99999/read", "", cookie)
	if wn.Code != http.StatusNotFound {
		t.Errorf("mark nonexistent read: status = %d, want 404", wn.Code)
	}
}
