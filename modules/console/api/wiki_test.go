package api

// wiki_test.go: 文库文档 API parent 校验回归测试
// （父文档存在性 / 项目归属 / 自引用 / 循环引用）。
import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/post"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// newWikiTestEnv 无密码（免认证）测试环境，含 post service（createWikidoc 需渲染 Markdown）。
func newWikiTestEnv(t *testing.T) (*gin.Engine, *store.Store) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	r := gin.New()
	posts := post.NewService(filepath.Join(t.TempDir(), "posts"))
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Posts: posts, Site: sitePtr(&tmpl.Site{})}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	return r, st
}

// wikiReq 发起 wiki 文档 API 请求（JSON body）。
func wikiReq(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// mkWikiParent 直接建库插入文档，返回 ID。
func mkWikiParent(t *testing.T, st *store.Store, projectID, slug string, parent int64) int64 {
	t.Helper()
	d := &store.WikiDoc{
		ProjectID:   projectID,
		Slug:        slug,
		Title:       slug,
		Parent:      parent,
		ContentMD:   "# " + slug,
		ContentHTML: "<h1>" + slug + "</h1>",
		Status:      store.StatusPublished,
	}
	if err := st.CreateWikidoc(d); err != nil {
		t.Fatalf("create wikidoc %s/%s: %v", projectID, slug, err)
	}
	return d.ID
}

// TestWikidocCreateParentValidation 创建文档时 parent 必须存在且属于当前项目。
func TestWikidocCreateParentValidation(t *testing.T) {
	r, st := newWikiTestEnv(t)
	aID := mkWikiParent(t, st, "stellar", "a", 0)
	// 其它项目文档：不应被当前项目引用
	xID := mkWikiParent(t, st, "other", "x", 0)

	// parent = 合法父文档 → 201
	w := wikiReq(r, http.MethodPost, "/api/v1/wiki/stellar/docs",
		`{"slug":"b","title":"B","order":0,"parent":`+strconv.FormatInt(aID, 10)+`,"content_md":"","status":"draft"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("valid parent should create, got %d: %s", w.Code, w.Body.String())
	}

	// parent = 不存在的 ID → 400
	w = wikiReq(r, http.MethodPost, "/api/v1/wiki/stellar/docs",
		`{"slug":"c","title":"C","order":0,"parent":99999,"content_md":"","status":"draft"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("nonexistent parent should be 400, got %d", w.Code)
	}

	// parent = 其它项目的文档 ID → 400
	w = wikiReq(r, http.MethodPost, "/api/v1/wiki/stellar/docs",
		`{"slug":"d","title":"D","order":0,"parent":`+strconv.FormatInt(xID, 10)+`,"content_md":"","status":"draft"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("cross-project parent should be 400, got %d", w.Code)
	}

	// parent = 0（顶级）→ 201
	w = wikiReq(r, http.MethodPost, "/api/v1/wiki/stellar/docs",
		`{"slug":"e","title":"E","order":0,"parent":0,"content_md":"","status":"draft"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("top-level create should succeed, got %d: %s", w.Code, w.Body.String())
	}
}

// TestWikidocUpdateParentValidation 更新文档时拒绝自引用与循环引用。
func TestWikidocUpdateParentValidation(t *testing.T) {
	r, st := newWikiTestEnv(t)
	aID := mkWikiParent(t, st, "stellar", "a", 0)
	a1ID := mkWikiParent(t, st, "stellar", "a1", aID)
	a1xID := mkWikiParent(t, st, "stellar", "a1x", a1ID)

	// 父文档改为自身 → 400
	w := wikiReq(r, http.MethodPut, "/api/v1/wiki/stellar/docs/a1",
		`{"slug":"a1","title":"A1","order":0,"parent":`+strconv.FormatInt(a1ID, 10)+`,"content_md":"","status":"published"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("self parent should be 400, got %d: %s", w.Code, w.Body.String())
	}

	// 父文档为后代（a1x）→ 400（循环引用）
	w = wikiReq(r, http.MethodPut, "/api/v1/wiki/stellar/docs/a1",
		`{"slug":"a1","title":"A1","order":0,"parent":`+strconv.FormatInt(a1xID, 10)+`,"content_md":"","status":"published"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("descendant parent should be 400, got %d: %s", w.Code, w.Body.String())
	}

	// 父文档不存在 → 400
	w = wikiReq(r, http.MethodPut, "/api/v1/wiki/stellar/docs/a1",
		`{"slug":"a1","title":"A1","order":0,"parent":99999,"content_md":"","status":"published"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("nonexistent parent should be 400, got %d", w.Code)
	}

	// 合法：父文档改为 a（保持原层级）→ 200
	w = wikiReq(r, http.MethodPut, "/api/v1/wiki/stellar/docs/a1",
		`{"slug":"a1","title":"A1","order":0,"parent":`+strconv.FormatInt(aID, 10)+`,"content_md":"","status":"published"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("valid parent should update, got %d: %s", w.Code, w.Body.String())
	}

	// 合法：父文档置 0 升为顶级 → 200
	w = wikiReq(r, http.MethodPut, "/api/v1/wiki/stellar/docs/a1",
		`{"slug":"a1","title":"A1","order":0,"parent":0,"content_md":"","status":"published"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("promote to top should update, got %d: %s", w.Code, w.Body.String())
	}
}
