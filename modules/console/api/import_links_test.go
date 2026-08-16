package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	ghpkg "blog/internal/github"
	tmpl "blog/internal/template"
)

// friendIssueBody 构造带 ```json 友链信息块的 issue body。
func friendIssueBody(t, url, feed string) string {
	return "### 友链信息\n```json\n{\"title\":\"测试友链\",\"url\":\"" + url + "\",\"feed\":\"" + feed + "\"}\n```"
}

func TestExtractJSONBlock(t *testing.T) {
	body := "### 友链信息\n```json\n{\"title\":\"x\"}\n```\ntrailing"
	block := extractJSONBlock(body)
	if !strings.Contains(block, `"title":"x"`) {
		t.Fatalf("expected json block, got %q", block)
	}
	if extractJSONBlock("no block here") != "" {
		t.Fatal("expected empty block for body without ```json")
	}
	// 多个块取第一个
	multi := "```json\n{\"a\":1}\n```\n```json\n{\"b\":2}\n```"
	if got := extractJSONBlock(multi); got != "{\"a\":1}" {
		t.Fatalf("expected first block, got %q", got)
	}
}

func TestParseFriendIssue(t *testing.T) {
	body := "### 友链信息\n```json\n{\"title\":\"小屋\",\"url\":\"https://x.top/\",\"avatar\":\"https://x.top/a.png\",\"icon\":\"https://x.top/i.png\",\"description\":\"desc\",\"feed\":\"https://x.top/atom.xml\",\"rss\":\"https://x.top/rss.xml\"}\n```"
	link, err := parseFriendIssue(ghpkg.Issue{Number: 10, Title: "issue-title", Body: body, Labels: []struct {
		Name string `json:"name"`
	}{{Name: "失联"}}})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if link.Name != "小屋" || link.URL != "https://x.top/" {
		t.Errorf("name/url mismatch: %+v", link)
	}
	if link.Avatar != "https://x.top/a.png" {
		t.Errorf("avatar should prefer avatar over icon: %q", link.Avatar)
	}
	if link.Description != "desc" || link.RSS != "https://x.top/atom.xml" {
		t.Errorf("description/rss mismatch: %+v", link)
	}
	if link.Label != "失联" {
		t.Errorf("label mismatch: %q", link.Label)
	}

	// icon 别名：无 avatar 时取 icon；无 feed 时取 rss
	link2, err := parseFriendIssue(ghpkg.Issue{Number: 9, Body: "### 友链信息\n```json\n{\"title\":\"t\",\"url\":\"https://y.top\",\"icon\":\"https://y.top/i.png\",\"rss\":\"https://y.top/rss.xml\"}\n```"})
	if err != nil {
		t.Fatalf("parse2: %v", err)
	}
	if link2.Avatar != "https://y.top/i.png" || link2.RSS != "https://y.top/rss.xml" {
		t.Errorf("alias mapping mismatch: %+v", link2)
	}

	// title 回退 issue.Title
	link3, err := parseFriendIssue(ghpkg.Issue{Number: 1, Title: "站点名", Body: "### 友链信息\n```json\n{\"url\":\"https://z.top\"}\n```"})
	if err != nil {
		t.Fatalf("parse3: %v", err)
	}
	if link3.Name != "站点名" {
		t.Errorf("title fallback mismatch: %q", link3.Name)
	}
}

func TestParseFriendIssueErrors(t *testing.T) {
	if _, err := parseFriendIssue(ghpkg.Issue{Number: 3, Body: "no json block"}); err == nil {
		t.Fatal("expected error for missing json block")
	}
	if _, err := parseFriendIssue(ghpkg.Issue{Number: 4, Body: "### 友链信息\n```json\n{\"title\":\"x\"}\n```"}); err == nil {
		t.Fatal("expected error for missing url")
	}
	if _, err := parseFriendIssue(ghpkg.Issue{Number: 5, Body: "### 友链信息\n```json\nnot json\n```"}); err == nil {
		t.Fatal("expected error for invalid json")
	}
}

func TestNormalizeLinkURL(t *testing.T) {
	cases := map[string]string{
		"https://x.top/":    "https://x.top",
		"https://x.top":     "https://x.top",
		" https://y.top/  ": "https://y.top",
	}
	for in, want := range cases {
		if got := normalizeLinkURL(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestImportLinksNoGHClient(t *testing.T) {
	// 安全审计 2026-08-16：links/import 收进 sys 组——无密码站点不再历史放行（403）；
	// 503（未装配 gh client）改由启用密码 + admin cookie 的路径覆盖。
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	site := &tmpl.Site{}
	r := gin.New()
	if _, err := Mount(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site)}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links/import", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("passwordless site should lock sys endpoints (403), got %d", w.Code)
	}

	enableAdmin(t, st, site, "secret")
	admin, err := st.GetUserByUsername("admin")
	if err != nil || admin == nil {
		t.Fatalf("get admin: %v", err)
	}
	w2 := do2FA(r, http.MethodPost, "/api/v1/links/import", `{}`, authCookie(admin))
	if w2.Code != http.StatusServiceUnavailable {
		t.Errorf("admin with no gh client should get 503, got %d body=%s", w2.Code, w2.Body.String())
	}
}

func TestImportLinksUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "secret")
	r := gin.New()
	if _, err := Mount(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site)}, GH: ghpkg.New(ghpkg.Config{CacheDir: t.TempDir()})}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links/import", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestFormatImportFail(t *testing.T) {
	if got := formatImportFail(7, "缺 url"); got != "issue #7: 缺 url" {
		t.Errorf("format mismatch: %q", got)
	}
}

