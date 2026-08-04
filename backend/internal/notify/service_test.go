package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"blog/internal/store"
)

// newTestEnv 内存测试环境：store + users 表 + settings 表 + notifications 表 + 2 个用户。
func newTestEnv(t *testing.T) (*store.Store, []*store.User) {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.EnsureUsersTable(); err != nil {
		t.Fatalf("ensure users: %v", err)
	}
	if err := st.EnsureSettingsTable(); err != nil {
		t.Fatalf("ensure settings: %v", err)
	}
	if err := st.EnsureNotificationsTable(); err != nil {
		t.Fatalf("ensure notifications: %v", err)
	}
	var users []*store.User
	for _, name := range []string{"admin", "editor"} {
		u := &store.User{Username: name}
		if err := st.CreateUser(u); err != nil {
			t.Fatalf("create user %s: %v", name, err)
		}
		users = append(users, u)
	}
	return st, users
}

// TestNotifyWritesForAllUsers 站内通知落库：全部用户各收到一条，字段完整。
func TestNotifyWritesForAllUsers(t *testing.T) {
	st, users := newTestEnv(t)
	svc := New(st)

	svc.notify(Event{
		Type:    "comment_new",
		Title:   "新评论：小明",
		Content: "写得好！",
		Link:    "/posts/hello",
		Nick:    "小明",
		IP:      "1.2.3.4",
		PageKey: "/posts/hello",
		Status:  "pending",
	})

	for _, u := range users {
		items, total, err := svc.List(u.ID, 1, 20)
		if err != nil {
			t.Fatalf("list notifications for %s: %v", u.Username, err)
		}
		if total != 1 || len(items) != 1 {
			t.Fatalf("user %s: total = %d, want 1", u.Username, total)
		}
		n := items[0]
		if n.Type != "comment_new" || n.Title != "新评论：小明" || n.Content != "写得好！" ||
			n.Link != "/posts/hello" || n.Read != 0 || n.UserID != u.ID {
			t.Errorf("user %s: notification fields mismatch: %+v", u.Username, n)
		}
	}
}

// TestMarkReadFlow 已读切换：未读数递减、单条已读、全部已读、分页未读优先。
func TestMarkReadFlow(t *testing.T) {
	st, users := newTestEnv(t)
	svc := New(st)
	uid := users[0].ID

	for i := 0; i < 3; i++ {
		svc.notify(Event{Type: "comment_new", Title: "t", Content: "c", Link: "/p"})
	}
	// 未读计数
	if n, _ := svc.UnreadCount(uid); n != 3 {
		t.Fatalf("unread = %d, want 3", n)
	}
	// 未读优先：列表第一条应为未读
	items, _, _ := svc.List(uid, 1, 20)
	if len(items) != 3 {
		t.Fatalf("list len = %d, want 3", len(items))
	}
	for _, it := range items {
		if it.Read != 0 {
			t.Errorf("list should be unread-first, got read=%d", it.Read)
		}
	}
	// 单条已读
	ok, err := svc.MarkRead(uid, items[0].ID)
	if err != nil || !ok {
		t.Fatalf("mark read: ok=%v err=%v", ok, err)
	}
	if n, _ := svc.UnreadCount(uid); n != 2 {
		t.Fatalf("unread after mark one = %d, want 2", n)
	}
	// 跨用户隔离：另一用户未读数不受影响
	if n, _ := svc.UnreadCount(users[1].ID); n != 3 {
		t.Fatalf("other user unread = %d, want 3", n)
	}
	// 全部已读
	upd, err := svc.MarkAllRead(uid)
	if err != nil || upd != 2 {
		t.Fatalf("mark all: updated=%d err=%v, want 2", upd, err)
	}
	if n, _ := svc.UnreadCount(uid); n != 0 {
		t.Fatalf("unread after mark all = %d, want 0", n)
	}
}

// TestPostWebhookPayload Webhook：配置 webhook_url 后事件 POST JSON 字段完整。
func TestPostWebhookPayload(t *testing.T) {
	st, _ := newTestEnv(t)
	svc := New(st)

	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("webhook method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("webhook content-type = %s", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode webhook payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := st.PutSetting("notify", `{"webhook_url":"`+srv.URL+`"}`); err != nil {
		t.Fatalf("put webhook config: %v", err)
	}
	svc.notify(Event{
		Type:       "comment_reply",
		Title:      "小红 回复了评论",
		Content:    "同意",
		Link:       "/posts/hello",
		Nick:       "小红",
		Email:      "red@example.com",
		IP:         "9.9.9.9",
		PageKey:    "/posts/hello",
		Status:     "approved",
		ParentNick: "小明",
	})
	if got == nil {
		t.Fatal("webhook should have received a payload")
	}
	want := map[string]string{
		"type": "comment_reply", "title": "小红 回复了评论", "content": "同意",
		"link": "/posts/hello", "nick": "小红", "email": "red@example.com",
		"ip": "9.9.9.9", "page_key": "/posts/hello", "status": "approved",
		"parent_nick": "小明",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("webhook payload[%q] = %v, want %q", k, got[k], v)
		}
	}
}

// TestWebhookSkippedWithoutConfig 未配置 webhook_url 时不应发起请求。
func TestWebhookSkippedWithoutConfig(t *testing.T) {
	st, _ := newTestEnv(t)
	svc := New(st)
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	svc.notify(Event{Type: "comment_new", Title: "t", Content: "c", Link: "/p"})
	if called {
		t.Error("webhook should be skipped when webhook_url is empty")
	}
}
