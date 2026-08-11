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
	spamID := seedComment(t, st, &store.Comment{
		PageKey: "/post/hello", Nick: "广告号", Email: "spam@x.com",
		ContentMD: "垃圾评论", Status: store.CommentSpam,
	})
	rejectedID := seedComment(t, st, &store.Comment{
		PageKey: "/post/hello", Nick: "访客3", Email: "g3@x.com",
		ContentMD: "被拒评论", Status: store.CommentRejected,
	})

	// 待审筛选
	w := do2FA(r, http.MethodGet, "/api/v1/comments?status=pending", "", cookie)
	var list struct {
		Items  []commentJSON  `json:"items"`
		Total  int            `json:"total"`
		Counts map[string]int `json:"counts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || list.Total != 1 {
		t.Fatalf("pending list: %+v (err %v), want 1", list, err)
	}
	if list.Items[0].PageKey != "/post/hello" || list.Items[0].Nick != "访客" {
		t.Errorf("pending item mismatch: %+v", list.Items[0])
	}
	// counts：各状态真实计数
	wantCounts := map[string]int{"all": 4, "pending": 1, "approved": 1, "spam": 1, "rejected": 1}
	for k, want := range wantCounts {
		if list.Counts[k] != want {
			t.Errorf("counts[%q] = %d, want %d (counts=%+v)", k, list.Counts[k], want, list.Counts)
		}
	}
	// 全部
	w = do2FA(r, http.MethodGet, "/api/v1/comments", "", cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 4 {
		t.Errorf("all comments total = %d, want 4", list.Total)
	}
	// spam / rejected 筛选真实生效
	w = do2FA(r, http.MethodGet, "/api/v1/comments?status=spam", "", cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 || list.Items[0].Nick != "广告号" {
		t.Errorf("spam filter: total=%d items=%+v, want 1 spam item", list.Total, list.Items)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/comments?status=rejected", "", cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 || list.Items[0].ID != rejectedID {
		t.Errorf("rejected filter: total=%d items=%+v, want 1 rejected item", list.Total, list.Items)
	}
	// 非法 status 回退全量
	w = do2FA(r, http.MethodGet, "/api/v1/comments?status=bogus", "", cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 4 {
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

	// 通用状态端点：拒绝 / 恢复 / 标垃圾
	w = do2FA(r, http.MethodPut, "/api/v1/comments/"+itoa(spamID)+"/status", `{"status":"rejected"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("set status rejected: status = %d, body = %s", w.Code, w.Body.String())
	}
	if got, _ := st.GetComment(spamID); got == nil || got.Status != store.CommentRejected {
		t.Errorf("comment status after reject = %+v, want rejected", got)
	}
	w = do2FA(r, http.MethodPut, "/api/v1/comments/"+itoa(spamID)+"/status", `{"status":"spam"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("set status spam: status = %d, body = %s", w.Code, w.Body.String())
	}
	if got, _ := st.GetComment(spamID); got == nil || got.Status != store.CommentSpam {
		t.Errorf("comment status after spam = %+v, want spam", got)
	}
	// 非法 status → 400；不存在 → 404；非法 ID → 400
	w = do2FA(r, http.MethodPut, "/api/v1/comments/"+itoa(spamID)+"/status", `{"status":"bogus"}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("set status bogus: status = %d, want 400", w.Code)
	}
	w = do2FA(r, http.MethodPut, "/api/v1/comments/99999/status", `{"status":"rejected"}`, cookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("set status missing: status = %d, want 404", w.Code)
	}
	w = do2FA(r, http.MethodPut, "/api/v1/comments/abc/status", `{"status":"rejected"}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("set status bad id: status = %d, want 400", w.Code)
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
	if list.Total != 3 {
		t.Errorf("after cascade delete total = %d, want 3 (reply removed too)", list.Total)
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
	// 批量 reject / spam
	idC := seedComment(t, st, &store.Comment{PageKey: "/p", Nick: "c", ContentMD: "c"})
	idD := seedComment(t, st, &store.Comment{PageKey: "/p", Nick: "d", ContentMD: "d"})
	w = do2FA(r, http.MethodPost, "/api/v1/comments/batch",
		`{"ids":[`+itoa(idC)+`,`+itoa(idD)+`],"action":"reject"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("batch reject: status = %d", w.Code)
	}
	_ = json.Unmarshal(w.Body.Bytes(), &batch)
	if batch.Succeeded != 2 || batch.Failed != 0 {
		t.Errorf("batch reject = %+v, want 2/0", batch)
	}
	if got, _ := st.GetComment(idC); got == nil || got.Status != store.CommentRejected {
		t.Errorf("comment %d after batch reject = %+v, want rejected", idC, got)
	}
	w = do2FA(r, http.MethodPost, "/api/v1/comments/batch",
		`{"ids":[`+itoa(idC)+`,`+itoa(idD)+`],"action":"spam"}`, cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &batch)
	if batch.Succeeded != 2 || batch.Failed != 0 {
		t.Errorf("batch spam = %+v, want 2/0", batch)
	}
	if got, _ := st.GetComment(idD); got == nil || got.Status != store.CommentSpam {
		t.Errorf("comment %d after batch spam = %+v, want spam", idD, got)
	}
}
