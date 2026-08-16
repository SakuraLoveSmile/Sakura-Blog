package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// Spec 71 评论用户管理 API 测试：列表聚合/封禁解封/用户级批量处置/删除 +
// 封禁生效链路（登录拒绝、既有会话按匿名处理）。

// seedReaderUser 直接落库一个 reader 用户（绕开注册限流，便于测试多用户）。
func seedReaderUser(t *testing.T, st *store.Store, username string) *store.User {
	t.Helper()
	hash, err := store.HashPassword("reader66")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u := &store.User{
		Username: username, PasswordHash: hash,
		Email:      username + "@example.com",
		AuthSecret: store.GenerateAuthSecret(),
		Role:       store.RoleReader,
	}
	if err := st.CreateUser(u); err != nil {
		t.Fatalf("create reader %s: %v", username, err)
	}
	return u
}

// doUserReq 带读者 cookie（blog_user）的请求。
func doUserReq(r *gin.Engine, method, path, body, userCookie string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if userCookie != "" {
		req.AddCookie(&http.Cookie{Name: userCookieName, Value: userCookie})
	}
	r.ServeHTTP(w, req)
	return w
}

// signReaderCookie 为 reader 签合法读者 cookie。
func signReaderCookie(u *store.User) string {
	return signUserCookie(time.Now().Add(userCookieTTL).Unix(), u.ID, u.AuthSecret)
}

