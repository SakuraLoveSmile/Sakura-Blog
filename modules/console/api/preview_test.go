package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// Spec 63：组件库标签预览（preview_profile=component-library）Fixture 模式测试。
// 覆盖：静态/交互/数据/外部插件渲染、data-api 改写与 preview- 隔离 ID、
// Fixture 端点三态（normal/empty/error）、预览请求不写入真实 Rating/Vote 数据、
// 以及缺省（无 profile）渲染行为不变。

func postPreview(t *testing.T, r *gin.Engine, cookie, body string) *httptest.ResponseRecorder {
	t.Helper()
	return do2FA(r, http.MethodPost, "/api/v1/preview/markdown", body, cookie)
}

// previewBody 渲染并返回 html 字段。
func previewBody(t *testing.T, r *gin.Engine, cookie, body string) string {
	t.Helper()
	w := postPreview(t, r, cookie, body)
	if w.Code != http.StatusOK {
		t.Fatalf("preview status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		HTML string `json:"html"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	return resp.HTML
}

func TestPreviewMarkdownNormalUnchanged(t *testing.T) {
	// 缺省（无 preview_profile）：渲染行为与线上一致，data-api 指向真实端点、ID 不加前缀。
	r, _, user, _ := newFullStellarEnv(t)
	cookie := authCookie(user)
	out := previewBody(t, r, cookie, `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","markdown":"{% rating id:demo-post %}\n\n{% friends %}"}`)
	for _, want := range []string{
		`class="tag-plugin ds-rating is-disabled"`,
		`data-id="demo-post"`,
		`class="tag-plugin users-wrap"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("normal preview missing %q: %s", want, out)
		}
	}
}

func TestPreviewMarkdownComponentLibraryRewrites(t *testing.T) {
	// Fixture 模式：rating/vote 端点改写 + preview- 隔离 ID；friends/sites 外部 URL 一律本地化。
	r, _, user, _ := newFullStellarEnv(t)
	cookie := authCookie(user)
	out := previewBody(t, r, cookie, `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","markdown":"{% rating id:demo-post %}\n\n{% vote 有帮助吗 id:post-1 %}\n\n{% friends api:https://example.com/data.json %}\n\n{% sites api:https://example.com/sites.json %}\n\n{% md https://raw.githubusercontent.com/x/y/main/README.md %}","preview_profile":"component-library"}`)
	for _, want := range []string{
		`data-api="/api/v1/preview/rating"`,
		`data-id="preview-demo-post"`,
		`data-api="/api/v1/preview/vote"`,
		`data-id="preview-post-1"`,
		`data-api="/api/v1/preview/friends"`,
		`data-api="/api/v1/preview/sites"`,
		`src="/api/v1/preview/md?src=`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("component-library preview missing %q: %s", want, out)
		}
	}
	// 公网 URL 不得作为直连端点残留（编码进查询参数的来源标识除外）
	for _, bad := range []string{"example.com/data.json", "example.com/sites.json", `/api/rating"`, `/api/vote"`} {
		if strings.Contains(out, bad) {
			t.Errorf("component-library preview should drop %q: %s", bad, out)
		}
	}
}

func TestPreviewMarkdownComponentLibraryGithubFixture(t *testing.T) {
	// ghrepo/ghuser/ghcard 服务端使用确定性 Fixture 数据（不发公网请求）。
	r, _, user, _ := newFullStellarEnv(t)
	cookie := authCookie(user)
	out := previewBody(t, r, cookie, `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","markdown":"{% blog_ghrepo vuejs/vue %}\n\n{% blog_ghuser octocat %}\n\n{% ghcard anuraghazra/github-readme-stats %}","preview_profile":"component-library"}`)
	for _, want := range []string{
		`<span class="repo-name">vuejs/vue</span>`,
		`>⭐ 12800<`,
		`<span class="user-login">@octocat</span>`,
		`<span>3200 followers</span>`,
		`<img src="/assets/img/preview/avatar-gh-vuejs.svg"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("github fixture missing %q: %s", want, out)
		}
	}
}

func TestPreviewMarkdownComponentLibraryStaticPlugins(t *testing.T) {
	// 代表性静态插件：note/banner/poetry/frame 结构完整。
	r, _, user, _ := newFullStellarEnv(t)
	cookie := authCookie(user)
	out := previewBody(t, r, cookie, `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","markdown":"{% note color:green 提示 这是提示内容 %}\n\n{% banner 我的博客 %}\n欢迎。\n{% endbanner %}\n\n{% poetry 静夜思 author:李白 %}\n床前明月光。\n{% endpoetry %}\n\n{% frame iphone img:https://example.com/s.png %}","preview_profile":"component-library"}`)
	for _, want := range []string{
		`class="tag-plugin colorful note"`,
		`color="green"`,
		`class="tag-plugin banner no-bg"`,
		`class="tag-plugin poetry"`,
		`<div class="meta"><span>李白</span></div>`,
		`id="iphone"`,
		`class="frame-wrap"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("static plugin preview missing %q: %s", want, out)
		}
	}
}

func TestPreviewMarkdownComponentLibraryInteractivePlugins(t *testing.T) {
	// 代表性交互插件：tabs/folding/swiper 结构完整（交互初始化由前端预览运行时负责）。
	r, _, user, _ := newFullStellarEnv(t)
	cookie := authCookie(user)
	out := previewBody(t, r, cookie, `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","markdown":"{% tabs %}\n<!-- tab 标签一 -->\n第一页。\n<!-- tab 标签二 -->\n第二页。\n{% endtabs %}\n\n{% folding color:cyan open:true 详情 %}\n折叠内容。\n{% endfolding %}\n\n{% swiper %}\n![](https://example.com/1.png)\n![](https://example.com/2.png)\n{% endswiper %}","preview_profile":"component-library"}`)
	for _, want := range []string{
		`class="tag-plugin tabs"`,
		`nav-tabs`,
		`<details class="tag-plugin colorful folding"`,
		`open`,
		`class="tag-plugin swiper fancybox"`,
		`<div class="swiper-slide">`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("interactive plugin preview missing %q: %s", want, out)
		}
	}
}

func TestPreviewMarkdownComponentLibraryExternalPlugins(t *testing.T) {
	// md：渲染为 ds-mdrender 容器，src 指向本地 Fixture 端点。
	r, _, user, _ := newFullStellarEnv(t)
	cookie := authCookie(user)
	out := previewBody(t, r, cookie, `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","markdown":"{% md https://raw.githubusercontent.com/xaoxuu/hexo-theme-stellar/main/README.md %}","preview_profile":"component-library"}`)
	for _, want := range []string{
		`class="data-service ds-mdrender"`,
		`src="/api/v1/preview/md?src=`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("external plugin preview missing %q: %s", want, out)
		}
	}
}

func TestPreviewMarkdownComponentLibraryLinksFixture(t *testing.T) {
	// {% blog_links %} 在 Fixture 模式服务端直出确定性分组。
	r, _, user, _ := newFullStellarEnv(t)
	cookie := authCookie(user)
	out := previewBody(t, r, cookie, `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","markdown":"{% blog_links %}","preview_profile":"component-library"}`)
	for _, want := range []string{
		`<h2>推荐站点</h2>`,
		`<span class="title">Fixture Blog</span>`,
		`<h2>小伙伴</h2>`,
		`<span class="title">Alice</span>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("links fixture missing %q: %s", want, out)
		}
	}
}

func TestPreviewFixtureRatingVoteNoDBWrite(t *testing.T) {
	// 预览 Fixture 端点：GET 返回确定性数据；POST 接受动作但不落库。
	r, st, user, _ := newFullEnv(t)
	cookie := authCookie(user)

	// 前置：无任何评分/投票记录
	if sum, err := st.GetRating("preview-demo"); err != nil || sum.Count != 0 {
		t.Fatalf("rating should be empty before fixture POST, got %+v err %v", sum, err)
	}
	if vc, err := st.GetVotes("preview-demo"); err != nil || vc.Up+vc.Down != 0 {
		t.Fatalf("vote should be empty before fixture POST, got %+v err %v", vc, err)
	}

	// rating GET 确定性数据
	w := do2FA(r, http.MethodGet, "/api/v1/preview/rating?id=preview-demo", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("fixture rating GET status = %d", w.Code)
	}
	var rating struct {
		Avg   float64 `json:"avg"`
		Count int     `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &rating); err != nil {
		t.Fatalf("decode fixture rating: %v", err)
	}
	if rating.Avg != 4.6 || rating.Count != 128 {
		t.Errorf("fixture rating = %+v, want avg 4.6 / count 128", rating)
	}

	// rating POST（评分动作）→ 200 且不落库
	w = do2FA(r, http.MethodPost, "/api/v1/preview/rating", `{"id":"preview-demo","score":5}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("fixture rating POST status = %d", w.Code)
	}
	if sum, err := st.GetRating("preview-demo"); err != nil || sum.Count != 0 {
		t.Errorf("fixture rating POST must not write DB rows, got %+v err %v", sum, err)
	}

	// vote info GET 确定性数据
	w = do2FA(r, http.MethodGet, "/api/v1/preview/vote/info?id=preview-demo", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("fixture vote info status = %d", w.Code)
	}
	var vote struct {
		Votes struct {
			Up   int `json:"up"`
			Down int `json:"down"`
		} `json:"votes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &vote); err != nil {
		t.Fatalf("decode fixture vote: %v", err)
	}
	if vote.Votes.Up != 86 || vote.Votes.Down != 12 {
		t.Errorf("fixture vote = %+v, want up 86 / down 12", vote.Votes)
	}

	// vote update POST → 200 且不落库
	w = do2FA(r, http.MethodPost, "/api/v1/preview/vote/update?id=preview-demo&value=up", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("fixture vote update status = %d", w.Code)
	}
	if vc, err := st.GetVotes("preview-demo"); err != nil || vc.Up+vc.Down != 0 {
		t.Errorf("fixture vote update must not write DB rows, got %+v err %v", vc, err)
	}

	// 真实端点不受影响：写真实 rating 仍可查询（对照验证隔离性）
	if err := st.VoteRating("real-post", 5, "hash"); err != nil {
		t.Fatalf("real rating write: %v", err)
	}
	sum, err := st.GetRating("real-post")
	if err != nil || sum == nil || sum.Count != 1 || sum.Avg != 5 {
		t.Errorf("real rating should be recorded, got %+v err %v", sum, err)
	}
}

func TestPreviewFixtureFriendsSitesMDStates(t *testing.T) {
	r, _, user, _ := newFullEnv(t)
	cookie := authCookie(user)

	// friends normal：非空 content
	w := do2FA(r, http.MethodGet, "/api/v1/preview/friends", "", cookie)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"title":"Fixture 博客"`) {
		t.Errorf("fixture friends normal: status=%d body=%s", w.Code, w.Body.String())
	}
	// friends empty
	w = do2FA(r, http.MethodGet, "/api/v1/preview/friends?state=empty", "", cookie)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"content":[]`) {
		t.Errorf("fixture friends empty: status=%d body=%s", w.Code, w.Body.String())
	}
	// friends error
	w = do2FA(r, http.MethodGet, "/api/v1/preview/friends?state=error", "", cookie)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("fixture friends error: status=%d, want 500", w.Code)
	}

	// sites normal
	w = do2FA(r, http.MethodGet, "/api/v1/preview/sites", "", cookie)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"title":"Fixture Search"`) {
		t.Errorf("fixture sites normal: status=%d body=%s", w.Code, w.Body.String())
	}
	// sites empty / error
	if w = do2FA(r, http.MethodGet, "/api/v1/preview/sites?state=empty", "", cookie); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"content":[]`) {
		t.Errorf("fixture sites empty: status=%d body=%s", w.Code, w.Body.String())
	}
	if w = do2FA(r, http.MethodGet, "/api/v1/preview/sites?state=error", "", cookie); w.Code != http.StatusInternalServerError {
		t.Errorf("fixture sites error: status=%d, want 500", w.Code)
	}

	// md normal：Markdown 文本（含标题与列表）
	w = do2FA(r, http.MethodGet, "/api/v1/preview/md?src=x", "", cookie)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "# 外部 Markdown Fixture") {
		t.Errorf("fixture md normal: status=%d body=%s", w.Code, w.Body.String())
	}
	// md empty / error
	if w = do2FA(r, http.MethodGet, "/api/v1/preview/md?src=x&state=empty", "", cookie); w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "" {
		t.Errorf("fixture md empty: status=%d body=%q", w.Code, w.Body.String())
	}
	if w = do2FA(r, http.MethodGet, "/api/v1/preview/md?src=x&state=error", "", cookie); w.Code != http.StatusInternalServerError {
		t.Errorf("fixture md error: status=%d, want 500", w.Code)
	}

	// rating/vote 也支持 empty/error 状态
	if w = do2FA(r, http.MethodGet, "/api/v1/preview/rating?id=x&state=empty", "", cookie); !strings.Contains(w.Body.String(), `"avg":0`) {
		t.Errorf("fixture rating empty: body=%s", w.Body.String())
	}
	if w = do2FA(r, http.MethodGet, "/api/v1/preview/vote/info?id=x&state=empty", "", cookie); !strings.Contains(w.Body.String(), `"up":0`) {
		t.Errorf("fixture vote empty: body=%s", w.Body.String())
	}
	if w = do2FA(r, http.MethodGet, "/api/v1/preview/rating?id=x&state=error", "", cookie); w.Code != http.StatusInternalServerError {
		t.Errorf("fixture rating error: status=%d, want 500", w.Code)
	}
}

// 确保 store 导入被使用（GetRating/GetVotes 返回类型引用）。
var _ = store.RatingSummary{}

// ---------- Spec 64：friends variant（albums/posters） ----------

func TestPreviewFriendsVariantAlbumsPosters(t *testing.T) {
	// /api/v1/preview/friends?variant=albums|posters 分别返回方形专辑/竖版海报数据，
	// 封面为本地素材；缺省 variant 保持普通友链卡（内联 SVG 头像）。
	r, _, user, _ := newFullEnv(t)
	cookie := authCookie(user)
	w := do2FA(r, http.MethodGet, "/api/v1/preview/friends?variant=albums", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("albums variant: status=%d", w.Code)
	}
	for _, want := range []string{`/assets/img/preview/album-1.jpg`, `"首张专辑"`, `"现场版"`} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("albums variant missing %q: %s", want, w.Body.String())
		}
	}
	w = do2FA(r, http.MethodGet, "/api/v1/preview/friends?variant=posters", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("posters variant: status=%d", w.Code)
	}
	for _, want := range []string{`/assets/img/preview/poster-1.jpg`, `"夏日海报"`, `"春日海报"`} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("posters variant missing %q: %s", want, w.Body.String())
		}
	}
	// 缺省 variant：普通友链卡（data URI 头像）
	w = do2FA(r, http.MethodGet, "/api/v1/preview/friends", "", cookie)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "data:image/svg+xml") {
		t.Errorf("default friends should keep data-uri avatar: status=%d body=%s", w.Code, w.Body.String())
	}
}
