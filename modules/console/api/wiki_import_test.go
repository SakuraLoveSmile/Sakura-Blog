package api

// wiki_import_test.go: 从「文章」导入文库文档的 API 回归测试
// （内容复制 / 原文章只读 / Slug 跳过 / 请求校验 / 状态与父文档 / 变更事件）。
import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/config"
	"blog/internal/content"
	"blog/internal/post"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// mkImportablePost 直接建库插入文章，返回创建后的记录。
func mkImportablePost(t *testing.T, st *store.Store, slug, title, body, status string) *store.Post {
	t.Helper()
	p := &store.Post{
		Slug: slug, Title: title, ContentMD: body, ContentHTML: "<p>cached</p>",
		Status: status, StellarVersion: 2, StellarConfig: "{}", BlogExtensions: "{}",
	}
	if _, err := st.Create(p); err != nil {
		t.Fatalf("create post %s: %v", slug, err)
	}
	loaded, err := st.GetBySlug(slug)
	if err != nil || loaded == nil {
		t.Fatalf("load post %s: %v", slug, err)
	}
	return loaded
}

func importReq(r *gin.Engine, project, body string) (int, []byte) {
	w := wikiReq(r, http.MethodPost, "/api/v1/wiki/"+project+"/docs/import", body)
	return w.Code, w.Body.Bytes()
}

func decodeImportResp(t *testing.T, body []byte) WikiDocImportResponse {
	t.Helper()
	var resp WikiDocImportResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode import response: %v: %s", err, string(body))
	}
	return resp
}

// TestWikiImportCopiesArticleAndKeepsSourceReadOnly 导入复制标题/正文/slug，
// 文档默认草稿，原文章内容与状态不变，shortcode 原文逐字保留。
func TestWikiImportCopiesArticleAndKeepsSourceReadOnly(t *testing.T) {
	r, st := newWikiTestEnv(t)
	mkWikiParent(t, st, "stellar", "intro", 0)
	body := "# 快速开始\n\n正文段落。\n\n{% box 提示 %}\n保留的 shortcode。\n{% endbox %}\n"
	src := mkImportablePost(t, st, "guide-intro", "快速开始", body, store.StatusPublished)

	code, raw := importReq(r, "stellar", `{"slugs":["guide-intro"]}`)
	if code != http.StatusOK {
		t.Fatalf("import status = %d: %s", code, string(raw))
	}
	resp := decodeImportResp(t, raw)
	if len(resp.Imported) != 1 || len(resp.Skipped) != 0 || len(resp.Failed) != 0 {
		t.Fatalf("unexpected buckets: imported=%d skipped=%d failed=%d %s", len(resp.Imported), len(resp.Skipped), len(resp.Failed), string(raw))
	}
	item := resp.Imported[0]
	if item.PostSlug != "guide-intro" || item.Slug != "guide-intro" || item.Title != "快速开始" || item.Result != "imported" || item.DocID == 0 {
		t.Fatalf("unexpected imported item: %+v", item)
	}

	doc, err := st.GetWikidocBySlug("stellar", "guide-intro")
	if err != nil || doc == nil {
		t.Fatalf("doc should exist: %v", err)
	}
	if doc.Title != "快速开始" || doc.ContentMD != body {
		t.Fatalf("doc content mismatch: title=%q md=%q", doc.Title, doc.ContentMD)
	}
	if doc.Status != store.StatusDraft {
		t.Fatalf("default import status = %q, want draft", doc.Status)
	}
	if strings.TrimSpace(doc.ContentHTML) == "" {
		t.Fatal("imported doc should render content_html")
	}
	if doc.Order != 0 || doc.Parent != 0 {
		t.Fatalf("imported doc order/parent = %d/%d", doc.Order, doc.Parent)
	}
	if doc.StellarVersion != 2 || doc.StellarConfig != "{}" || doc.BlogExtensions != "{}" {
		t.Fatalf("imported doc stellar metadata = %d/%q/%q", doc.StellarVersion, doc.StellarConfig, doc.BlogExtensions)
	}

	after, err := st.GetBySlug("guide-intro")
	if err != nil || after == nil {
		t.Fatalf("source post should remain: %v", err)
	}
	if after.ContentMD != src.ContentMD || after.Status != store.StatusPublished || after.Title != src.Title {
		t.Fatalf("source post was modified: %+v", after)
	}
}

