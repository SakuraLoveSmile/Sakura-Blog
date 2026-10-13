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

	ensureRouteSchema(t, st)
	if _, err := st.Create(&store.Post{Slug: "p1", Title: "文章标题", ContentMD: "文章正文含独特词甲", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreatePage(&store.Page{Slug: "about", Title: "页面标题", ContentMD: "页面正文含独特词乙", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateMoment(&store.Moment{ContentMD: "瞬间正文含独特词丙", ContentHTML: "瞬间正文含独特词丙", Visibility: store.MomentVisibilityPublic}); err != nil {
		t.Fatal(err)
	}

	// 索引并不等于公开授权：过期行及私密瞬间即使命中也必须过滤。
	if _, err := st.CreateMoment(&store.Moment{ContentMD: "私密瞬间含独特词", ContentHTML: "私密瞬间含独特词", Visibility: store.MomentVisibilityPrivate}); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []struct{ kind, slug string }{{search.TypePost, "stale"}, {search.TypeMoment, "moment-2"}} {
		if err := idx.IndexDoc(candidate.kind, candidate.slug, "隐藏标题", "隐藏正文含独特词"); err != nil {
			t.Fatal(err)
		}
	}
	ws, _ := newWikiVerifyServer(t)
	ws.st, ws.searchIdx = st, idx
	ws.site.Store(canonicalRouteSite(t, "搜索测试", nil, map[string]any{"search": map[string]any{"provider": "go"}}))
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
	if items := doSearch("独特词", "page"); len(items) != 1 || items[0]["type"] != "page" || items[0]["path"] != "/about" {
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

func TestStellarSearchPathsStayWithinSubpath(t *testing.T) {
	ws, st, _, r := stellarRouteFixture(t)
	site := *ws.site.Load()
	site.URL = "https://fixture.example/sub"
	version := *site.StellarV2
	version.Theme = stellarCopyObject(version.Theme)
	searchConfig := stellarCopyObject(stellarObject(version.Theme["search"]))
	local := stellarCopyObject(stellarObject(searchConfig["local"]))
	local["scope"] = "all"
	searchConfig["local"] = local
	version.Theme["search"] = searchConfig
	site.StellarV2 = &version
	ws.site.Store(&site)
	row := createStellarPost(t, st, "marker", "")
	if err := ws.searchIdx.IndexDoc(search.TypePost, row.Slug, row.Title, row.ContentMD); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWikiProject(&store.WikiProject{ID: "docs", Visible: true, StellarVersion: 2, StellarConfig: `{"name":"文档","route":{"path":"/docs/"}}`}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "docs", Slug: "intro", Title: "文档", ContentMD: "marker", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{"collection":{"profile":"wiki","id":"docs"}}`}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateNotebook(&store.Notebook{ID: "journal", StellarVersion: 2, StellarConfig: `{"name":"笔记","route":{"path":"/journal/"}}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateNote(&store.Note{NotebookID: "journal", Slug: "entry", Title: "笔记", ContentMD: "marker", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{"collection":{"profile":"notebook","id":"journal"}}`}); err != nil {
		t.Fatal(err)
	}
	moment := &store.Moment{ContentMD: "marker", Visibility: store.MomentVisibilityPublic}
	if _, err := st.CreateMoment(moment); err != nil {
		t.Fatal(err)
	}
	if err := ws.searchIdx.IndexDoc(search.TypeMoment, store.MomentSlugKey(moment.ID), "marker", moment.ContentMD); err != nil {
		t.Fatal(err)
	}
	w := stellarRequest(r, "/sub/search.json")
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	var localItems []stellarSearchItem
	if err := json.Unmarshal(w.Body.Bytes(), &localItems); err != nil {
		t.Fatal(err)
	}
	localPaths := map[string]bool{}
	for _, item := range localItems {
		localPaths[item.Path] = true
		if target := stellarRequest(r, item.Path); target.Code != http.StatusOK {
			t.Errorf("local search target %s => %d", item.Path, target.Code)
		}
	}
	for _, want := range []string{"/sub/post/marker/", "/sub/docs/intro/", "/sub/journal/entry/"} {
		if !localPaths[want] {
			t.Errorf("local search missing subpath %s: %#v", want, localPaths)
		}
	}
	w = stellarRequest(r, "/sub/api/search?q=marker")
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	var response stellarSearchAPIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	apiPaths := map[string]bool{}
	for _, item := range response.Items {
		apiPaths[item.Path] = true
		if target := stellarRequest(r, item.Path); target.Code != http.StatusOK {
			t.Errorf("search target %s => %d", item.Path, target.Code)
		}
		if strings.Contains(item.Path, "/sub/sub/") {
			t.Errorf("search target repeated root: %s", item.Path)
		}
	}
	for _, want := range []string{"/sub/post/marker", "/sub/docs/intro", "/sub/journal/entry", "/sub/moments"} {
		if !apiPaths[want] {
			t.Errorf("server search missing subpath %s: %#v", want, apiPaths)
		}
	}
}

func TestStellarGoSearchScopeDomainAndContentPolicy(t *testing.T) {
	ws, st, _, router := stellarRouteFixture(t)
	row := createStellarPost(t, st, "headline", "")
	if err := ws.searchIdx.IndexDoc(search.TypePost, row.Slug, row.Title, row.ContentMD); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWikiProject(&store.WikiProject{ID: "docs", Visible: true, StellarVersion: 2, StellarConfig: `{"name":"文档"}`}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "docs", Slug: "intro", Title: "文档标题", ContentMD: "bodymarker", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{"title":"文档标题","collection":{"profile":"wiki","id":"docs"}}`}); err != nil {
		t.Fatal(err)
	}
	read := func(query string) []stellarSearchAPIItem {
		t.Helper()
		response := stellarRequest(router, "/api/search?"+query)
		if response.Code != http.StatusOK {
			t.Fatal(response.Code, response.Body.String())
		}
		var result stellarSearchAPIResponse
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result.Items
	}
	for _, query := range []string{"q=bodymarker&scope=post", "q=bodymarker&domain=blog", "q=bodymarker&domain=wiki:other", "q=bodymarker&include_content=false"} {
		if items := read(query); len(items) != 0 {
			t.Fatalf("query %s escaped policy: %#v", query, items)
		}
	}
	if items := read("q=bodymarker&scope=page&domain=wiki:docs"); len(items) != 1 || items[0].Type != "wiki" || len(items[0].Domains) != 1 || items[0].Domains[0] != "wiki:docs" {
		t.Fatalf("collection search: %#v", items)
	}
	if items := read("q=Go&include_content=false&domain=blog"); len(items) != 1 || items[0].Snippet != "" || items[0].AnchorID != "" {
		t.Fatalf("metadata-only search: %#v", items)
	}
	site := *ws.site.Load()
	version := *site.StellarV2
	version.Theme = stellarCopyObject(version.Theme)
	searchConfig := stellarCopyObject(stellarObject(version.Theme["search"]))
	local := stellarCopyObject(stellarObject(searchConfig["local"]))
	local["includeContent"] = false
	searchConfig["local"] = local
	version.Theme["search"] = searchConfig
	site.StellarV2 = &version
	ws.site.Store(&site)
	if items := read("q=bodymarker&include_content=true&scope=all"); len(items) != 0 {
		t.Fatalf("request expanded disabled server content policy: %#v", items)
	}
	local["scope"] = "post"
	if items := read("q=文档标题&scope=all"); len(items) != 0 {
		t.Fatalf("request expanded server scope: %#v", items)
	}
	if response := stellarRequest(router, "/api/search?q=marker&scope=bad"); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid scope status: %d", response.Code)
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