// --- Spec 56：粘贴文本导入（source=text） ---

func TestParseTextLinksJSONArray(t *testing.T) {
	content := `[
		{"name":"小屋","url":"https://a.top/","avatar":"https://a.top/a.png","description":"desc","rss":"https://a.top/atom.xml","label":"大佬","color":"#1BCDFC","sort":3},
		{"title":"旧格式","url":"https://b.top","icon":"https://b.top/i.png","feed":"https://b.top/feed"},
		{"name":"缺url"},
		{"url":"https://c.top"}
	]`
	links, failed, err := parseTextLinks(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}
	if len(failed) != 2 || !strings.Contains(failed[0], "缺 url") || !strings.Contains(failed[1], "缺 name/title") {
		t.Errorf("failed mismatch: %v", failed)
	}
	l := links[0]
	if l.Name != "小屋" || l.URL != "https://a.top/" || l.Label != "大佬" || l.Sort != 3 {
		t.Errorf("entry1 mismatch: %+v", l)
	}
	if l.Color != "1BCDFC" {
		t.Errorf("color should strip '#': %q", l.Color)
	}
	// 别名回退：title/icon/feed
	l2 := links[1]
	if l2.Name != "旧格式" || l2.Avatar != "https://b.top/i.png" || l2.RSS != "https://b.top/feed" {
		t.Errorf("alias fallback mismatch: %+v", l2)
	}
}

func TestParseTextLinksYAML(t *testing.T) {
	// 顶层列表
	list := "- name: 甲\n  url: https://a.top\n- name: 乙\n  url: https://b.top\n"
	links, failed, err := parseTextLinks(list)
	if err != nil || len(links) != 2 || len(failed) != 0 {
		t.Fatalf("yaml list: links=%d failed=%v err=%v", len(links), failed, err)
	}
	// links 顶层键
	wrap := "links:\n  - name: 丙\n    url: https://c.top\n"
	links, _, err = parseTextLinks(wrap)
	if err != nil || len(links) != 1 || links[0].Name != "丙" {
		t.Fatalf("yaml wrap: links=%+v err=%v", links, err)
	}
}

func TestParseTextLinksErrors(t *testing.T) {
	cases := []string{"", "   ", "这是一段纯文本", "[]", `{"foo":"bar"}`}
	for _, c := range cases {
		if _, _, err := parseTextLinks(c); err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

// TestImportLinksTextSource 端到端：认证后 text 导入 → 去重幂等 → 默认分组。
func TestImportLinksTextSource(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	cookie := authCookie(user)

	body := `{"source":"text","content":"[{\"name\":\"甲\",\"url\":\"https://a.top\"},{\"name\":\"乙\",\"url\":\"https://b.top\",\"rss\":\"https://b.top/feed\"}]"}`
	w := do2FA(r, http.MethodPost, "/api/v1/links/import", body, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("import: status = %d, body = %s", w.Code, w.Body.String())
	}
	var res linkImportResult
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res.Imported != 2 || res.Skipped != 0 || len(res.Failed) != 0 {
		t.Fatalf("round1 = %+v, want imported=2 skipped=0", res)
	}
	// 默认分组落库
	links, err := st.ListLinks()
	if err != nil || len(links) != 2 {
		t.Fatalf("list links: %v (n=%d)", err, len(links))
	}
	for _, l := range links {
		if l.GroupTitle != "导入友链" {
			t.Errorf("default group mismatch: %q", l.GroupTitle)
		}
	}

	// 再次导入：全部跳过（URL 去重幂等）
	w = do2FA(r, http.MethodPost, "/api/v1/links/import", body, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("reimport: status = %d", w.Code)
	}
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res.Imported != 0 || res.Skipped != 2 {
		t.Fatalf("round2 = %+v, want imported=0 skipped=2", res)
	}

	// 非法内容 → 400
	w = do2FA(r, http.MethodPost, "/api/v1/links/import", `{"source":"text","content":"不是列表"}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("bad content: status = %d, want 400", w.Code)
	}
	// 未知来源 → 400
	w = do2FA(r, http.MethodPost, "/api/v1/links/import", `{"source":"opml"}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("unknown source: status = %d, want 400", w.Code)
	}
}