// TestWikiImportSkipsExistingDocWithoutOverwrite 同 Slug 已存在时跳过并保留原文档。
func TestWikiImportSkipsExistingDocWithoutOverwrite(t *testing.T) {
	r, st := newWikiTestEnv(t)
	mkWikiParent(t, st, "stellar", "intro", 0)
	existing := &store.WikiDoc{
		ProjectID: "stellar", Slug: "guide-intro", Title: "已有文档",
		ContentMD: "旧正文", ContentHTML: "<p>旧正文</p>", Status: store.StatusPublished,
	}
	if err := st.CreateWikidoc(existing); err != nil {
		t.Fatalf("create doc: %v", err)
	}
	mkImportablePost(t, st, "guide-intro", "文章标题", "新正文", store.StatusPublished)

	code, raw := importReq(r, "stellar", `{"slugs":["guide-intro"]}`)
	if code != http.StatusOK {
		t.Fatalf("import status = %d: %s", code, string(raw))
	}
	resp := decodeImportResp(t, raw)
	if len(resp.Imported) != 0 || len(resp.Skipped) != 1 {
		t.Fatalf("expected only skipped bucket, got %s", string(raw))
	}
	if resp.Skipped[0].Result != "skipped" || !strings.Contains(resp.Skipped[0].Reason, "同名 Slug") {
		t.Fatalf("unexpected skip item: %+v", resp.Skipped[0])
	}
	doc, err := st.GetWikidocBySlug("stellar", "guide-intro")
	if err != nil || doc == nil {
		t.Fatalf("doc should still exist: %v", err)
	}
	if doc.Title != "已有文档" || doc.ContentMD != "旧正文" || doc.Status != store.StatusPublished {
		t.Fatalf("existing doc was overwritten: %+v", doc)
	}

	// 重复导入同一批：仍然跳过，不产生重复文档。
	_, raw = importReq(r, "stellar", `{"slugs":["guide-intro","guide-intro"]}`)
	resp = decodeImportResp(t, raw)
	if len(resp.Imported) != 0 || len(resp.Skipped) != 1 {
		t.Fatalf("duplicate import should skip: %s", string(raw))
	}
}

// TestWikiImportRejectsInvalidRequests 校验项目、请求体、状态、父文档与批量上限。
func TestWikiImportRejectsInvalidRequests(t *testing.T) {
	r, st := newWikiTestEnv(t)
	parentID := mkWikiParent(t, st, "stellar", "intro", 0)
	otherID := mkWikiParent(t, st, "other", "x", 0)
	mkImportablePost(t, st, "guide-intro", "快速开始", "正文", store.StatusPublished)

	cases := []struct {
		name    string
		project string
		body    string
		want    int
	}{
		{"项目不存在", "missing-project", `{"slugs":["guide-intro"]}`, http.StatusNotFound},
		{"空列表", "stellar", `{"slugs":[]}`, http.StatusBadRequest},
		{"空白 slug", "stellar", `{"slugs":["  ",""]}`, http.StatusBadRequest},
		{"非法状态", "stellar", `{"slugs":["guide-intro"],"status":"trashed"}`, http.StatusBadRequest},
		{"父文档不存在", "stellar", `{"slugs":["guide-intro"],"parent":99999}`, http.StatusBadRequest},
		{"跨项目父文档", "stellar", `{"slugs":["guide-intro"],"parent":` + strconv.FormatInt(otherID, 10) + `}`, http.StatusBadRequest},
		{"超上限", "stellar", `{"slugs":[` + repeatedSlugJSON(wikiDocImportMaxItems+1) + `]}`, http.StatusBadRequest},
		{"请求格式错误", "stellar", `{"slugs":`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, raw := importReq(r, tc.project, tc.body)
			if code != tc.want {
				t.Fatalf("%s: status = %d, want %d: %s", tc.name, code, tc.want, string(raw))
			}
		})
	}

	// 合法父文档但空列表：仍按请求校验返回 400，不写入。
	code, _ := importReq(r, "stellar", `{"slugs":[],"parent":`+strconv.FormatInt(parentID, 10)+`}`)
	if code != http.StatusBadRequest {
		t.Fatalf("empty slugs with parent: status = %d", code)
	}
}

// TestWikiImportReportsPerItemFailures 单篇失败只记入 failed，不影响同批其它文章。
func TestWikiImportReportsPerItemFailures(t *testing.T) {
	r, st := newWikiTestEnv(t)
	mkWikiParent(t, st, "stellar", "intro", 0)
	mkImportablePost(t, st, "ok-post", "可导入", "正文", store.StatusPublished)
	emptyPost := mkImportablePost(t, st, "empty-post", "无正文", "占位", store.StatusPublished)
	if _, err := st.DB().Exec(`UPDATE posts SET content_md = '' WHERE slug = ?`, emptyPost.Slug); err != nil {
		t.Fatalf("blank source body: %v", err)
	}
	trashed := mkImportablePost(t, st, "trashed-post", "回收站文章", "正文", store.StatusPublished)
	if _, err := st.DB().Exec(`UPDATE posts SET deleted_at = '2026-11-01T00:00:00Z' WHERE slug = ?`, trashed.Slug); err != nil {
		t.Fatalf("trash source post: %v", err)
	}

	code, raw := importReq(r, "stellar", `{"slugs":["ok-post","empty-post","trashed-post","no-such-post","ok-post"]}`)
	if code != http.StatusOK {
		t.Fatalf("import status = %d: %s", code, string(raw))
	}
	resp := decodeImportResp(t, raw)
	if len(resp.Imported) != 1 || resp.Imported[0].PostSlug != "ok-post" {
		t.Fatalf("imported bucket = %+v", resp.Imported)
	}
	if len(resp.Failed) != 3 {
		t.Fatalf("failed bucket count = %d: %s", len(resp.Failed), string(raw))
	}
	reasons := map[string]string{}
	for _, item := range resp.Failed {
		reasons[item.PostSlug] = item.Reason
	}
	if !strings.Contains(reasons["empty-post"], "Markdown 原文") {
		t.Fatalf("empty body reason = %q", reasons["empty-post"])
	}
	if !strings.Contains(reasons["trashed-post"], "回收站") {
		t.Fatalf("trashed reason = %q", reasons["trashed-post"])
	}
	if !strings.Contains(reasons["no-such-post"], "不存在") {
		t.Fatalf("missing reason = %q", reasons["no-such-post"])
	}
	if doc, _ := st.GetWikidocBySlug("stellar", "ok-post"); doc == nil {
		t.Fatal("successful item should be written")
	}
}

