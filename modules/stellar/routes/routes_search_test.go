package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/search"
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
)

func TestSearchItemsFromPosts(t *testing.T) {
	posts := []*store.Post{
		{Slug: "open", Title: "公开文章", ContentHTML: "<p>正文内容</p>"},
		{Slug: "locked", Title: "加密文章", ContentHTML: "<p>秘密</p>", Password: "pw"},
	}
	items := searchItemsFromPosts(posts)
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
	if items[0].Path != "/post/open" || items[0].Content != "正文内容" {
		t.Errorf("open item mismatch: %+v", items[0])
	}
	// 受密码保护文章：标题保留、正文不泄漏
	if items[1].Title != "加密文章" || items[1].Content != "" {
		t.Errorf("protected post must not leak content: %+v", items[1])
	}
	if strings.Contains(items[1].Content, "秘密") {
		t.Error("protected content leaked")
	}
}

func TestSearchItemsFromPostsConfig(t *testing.T) {
	posts := []*store.Post{{Slug: "private", Title: "私密", ContentHTML: "<p>机密正文</p>"}}
	include := false
	items := searchItemsFromPostsWithConfig(posts, tmpl.SearchConfig{
		LocalSearch: tmpl.LocalSearchConfig{Content: &include, SkipSearch: []string{"/post/private"}},
	})
	if len(items) != 0 {
		t.Fatalf("skip_search should exclude the post, got %+v", items)
	}
	items = searchItemsFromPostsWithConfig(posts, tmpl.SearchConfig{
		LocalSearch: tmpl.LocalSearchConfig{Content: &include},
	})
	if len(items) != 1 || items[0].Content != "" {
		t.Fatalf("content=false should keep title and remove body, got %+v", items)
	}
	items = searchItemsFromPostsWithConfig(posts, tmpl.SearchConfig{
		LocalSearch: tmpl.LocalSearchConfig{Field: "page"},
	})
	if len(items) != 0 {
		t.Fatalf("field=page should exclude posts, got %+v", items)
	}
}

func TestSearchPathAllowedNormalizesPrefixes(t *testing.T) {
	cfg := tmpl.SearchConfig{LocalSearch: tmpl.LocalSearchConfig{SkipSearch: []string{"/wiki/private"}}}
	if searchPathAllowed("/wiki/private/chapter", cfg) {
		t.Fatal("skip_search prefix should exclude nested paths")
	}
	if !searchPathAllowed("/wiki/public", cfg) {
		t.Fatal("unmatched path should remain searchable")
	}
}

