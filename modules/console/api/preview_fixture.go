package api

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

// Spec 63：组件库预览 Fixture 端点。
// preview_profile=component-library 时，后端把数据标签的 data-api/src 改写为下列本地端点，
// 只服务预览：不落库、不发外部请求，支持 normal / empty / error 三种确定性状态（默认 normal），
// 供测试与降级验证使用。响应 JSON 形状对齐对应前台 service 脚本的期望字段。

const (
	fixtureStateNormal = "normal"
	fixtureStateEmpty  = "empty"
	fixtureStateError  = "error"
)

// setPreviewFixtureCORS 只为公开、只读、无凭据的预览 Fixture 端点补跨源头。
// 预览 iframe 使用 sandbox="allow-scripts"（opaque origin），其 fetch 与 Console 页面跨源；
// 不加这些头时 friends/sites/rating/vote 的本地 Fixture 数据会被浏览器拦截。
// 这里不放宽 sandbox，也不允许凭据（不使用通配时携带 cookie 的请求需 allow-credentials，未设置）。
func setPreviewFixtureCORS(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Content-Type")
	c.Header("Access-Control-Max-Age", "600")
}

// previewFixtureCORS 给 Fixture 响应补 CORS 头。
func previewFixtureCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		setPreviewFixtureCORS(c)
		c.Next()
	}
}

// previewFixturePreflight 处理 Fixture 端点的 CORS 预检（rating/vote 的 POST 会先 OPTIONS）。
func previewFixturePreflight(c *gin.Context) {
	setPreviewFixtureCORS(c)
	c.AbortWithStatus(http.StatusNoContent)
}

func fixtureState(c *gin.Context) string {
	switch c.Query("state") {
	case fixtureStateEmpty, fixtureStateError:
		return c.Query("state")
	default:
		return fixtureStateNormal
	}
}

// fixtureAvatar 离线可用的内联 SVG 圆形头像 data URI（首字母白字）。
func fixtureAvatar(letter string) string {
	return "data:image/svg+xml;charset=utf-8," + url.PathEscape(
		"<svg xmlns='http://www.w3.org/2000/svg' width='96' height='96'><rect width='96' height='96' rx='48' fill='#4a6cf7'/><text x='48' y='62' font-size='40' text-anchor='middle' fill='white'>"+letter+"</text></svg>")
}

// ---------- rating ----------

// previewRatingFixture GET /api/v1/preview/rating?id=xxx：rating.js 实际契约（原生评分形状 {avg, count}）。
func (h *Handler) previewRatingFixture(c *gin.Context) {
	switch fixtureState(c) {
	case fixtureStateError:
		Fail(c, http.StatusInternalServerError, "fixture error")
	case fixtureStateEmpty:
		c.JSON(http.StatusOK, gin.H{"avg": 0, "count": 0})
	default:
		c.JSON(http.StatusOK, gin.H{"avg": 4.5, "count": 130})
	}
}

// previewRatingFixtureSubmit POST /api/v1/preview/rating：评分提交，接受动作但不落库。
func (h *Handler) previewRatingFixtureSubmit(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"avg": 4.5, "count": 130})
}

// previewRatingInfoFixture GET /api/v1/preview/rating/info?id=xxx：旧版 star_vote 摘要端点，保留兼容。
// 形状为 {rating: {"5": count, ...}}，不写库；新的 rating.js 不再使用该端点。
func (h *Handler) previewRatingInfoFixture(c *gin.Context) {
	switch fixtureState(c) {
	case fixtureStateError:
		Fail(c, http.StatusInternalServerError, "fixture error")
	case fixtureStateEmpty:
		c.JSON(http.StatusOK, gin.H{"rating": gin.H{}})
	default:
		c.JSON(http.StatusOK, gin.H{"rating": gin.H{"5": 86, "4": 30, "3": 8, "2": 4, "1": 2}})
	}
}

