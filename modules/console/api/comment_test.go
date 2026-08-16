package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
	tmpl "blog/internal/template"
)

// newBatchTestEnv 组装无密码（免认证）测试环境，返回 engine 与 store。
func newBatchTestEnv(t *testing.T) (*gin.Engine, *store.Store) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	r := gin.New()
	if _, err := Mount(r, MountDeps{Config: Config{Store: st, Site: sitePtr(&tmpl.Site{})}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	return r, st
}

// mkTestComment 插入一条评论，返回 id。
func mkTestComment(t *testing.T, st *store.Store, status string, rootID int64) int64 {
	t.Helper()
	id, err := st.CreateComment(&store.Comment{
		PageKey:   "/post/x",
		Nick:      "n",
		Email:     "e@example.com",
		ContentMD: "hi",
		Status:    status,
		RootID:    rootID,
	})
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}
	return id
}

// postBatch 发起批量请求，返回 recorder。
func postBatch(r *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/comments/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestBatchCommentsUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "secret")
	r := gin.New()
	if _, err := Mount(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site)}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	w := postBatch(r, `{"ids":[1],"action":"approve"}`)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated batch should get 401, got %d", w.Code)
	}
}

func TestBatchCommentsInvalidAction(t *testing.T) {
	r, _ := newBatchTestEnv(t)
	w := postBatch(r, `{"ids":[1],"action":"bogus"}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid action should get 400, got %d", w.Code)
	}
}

func TestBatchCommentsEmptyIDs(t *testing.T) {
	r, _ := newBatchTestEnv(t)
	w := postBatch(r, `{"ids":[],"action":"approve"}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("empty ids should get 400, got %d", w.Code)
	}
}

func TestBatchCommentsOverLimit(t *testing.T) {
	r, _ := newBatchTestEnv(t)
	ids := make([]int64, 101)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	body, _ := json.Marshal(map[string]interface{}{"ids": ids, "action": "approve"})
	w := postBatch(r, string(body))
	if w.Code != http.StatusBadRequest {
		t.Errorf("over 100 ids should get 400, got %d", w.Code)
	}
}

func TestBatchCommentsApprove(t *testing.T) {
	r, st := newBatchTestEnv(t)
	a := mkTestComment(t, st, store.CommentPending, 0)
	b := mkTestComment(t, st, store.CommentPending, 0)

	// 混入不存在 id：其余照常成功，failed 计数正确
	w := postBatch(r, fmt.Sprintf(`{"ids":[%d,%d,99999],"action":"approve"}`, a, b))
	if w.Code != http.StatusOK {
		t.Fatalf("batch approve: status %d, body %s", w.Code, w.Body.String())
	}
	var resp struct {
		Succeeded int `json:"succeeded"`
		Failed    int `json:"failed"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal resp: %v", err)
	}
	if resp.Succeeded != 2 || resp.Failed != 1 {
		t.Errorf("succeeded=%d failed=%d, want 2/1", resp.Succeeded, resp.Failed)
	}
	for _, id := range []int64{a, b} {
		cm, _ := st.GetComment(id)
		if cm == nil || cm.Status != store.CommentApproved {
			t.Errorf("comment %d not approved: %+v", id, cm)
		}
	}
}

func TestBatchCommentsDelete(t *testing.T) {
	r, st := newBatchTestEnv(t)
	root := mkTestComment(t, st, store.CommentApproved, 0)
	child := mkTestComment(t, st, store.CommentApproved, root)
	other := mkTestComment(t, st, store.CommentPending, 0)

	w := postBatch(r, fmt.Sprintf(`{"ids":[%d,77777],"action":"delete"}`, root))
	if w.Code != http.StatusOK {
		t.Fatalf("batch delete: status %d, body %s", w.Code, w.Body.String())
	}
	var resp struct {
		Succeeded int `json:"succeeded"`
		Failed    int `json:"failed"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal resp: %v", err)
	}
	if resp.Succeeded != 1 || resp.Failed != 1 {
		t.Errorf("succeeded=%d failed=%d, want 1/1", resp.Succeeded, resp.Failed)
	}
	// 级联删除子评论，未选中的不受影响
	for _, id := range []int64{root, child} {
		if cm, _ := st.GetComment(id); cm != nil {
			t.Errorf("comment %d should be deleted", id)
		}
	}
	if cm, _ := st.GetComment(other); cm == nil {
		t.Error("unselected comment should survive")
	}
}