// TestCommentUsersListAndCounts 评论用户列表：仅 reader 出现，评论聚合正确，
// counts 全量计数，status/keyword 过滤生效。
func TestCommentUsersListAndCounts(t *testing.T) {
	r, st, admin, _ := newFullEnv(t)
	adminCookie := authCookie(admin)

	active := seedReaderUser(t, st, "active-reader")
	seedReaderUser(t, st, "silent-reader")
	seedComment(t, st, &store.Comment{
		PageKey: "/post/hello", Nick: "active-reader", Email: "active-reader@example.com",
		ContentMD: "第一条", Status: store.CommentApproved, UserID: active.ID,
	})
	seedComment(t, st, &store.Comment{
		PageKey: "/post/hello", Nick: "active-reader", Email: "active-reader@example.com",
		ContentMD: "第二条", Status: store.CommentPending, UserID: active.ID,
	})

	w := do2FA(r, http.MethodGet, "/api/v1/comment-users", "", adminCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("list comment users: status = %d, body = %s", w.Code, w.Body.String())
	}
	var list struct {
		Items []struct {
			ID           int64  `json:"id"`
			Username     string `json:"username"`
			Status       string `json:"status"`
			CommentCount int    `json:"comment_count"`
		} `json:"items"`
		Total  int `json:"total"`
		Counts struct {
			All    int `json:"all"`
			Banned int `json:"banned"`
		} `json:"counts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if list.Total != 2 || list.Counts.All != 2 || list.Counts.Banned != 0 {
		t.Fatalf("total/counts = %d/%+v, want 2 / {all:2 banned:0}", list.Total, list.Counts)
	}
	if list.Items[0].Username != "active-reader" || list.Items[0].CommentCount != 2 {
		t.Errorf("first item = %+v, want active-reader with 2 comments", list.Items[0])
	}

	// keyword 过滤
	w = do2FA(r, http.MethodGet, "/api/v1/comment-users?keyword=silent", "", adminCookie)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 || list.Items[0].Username != "silent-reader" {
		t.Errorf("keyword filter: %+v", list)
	}

	// 未认证拒绝
	w = do2FA(r, http.MethodGet, "/api/v1/comment-users", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated: status = %d, want 401", w.Code)
	}
}

// TestCommentUserBanFlow 封禁/解封全流程：状态持久化 + 登录拒绝 + 既有会话即时匿名化。
func TestCommentUserBanFlow(t *testing.T) {
	r, st, admin, _ := newFullEnv(t)
	adminCookie := authCookie(admin)
	reader := seedReaderUser(t, st, "reader-ban")

	ban := func(status string) *httptest.ResponseRecorder {
		return do2FA(r, http.MethodPut, fmt.Sprintf("/api/v1/comment-users/%d/status", reader.ID),
			fmt.Sprintf(`{"status":%q}`, status), adminCookie)
	}

	// 封禁
	if w := ban(store.UserStatusBanned); w.Code != http.StatusOK {
		t.Fatalf("ban: status = %d, body = %s", w.Code, w.Body.String())
	}
	got, _ := st.GetUserByID(reader.ID)
	if got == nil || got.Status != store.UserStatusBanned {
		t.Fatalf("status after ban = %+v", got)
	}

	// 封禁后登录被拒
	w := do2FA(r, http.MethodPost, "/api/auth/login",
		`{"username":"reader-ban","password":"reader66"}`, "")
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "封禁") {
		t.Errorf("banned login: status = %d body = %s, want 403 + 封禁提示", w.Code, w.Body.String())
	}

	// 既有会话（合法 cookie）按匿名处理
	cookie := signReaderCookie(reader)
	w = doUserReq(r, http.MethodGet, "/api/auth/me", "", cookie)
	var me struct {
		ID       int64 `json:"id"`
		Verified bool  `json:"verified"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if me.ID != 0 || me.Verified {
		t.Errorf("banned user session should be anonymous, got %+v", me)
	}

	// 解封后登录恢复
	if w := ban(store.UserStatusActive); w.Code != http.StatusOK {
		t.Fatalf("unban: status = %d", w.Code)
	}
	w = do2FA(r, http.MethodPost, "/api/auth/login",
		`{"username":"reader-ban","password":"reader66"}`, "")
	if w.Code != http.StatusOK {
		t.Errorf("login after unban: status = %d, want 200", w.Code)
	}
	w = doUserReq(r, http.MethodGet, "/api/auth/me", "", cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &me)
	if me.ID != reader.ID || !me.Verified {
		t.Errorf("session after unban should be verified, got %+v", me)
	}

	// 非法 status / 非 reader 保护
	if w := ban("weird"); w.Code != http.StatusBadRequest {
		t.Errorf("invalid status: %d, want 400", w.Code)
	}
	w = do2FA(r, http.MethodPut, fmt.Sprintf("/api/v1/comment-users/%d/status", admin.ID),
		`{"status":"banned"}`, adminCookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("ban non-reader: %d, want 400", w.Code)
	}
}

// TestCommentUserBulkAndDelete user_id 评论过滤 + 用户级批量处置 + 删除用户。
func TestCommentUserBulkAndDelete(t *testing.T) {
	r, st, admin, _ := newFullEnv(t)
	adminCookie := authCookie(admin)
	spammer := seedReaderUser(t, st, "spammer")
	other := seedReaderUser(t, st, "other-reader")

	for i, md := range []string{"垃圾一", "垃圾二", "垃圾三"} {
		_ = i
		seedComment(t, st, &store.Comment{
			PageKey: "/post/hello", Nick: "spammer", Email: "spammer@example.com",
			ContentMD: md, Status: store.CommentApproved, UserID: spammer.ID,
		})
	}
	otherID := seedComment(t, st, &store.Comment{
		PageKey: "/post/hello", Nick: "other", Email: "other-reader@example.com",
		ContentMD: "正常评论", Status: store.CommentApproved, UserID: other.ID,
	})

	// 按用户过滤评论列表
	w := do2FA(r, http.MethodGet, fmt.Sprintf("/api/v1/comments?user_id=%d", spammer.ID), "", adminCookie)
	var clist struct {
		Total int `json:"total"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &clist)
	if w.Code != http.StatusOK || clist.Total != 3 {
		t.Fatalf("comments by user: status = %d total = %d, want 200/3", w.Code, clist.Total)
	}

	bulk := func(action string) *httptest.ResponseRecorder {
		return do2FA(r, http.MethodPost, fmt.Sprintf("/api/v1/comment-users/%d/comments/bulk", spammer.ID),
			fmt.Sprintf(`{"action":%q}`, action), adminCookie)
	}
	var bulkResp struct {
		OK       bool `json:"ok"`
		Affected int  `json:"affected"`
	}

	// 全部标为垃圾
	w = bulk("spam")
	_ = json.Unmarshal(w.Body.Bytes(), &bulkResp)
	if w.Code != http.StatusOK || bulkResp.Affected != 3 {
		t.Fatalf("bulk spam: status = %d resp = %+v, want 3 affected", w.Code, bulkResp)
	}
	if cm, _ := st.GetComment(otherID); cm == nil || cm.Status != store.CommentApproved {
		t.Errorf("other's comment should stay approved, got %+v", cm)
	}

	// 全部恢复通过
	w = bulk("approve")
	_ = json.Unmarshal(w.Body.Bytes(), &bulkResp)
	if w.Code != http.StatusOK || bulkResp.Affected != 3 {
		t.Fatalf("bulk approve: status = %d resp = %+v", w.Code, bulkResp)
	}

	// 全部删除
	w = bulk("delete")
	_ = json.Unmarshal(w.Body.Bytes(), &bulkResp)
	if w.Code != http.StatusOK || bulkResp.Affected != 3 {
		t.Fatalf("bulk delete: status = %d resp = %+v", w.Code, bulkResp)
	}
	w = do2FA(r, http.MethodGet, fmt.Sprintf("/api/v1/comments?user_id=%d", spammer.ID), "", adminCookie)
	_ = json.Unmarshal(w.Body.Bytes(), &clist)
	if clist.Total != 0 {
		t.Errorf("comments after bulk delete = %d, want 0", clist.Total)
	}
	if cm, _ := st.GetComment(otherID); cm == nil {
		t.Errorf("other's comment should survive")
	}

	// 非法 action / 非 reader
	if w := bulk("reject"); w.Code != http.StatusBadRequest {
		t.Errorf("invalid action: %d, want 400", w.Code)
	}
	w = do2FA(r, http.MethodPost, fmt.Sprintf("/api/v1/comment-users/%d/comments/bulk", admin.ID),
		`{"action":"spam"}`, adminCookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("bulk on non-reader: %d, want 400", w.Code)
	}

	// 删除评论用户
	w = do2FA(r, http.MethodDelete, fmt.Sprintf("/api/v1/comment-users/%d", spammer.ID), "", adminCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("delete comment user: status = %d, body = %s", w.Code, w.Body.String())
	}
	if u, _ := st.GetUserByID(spammer.ID); u != nil {
		t.Errorf("user should be deleted")
	}
	w = do2FA(r, http.MethodGet, "/api/v1/comment-users", "", adminCookie)
	var ulist struct {
		Total int `json:"total"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &ulist)
	if ulist.Total != 1 {
		t.Errorf("readers after delete = %d, want 1", ulist.Total)
	}

	// 非 reader 不允许经该端点删除
	w = do2FA(r, http.MethodDelete, fmt.Sprintf("/api/v1/comment-users/%d", admin.ID), "", adminCookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("delete non-reader: %d, want 400", w.Code)
	}
}