// previewRatingUpdateFixture POST /api/v1/preview/rating/update?id=xxx&value=n：rating.js 实际契约，不落库。
func (h *Handler) previewRatingUpdateFixture(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- vote ----------

// previewVoteFixtureInfo GET /api/v1/preview/vote/info?id=xxx：返回确定性投票计数，不写库。
func (h *Handler) previewVoteFixtureInfo(c *gin.Context) {
	switch fixtureState(c) {
	case fixtureStateError:
		Fail(c, http.StatusInternalServerError, "fixture error")
	case fixtureStateEmpty:
		c.JSON(http.StatusOK, gin.H{"votes": gin.H{"up": 0, "down": 0}})
	default:
		c.JSON(http.StatusOK, gin.H{"votes": gin.H{"up": 86, "down": 12}})
	}
}

// previewVoteFixtureUpdate POST /api/v1/preview/vote/update?id=xxx&value=up：接受投票动作但不落库。
func (h *Handler) previewVoteFixtureUpdate(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- friends / sites / md ----------

// previewFriendsFixture GET /api/v1/preview/friends：确定性友链卡片数据
// （形状对齐 friends.js 期望：{content:[{title,url,html_url,avatar_url,labels}]}）。
// Spec 64：新增内部预览参数 variant=default|albums|posters（仍需登录，仅供组件库使用）：
// - default：普通友链卡（内联 SVG 头像）；
// - albums：方形专辑封面卡（本地 /assets/img/preview/album-*.jpg）；
// - posters：竖版海报卡（本地 /assets/img/preview/poster-*.jpg）。
func (h *Handler) previewFriendsFixture(c *gin.Context) {
	switch fixtureState(c) {
	case fixtureStateError:
		Fail(c, http.StatusInternalServerError, "fixture error")
	case fixtureStateEmpty:
		c.JSON(http.StatusOK, gin.H{"content": []any{}})
	default:
		switch c.Query("variant") {
		case "albums":
			c.JSON(http.StatusOK, gin.H{"content": []gin.H{
				{"title": "首张专辑", "url": "https://example.com", "html_url": "https://example.com", "avatar_url": "/assets/img/preview/album-1.jpg", "labels": []gin.H{}},
				{"title": "第二专辑", "url": "https://example.com", "html_url": "https://example.com", "avatar_url": "/assets/img/preview/album-2.jpg", "labels": []gin.H{}},
				{"title": "精选集", "url": "https://example.com", "html_url": "https://example.com", "avatar_url": "/assets/img/preview/album-3.jpg", "labels": []gin.H{}},
				{"title": "现场版", "url": "https://example.com", "html_url": "https://example.com", "avatar_url": "/assets/img/preview/album-4.jpg", "labels": []gin.H{}},
			}})
		case "posters":
			c.JSON(http.StatusOK, gin.H{"content": []gin.H{
				{"title": "夏日海报", "url": "https://example.com", "html_url": "https://example.com", "avatar_url": "/assets/img/preview/poster-1.jpg", "labels": []gin.H{}},
				{"title": "秋日海报", "url": "https://example.com", "html_url": "https://example.com", "avatar_url": "/assets/img/preview/poster-2.jpg", "labels": []gin.H{}},
				{"title": "冬日海报", "url": "https://example.com", "html_url": "https://example.com", "avatar_url": "/assets/img/preview/poster-3.jpg", "labels": []gin.H{}},
				{"title": "春日海报", "url": "https://example.com", "html_url": "https://example.com", "avatar_url": "/assets/img/preview/poster-4.jpg", "labels": []gin.H{}},
			}})
		default:
			c.JSON(http.StatusOK, gin.H{"content": []gin.H{
				{"title": "Fixture 博客", "url": "https://example.com", "html_url": "https://example.com", "avatar_url": fixtureAvatar("B"), "labels": []gin.H{}},
				{"title": "Fixture Docs", "url": "https://docs.example.com", "html_url": "https://docs.example.com", "avatar_url": fixtureAvatar("D"), "labels": []gin.H{}},
				{"title": "Alice 的小站", "url": "https://alice.example.com", "html_url": "https://alice.example.com", "avatar_url": fixtureAvatar("A"), "labels": []gin.H{}},
				{"title": "Bob 实验室", "url": "https://bob.example.com", "html_url": "https://bob.example.com", "avatar_url": fixtureAvatar("B"), "labels": []gin.H{}},
			}})
		}
	}
}

// previewSitesFixture GET /api/v1/preview/sites：确定性站点卡片数据
// （形状对齐 sites.js 期望：{content:[{title,url,icon,cover,description,labels}]}）。
func (h *Handler) previewSitesFixture(c *gin.Context) {
	switch fixtureState(c) {
	case fixtureStateError:
		Fail(c, http.StatusInternalServerError, "fixture error")
	case fixtureStateEmpty:
		c.JSON(http.StatusOK, gin.H{"content": []any{}})
	default:
		c.JSON(http.StatusOK, gin.H{"content": []gin.H{
			{"title": "Fixture Search", "url": "https://search.example.com", "icon": fixtureAvatar("S"), "cover": "/assets/img/preview/banner-bg.jpg", "description": "确定性示例搜索引擎", "labels": []gin.H{{"name": "工具", "color": "4a6cf7", "lightness": 40, "hue": 228, "saturation": 90}}},
			{"title": "Fixture Docs", "url": "https://docs.example.com", "icon": fixtureAvatar("D"), "cover": "/assets/img/preview/gallery-1.jpg", "description": "示例文档中心", "labels": []gin.H{{"name": "文档", "color": "10b981", "lightness": 40, "hue": 152, "saturation": 88}}},
			{"title": "Fixture News", "url": "https://news.example.com", "icon": fixtureAvatar("N"), "cover": "/assets/img/preview/gallery-2.jpg", "description": "示例资讯站", "labels": []gin.H{{"name": "资讯", "color": "f59e0b", "lightness": 48, "hue": 38, "saturation": 92}}},
		}})
	}
}

// previewMDFixture GET /api/v1/preview/md?src=xxx：返回确定性 Markdown 文本
// （mdrender.js 拉取后经 marked 渲染；src 仅作来源标识，返回值固定）。
func (h *Handler) previewMDFixture(c *gin.Context) {
	switch fixtureState(c) {
	case fixtureStateError:
		Fail(c, http.StatusInternalServerError, "fixture error")
	case fixtureStateEmpty:
		c.String(http.StatusOK, "")
	default:
		c.String(http.StatusOK, "# 外部 Markdown Fixture\n\n这是一段**确定性 Fixture** 内容，用于组件库预览：\n\n- 不依赖公网接口；\n- 结构与真实外部 Markdown 一致；\n- 渲染失败时显示降级提示。\n\n```go\nfunc fixture() string { return \"preview\" }\n```\n")
	}
}
