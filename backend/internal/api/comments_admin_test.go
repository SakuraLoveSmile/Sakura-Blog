package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"blog/internal/store"
)

// Wave 4 测试覆盖补强：评论管理（列表筛选/审核/删除/批量/管理员回复）。

// seedComment 直接落库一条评论，返回 id。
func seedComment(t *testing.T, st *store.Store, cm *store.Comment) int64 {
	t.Helper()
	if cm.Status == "" {
		cm.Status = store.CommentPending
	}
	if cm.ContentHTML == "" {
		cm.ContentHTML = "<p>" + cm.ContentMD + "</p>"
	}
	id, err := st.CreateComment(cm)
	if err != nil {
		t.Fatalf("seed comment: %v", err)
	}
	return id
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// TestCommentAdminFlow 评论管理全链路：待审列表 → 审核通过 → 已审列表 →
// 管理员回复（嵌套挂根）→ 删除（级联）→ 批量操作。
func TestCommentAdminFlow(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	cookie := authCookie(user)

	pendingID := seedComment(t, st, &store.Comment{
		PageKey: "/post/hello", Nick: "访客", Email: "g@x.com",
		ContentMD: "第一条评论", Status: store.CommentPending,
	})
	seedComment(t, st, &store.Comment{
		PageKey: "/post/hello", Nick: "访客2", Email: "g2@x.com",
		ContentMD: "已通过评论", Status: store.CommentApproved,
	})

	// 待审筛选
	w := do2FA(r, http.MethodGet, "/api/v1/comments?status=pending", "", cookie)
	var list struct {
		Items []commentJSON `json:"items"`
		Total int           `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || list.Total != 1 {
		t.Fatalf("pending list: %+v (err %v), want 1", list, err)
	}
	if list.Items[0].PageKey != "/post/hello" || list.Items[0].Nick != "访客" {
		t.Errorf("pending item mismatch: %+v", list.Items[0])
	}
	// 全部
	w = do2FA(r, http.MethodGet, "/api/v1/comments", "", cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 2 {
		t.Errorf("all comments total = %d, want 2", list.Total)
	}
	// 非法 status 回退全量
	w = do2FA(r, http.MethodGet, "/api/v1/comments?status=spam", "", cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 2 {
		t.Errorf("invalid status filter should fallback to all, total = %d", list.Total)
	}

	// 审核通过
	w = do2FA(r, http.MethodPut, "/api/v1/comments/"+itoa(pendingID)+"/approve", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("approve: status = %d, body = %s", w.Code, w.Body.String())
	}
	got, err := st.GetComment(pendingID)
	if err != nil || got.Status != store.CommentApproved {
		t.Errorf("comment status after approve = %q (err %v), want approved", got.Status, err)
	}

	// 审核不存在 → 404；非法 ID → 400
	w = do2FA(r, http.MethodPut, "/api/v1/comments/99999/approve", "", cookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("approve missing: status = %d, want 404", w.Code)
	}
	w = do2FA(r, http.MethodPut, "/api/v1/comments/abc/approve", "", cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("approve bad id: status = %d, want 400", w.Code)
	}

	// 管理员回复（待审评论回复 → 挂到同一根）
	w = do2FA(r, http.MethodPost, "/api/v1/comments/reply",
		`{"parent_id":`+itoa(pendingID)+`,"content":"博主回复"}`, cookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("reply: status = %d, body = %s", w.Code, w.Body.String())
	}
	var reply commentJSON
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
		t.Fatalf("decode reply: %v", err)
	}
	if reply.ParentID != pendingID || reply.RootID != pendingID || !reply.IsAdmin {
		t.Errorf("reply mismatch: %+v", reply)
	}
	// 回复缺字段 → 400；父不存在 → 400
	w = do2FA(r, http.MethodPost, "/api/v1/comments/reply", `{"parent_id":0,"content":""}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("reply empty: status = %d, want 400", w.Code)
	}
	w = do2FA(r, http.MethodPost, "/api/v1/comments/reply", `{"parent_id":88888,"content":"x"}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("reply missing parent: status = %d, want 400", w.Code)
	}

	// 删除：级联（删根评论后回复一并消失）
	w = do2FA(r, http.MethodDelete, "/api/v1/comments/"+itoa(pendingID), "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: status = %d", w.Code)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/comments", "", cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 {
		t.Errorf("after cascade delete total = %d, want 1 (reply removed too)", list.Total)
	}

	// 批量：approve 与 delete（重建两条待审再批量）
	idA := seedComment(t, st, &store.Comment{PageKey: "/p", Nick: "a", ContentMD: "a"})
	idB := seedComment(t, st, &store.Comment{PageKey: "/p", Nick: "b", ContentMD: "b"})
	w = do2FA(r, http.MethodPost, "/api/v1/comments/batch",
		`{"ids":[`+itoa(idA)+`,`+itoa(idB)+`],"action":"approve"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("batch approve: status = %d", w.Code)
	}
	var batch struct {
		Succeeded int `json:"succeeded"`
		Failed    int `json:"failed"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &batch)
	if batch.Succeeded != 2 || batch.Failed != 0 {
		t.Errorf("batch approve = %+v, want 2/0", batch)
	}
	// 批量：非法 action / 空 ids / 超上限
	w = do2FA(r, http.MethodPost, "/api/v1/comments/batch", `{"ids":[1],"action":"x"}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("batch bad action: status = %d, want 400", w.Code)
	}
	w = do2FA(r, http.MethodPost, "/api/v1/comments/batch", `{"ids":[],"action":"approve"}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("batch empty ids: status = %d, want 400", w.Code)
	}
	// 批量 delete：混入不存在 id，其余成功
	w = do2FA(r, http.MethodPost, "/api/v1/comments/batch",
		`{"ids":[`+itoa(idA)+`,777777],"action":"delete"}`, cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &batch)
	if batch.Succeeded != 1 || batch.Failed != 1 {
		t.Errorf("batch delete mixed = %+v, want 1/1", batch)
	}
}