// TestAPISearchTypeFilter /api/search 支持 type 参数（post/page/moment），
// 返回条目携带 type 且路径按类型拼接；空 type 返回全部三类。
func TestAPISearchTypeFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	if err := st.EnsurePagesTable(); err != nil {
		t.Fatalf("ensure pages: %v", err)
	}
	if err := st.EnsureMomentsTable(); err != nil {
		t.Fatalf("ensure moments: %v", err)
	}
	idx := search.New(st.DB())
	if err := idx.Ensure(); err != nil {
		t.Fatalf("ensure fts: %v", err)
	}
	// 直接入索引（隔离端点测试点，不走写路径 hook）
	if err := idx.IndexDoc(search.TypePost, "p1", "文章标题", "文章正文含独特词甲"); err != nil {
		t.Fatal(err)
	}
	if err := idx.IndexDoc(search.TypePage, "about", "页面标题", "页面正文含独特词乙"); err != nil {
		t.Fatal(err)
	}
	if err := idx.IndexDoc(search.TypeMoment, "moment-1", "瞬间标题", "瞬间正文含独特词丙"); err != nil {
		t.Fatal(err)
	}

	ws := &webServer{st: st, searchIdx: idx}
	r := gin.New()
	ws.registerSearchRoutes(r)

	doSearch := func(q, docType string) []map[string]any {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/search?q="+q+"&type="+docType, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("search q=%s type=%s: status=%d body=%s", q, docType, w.Code, w.Body.String())
		}
		var body struct {
			Items []map[string]any `json:"items"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return body.Items
	}

	// 空 type：三类全出
	if items := doSearch("独特词", ""); len(items) != 3 {
		t.Fatalf("search all want 3, got %d", len(items))
	}
	// 各类型过滤 + 路径拼接 + type 字段
	if items := doSearch("独特词", "post"); len(items) != 1 || items[0]["type"] != "post" || items[0]["path"] != "/post/p1" {
		t.Errorf("post filter mismatch: %v", items)
	}
	if items := doSearch("独特词", "page"); len(items) != 1 || items[0]["type"] != "page" || items[0]["path"] != "/pages/about" {
		t.Errorf("page filter mismatch: %v", items)
	}
	if items := doSearch("独特词", "moment"); len(items) != 1 || items[0]["type"] != "moment" || items[0]["path"] != "/moments" {
		t.Errorf("moment filter mismatch: %v", items)
	}
	// 非法类型无结果（不报错）
	if items := doSearch("独特词", "wiki"); len(items) != 0 {
		t.Errorf("unknown type should be empty, got %d", len(items))
	}
}

// TestSearchItemsAnchors 1.41 索引条目带章节锚点：标题偏移正确、JSON 序列化含 anchors。
func TestSearchItemsAnchors(t *testing.T) {
	posts := []*store.Post{{
		Slug: "sections", Title: "章节文",
		ContentHTML: `<p>引言部分</p><h2 id="setup">环境准备</h2><p>安装 Go 环境</p><h3 id="run">运行</h3><p>启动服务</p>`,
	}}
	items := searchItemsFromPosts(posts)
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	it := items[0]
	if len(it.Anchors) != 2 {
		t.Fatalf("want 2 anchors, got %+v", it.Anchors)
	}
	if it.Anchors[0].ID != "setup" || it.Anchors[0].Text != "环境准备" {
		t.Errorf("anchor0 = %+v", it.Anchors[0])
	}
	if it.Anchors[1].ID != "run" || it.Anchors[1].Text != "运行" {
		t.Errorf("anchor1 = %+v", it.Anchors[1])
	}
	// 偏移按 UTF-16 计（与 JS slice 一致；BMP 内与 rune 数等价）：标题首字符应对上
	runes := []rune(it.Content)
	if sub := string(runes[it.Anchors[0].Offset:]); !strings.HasPrefix(sub, "环境准备") {
		t.Errorf("anchor0 offset %d 指向 %q", it.Anchors[0].Offset, sub)
	}
	// 序列化：条目含 anchors 数组（前端章节化消费）
	b, err := json.Marshal(it)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"anchors":[{"id":"setup"`) {
		t.Errorf("json missing anchors: %s", b)
	}
}

// TestResultAnchorMapsHitSection 服务端搜索结果锚点映射：命中词落在第二章节时返回该章节锚点。
func TestResultAnchorMapsHitSection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	ws := &webServer{st: st}
	p := &store.Post{
		Slug: "anchored", Title: "锚点文", Status: store.StatusPublished,
		ContentHTML: `<h2 id="intro">介绍</h2><p>这是介绍</p><h2 id="deploy">部署</h2><p>部署部署部署</p>`,
	}
	if _, err := ws.st.Create(p); err != nil {
		t.Fatalf("create: %v", err)
	}
	id, text := ws.resultAnchor(search.TypePost, "anchored", []string{"部署"})
	if id != "deploy" || text != "部署" {
		t.Fatalf("anchor = (%q, %q), want (deploy, 部署)", id, text)
	}
	// 命中首章节之前的引言 → 无锚点
	id, text = ws.resultAnchor(search.TypePost, "anchored", []string{"不存在的词"})
	if id != "" || text != "" {
		t.Fatalf("no-hit should be empty, got (%q, %q)", id, text)
	}
}
