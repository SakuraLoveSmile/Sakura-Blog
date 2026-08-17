package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// doJSON 发起带 JSON body 的请求，返回 recorder。
func doJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// mkAPIAttach 直接经 store 插入一条附件，返回 id。
func mkAPIAttach(t *testing.T, st *store.Store, filename string, groupID int64) int64 {
	t.Helper()
	id, err := st.CreateAttachment(&store.Attachment{
		Filename: filename,
		Path:     "2026/07/" + filename,
		URL:      "/uploads/2026/07/" + filename,
		Mime:     "image/png",
		Size:     100,
		GroupID:  groupID,
	})
	if err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	return id
}

// TestAttachmentGroupsUnauthorized 带密码站点下未登录访问分组端点应 401。
func TestAttachmentGroupsUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "secret")
	r := gin.New()
	if _, err := Mount(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site)}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	cases := []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/v1/attachment-groups", ""},
		{http.MethodPost, "/api/v1/attachment-groups", `{"name":"x"}`},
		{http.MethodPut, "/api/v1/attachment-groups/1", `{"name":"x"}`},
		{http.MethodDelete, "/api/v1/attachment-groups/1", ""},
		{http.MethodPost, "/api/v1/attachments/move", `{"ids":[1],"group_id":0}`},
	}
	for _, c := range cases {
		w := doJSON(r, c.method, c.path, c.body)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: got %d, want 401", c.method, c.path, w.Code)
		}
	}
}

// TestAttachmentGroupCreateAndList 建组 → 列表可见。
func TestAttachmentGroupCreateAndList(t *testing.T) {
	r, _ := newBatchTestEnv(t)

	// 空名报 400
	if w := doJSON(r, http.MethodPost, "/api/v1/attachment-groups", `{"name":"  "}`); w.Code != http.StatusBadRequest {
		t.Errorf("empty name: got %d, want 400", w.Code)
	}

	w := doJSON(r, http.MethodPost, "/api/v1/attachment-groups", `{"name":"封面","order":3}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create group: %d %s", w.Code, w.Body.String())
	}
	var created struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Order int    `json:"order"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if created.ID == 0 || created.Name != "封面" || created.Order != 3 {
		t.Fatalf("unexpected created: %+v", created)
	}

	w = doJSON(r, http.MethodGet, "/api/v1/attachment-groups", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list groups: %d", w.Code)
	}
	var listResp struct {
		Items []struct {
			ID    int64  `json:"id"`
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if len(listResp.Items) != 1 || listResp.Items[0].Name != "封面" || listResp.Items[0].Count != 0 {
		t.Fatalf("unexpected list: %+v", listResp.Items)
	}
}

// TestAttachmentGroupUpdateDelete 改名 + 删组（含不存在报 404）。
func TestAttachmentGroupUpdateDelete(t *testing.T) {
	r, st := newBatchTestEnv(t)
	gid, _ := st.CreateAttachmentGroup(&store.AttachmentGroup{Name: "组A"})
	att := mkAPIAttach(t, st, "a.png", gid)

	// 改名
	if w := doJSON(r, http.MethodPut, fmt.Sprintf("/api/v1/attachment-groups/%d", gid), `{"name":"组B","order":9}`); w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body.String())
	}
	// 改不存在的组 → 404
	if w := doJSON(r, http.MethodPut, "/api/v1/attachment-groups/99999", `{"name":"x"}`); w.Code != http.StatusNotFound {
		t.Errorf("update nonexistent: got %d, want 404", w.Code)
	}

	// 删组 → 组内附件归未分组
	if w := doJSON(r, http.MethodDelete, fmt.Sprintf("/api/v1/attachment-groups/%d", gid), ""); w.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	if a, _ := st.GetAttachment(att); a == nil || a.GroupID != store.UngroupedGroupID {
		t.Errorf("attachment should be ungrouped after delete: %+v", a)
	}
	// 删不存在的组 → 404
	if w := doJSON(r, http.MethodDelete, "/api/v1/attachment-groups/99999", ""); w.Code != http.StatusNotFound {
		t.Errorf("delete nonexistent: got %d, want 404", w.Code)
	}
}

// TestMoveAttachmentsEndpoint 批量移动端点：成功 + 边界。
func TestMoveAttachmentsEndpoint(t *testing.T) {
	r, st := newBatchTestEnv(t)
	gid, _ := st.CreateAttachmentGroup(&store.AttachmentGroup{Name: "目标"})
	a := mkAPIAttach(t, st, "a.png", store.UngroupedGroupID)
	b := mkAPIAttach(t, st, "b.png", store.UngroupedGroupID)

	// 空 ids → 400
	if w := doJSON(r, http.MethodPost, "/api/v1/attachments/move", `{"ids":[],"group_id":0}`); w.Code != http.StatusBadRequest {
		t.Errorf("empty ids: got %d, want 400", w.Code)
	}
	// 目标组不存在 → 400
	if w := doJSON(r, http.MethodPost, "/api/v1/attachments/move", fmt.Sprintf(`{"ids":[%d],"group_id":99999}`, a)); w.Code != http.StatusBadRequest {
		t.Errorf("bad group: got %d, want 400", w.Code)
	}

	// 正常移动到组
	w := doJSON(r, http.MethodPost, "/api/v1/attachments/move", fmt.Sprintf(`{"ids":[%d,%d],"group_id":%d}`, a, b, gid))
	if w.Code != http.StatusOK {
		t.Fatalf("move: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Moved int `json:"moved"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Moved != 2 {
		t.Errorf("moved=%d, want 2", resp.Moved)
	}
	for _, id := range []int64{a, b} {
		if att, _ := st.GetAttachment(id); att == nil || att.GroupID != gid {
			t.Errorf("attachment %d not moved: %+v", id, att)
		}
	}

	// 移出到未分组（group_id=0）合法
	if w := doJSON(r, http.MethodPost, "/api/v1/attachments/move", fmt.Sprintf(`{"ids":[%d],"group_id":0}`, a)); w.Code != http.StatusOK {
		t.Errorf("move to ungrouped: got %d, want 200", w.Code)
	}
}

// TestListAttachmentsGroupParam 列表端点 group 查询参数生效。
func TestListAttachmentsGroupParam(t *testing.T) {
	r, st := newBatchTestEnv(t)
	gid, _ := st.CreateAttachmentGroup(&store.AttachmentGroup{Name: "G"})
	mkAPIAttach(t, st, "in.png", gid)
	mkAPIAttach(t, st, "out.png", store.UngroupedGroupID)

	total := func(path string) int {
		w := doJSON(r, http.MethodGet, path, "")
		if w.Code != http.StatusOK {
			t.Fatalf("list %s: %d", path, w.Code)
		}
		var resp struct {
			Total int `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return resp.Total
	}

	if got := total("/api/v1/attachments"); got != 2 {
		t.Errorf("no filter: got %d, want 2", got)
	}
	if got := total(fmt.Sprintf("/api/v1/attachments?group=%d", gid)); got != 1 {
		t.Errorf("group filter: got %d, want 1", got)
	}
	if got := total("/api/v1/attachments?group=0"); got != 1 {
		t.Errorf("ungrouped filter: got %d, want 1", got)
	}
}
