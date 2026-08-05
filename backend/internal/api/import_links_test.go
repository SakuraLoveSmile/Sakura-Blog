package api

import (
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
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	r := gin.New()
	if _, err := Mount(r, MountDeps{Config: Config{Store: st, Site: &tmpl.Site{}}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links/import", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 without gh client, got %d", w.Code)
	}
}

func TestImportLinksUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	site := &tmpl.Site{}
	site.Admin.Password = "secret"
	r := gin.New()
	if _, err := Mount(r, MountDeps{Config: Config{Store: st, Site: site}, GH: ghpkg.New(ghpkg.Config{CacheDir: t.TempDir()})}); err != nil {
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