// TestWikiImportRespectsStatusAndParent 已发布导入可直接在线阅读，父文档生效。
func TestWikiImportRespectsStatusAndParent(t *testing.T) {
	r, st := newWikiTestEnv(t)
	parentID := mkWikiParent(t, st, "stellar", "intro", 0)
	mkImportablePost(t, st, "api-post", "接口文档", "正文", store.StatusDraft)

	code, raw := importReq(r, "stellar", `{"slugs":["api-post"],"status":"published","parent":`+strconv.FormatInt(parentID, 10)+`}`)
	if code != http.StatusOK {
		t.Fatalf("import status = %d: %s", code, string(raw))
	}
	resp := decodeImportResp(t, raw)
	if len(resp.Imported) != 1 {
		t.Fatalf("import should succeed: %s", string(raw))
	}
	doc, err := st.GetWikidocBySlug("stellar", "api-post")
	if err != nil || doc == nil {
		t.Fatalf("doc should exist: %v", err)
	}
	if doc.Status != store.StatusPublished || doc.Parent != parentID {
		t.Fatalf("doc status/parent = %q/%d, want published/%d", doc.Status, doc.Parent, parentID)
	}

	// 已发布文档应出现在文档树中。
	w := wikiReq(r, http.MethodGet, "/api/v1/wiki/stellar/docs", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list docs status = %d", w.Code)
	}
	var list struct {
		Tree []store.TreeNode `json:"tree"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode docs list: %v", err)
	}
	found := false
	var walk func(nodes []store.TreeNode)
	walk = func(nodes []store.TreeNode) {
		for _, node := range nodes {
			if node.Doc != nil && node.Doc.Slug == "api-post" {
				found = true
			}
			walk(node.Children)
		}
	}
	walk(list.Tree)
	if !found {
		t.Fatalf("imported published doc should be in tree: %s", w.Body.String())
	}
}

// TestWikiImportPublishesContentEventOnlyWhenWritten 只有实际写入才广播内容变更。
func TestWikiImportPublishesContentEventOnlyWhenWritten(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	if err := st.EnsureSettingsTable(); err != nil {
		t.Fatal(err)
	}
	if _, err := config.InitializeStellarV2(st, map[string]any{}, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	events := content.NewEvents()
	count := 0
	sub := events.Subscribe(func(e content.Event) {
		if e.Kind != content.ContentChanged {
			t.Fatalf("unexpected event %s", e.Kind)
		}
		count++
	})
	defer sub.Close()
	r := gin.New()
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Posts: post.NewService(filepath.Join(t.TempDir(), "posts")), RenderStellarMarkdown: func(input content.StellarBodyInput) (string, error) {
		loaded, err := config.LoadStellarV2(st)
		if err != nil {
			return "", err
		}
		return post.NewService("").RenderStellarV2WithContext(input.Markdown, post.StellarRenderContext{Theme: loaded.Theme, Page: input.Page, Language: content.ResolveStellarLanguage(input.Page, loaded.Blog), BlogExtensions: true})
	}, Site: sitePtr(&tmpl.Site{}), Events: events}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	mkWikiParent(t, st, "stellar", "intro", 0)
	mkImportablePost(t, st, "guide-intro", "快速开始", "正文", store.StatusPublished)

	if code, raw := importReq(r, "stellar", `{"slugs":["guide-intro"]}`); code != http.StatusOK || count != 1 {
		t.Fatalf("first import status=%d count=%d %s", code, count, string(raw))
	}
	if code, _ := importReq(r, "stellar", `{"slugs":["guide-intro"]}`); code != http.StatusOK || count != 1 {
		t.Fatalf("skipped import should not publish event: status=%d count=%d", code, count)
	}
}

func repeatedSlugJSON(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = `"post-` + strconv.Itoa(i) + `"`
	}
	return strings.Join(parts, ",")
}
