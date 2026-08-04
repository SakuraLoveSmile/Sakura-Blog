package api

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/config"
	"blog/internal/store"
)

// linkJSON 友链 API 响应/请求模型。
type linkJSON struct {
	ID          int64  `json:"id"`
	GroupTitle  string `json:"group_title"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Avatar      string `json:"avatar"`
	Description string `json:"description"`
	RSS         string `json:"rss"`
	Label       string `json:"label"`
	Color       string `json:"color"`
	Sort        int    `json:"sort"`
}

func toLinkJSON(l *store.Link) linkJSON {
	return linkJSON{
		ID: l.ID, GroupTitle: l.GroupTitle, Name: l.Name, URL: l.URL,
		Avatar: l.Avatar, Description: l.Description, RSS: l.RSS,
		Label: l.Label, Color: l.Color, Sort: l.Sort,
	}
}

func (r *linkJSON) toStore() *store.Link {
	return &store.Link{
		GroupTitle:  strings.TrimSpace(r.GroupTitle),
		Name:        strings.TrimSpace(r.Name),
		URL:         strings.TrimSpace(r.URL),
		Avatar:      strings.TrimSpace(r.Avatar),
		Description: strings.TrimSpace(r.Description),
		RSS:         strings.TrimSpace(r.RSS),
		Label:       strings.TrimSpace(r.Label),
		Color:       strings.TrimSpace(r.Color),
		Sort:        r.Sort,
	}
}

// listLinks GET /api/v1/links：友链扁平列表（含 group_title，控制台按组渲染）。
func (h *Handler) listLinks(c *gin.Context) {
	links, err := h.store.ListLinks()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]linkJSON, 0, len(links))
	for _, l := range links {
		items = append(items, toLinkJSON(l))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// createLink POST /api/v1/links：新增友链，成功后热更新站点。
func (h *Handler) createLink(c *gin.Context) {
	var req linkJSON
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Name == "" || req.URL == "" {
		Fail(c, http.StatusBadRequest, "名称与链接不能为空")
		return
	}
	l := req.toStore()
	if _, err := h.store.CreateLink(l); err != nil {
		Fail(c, http.StatusInternalServerError, "创建失败: " + err.Error())
		return
	}
	h.afterLinksChanged()
	c.JSON(http.StatusCreated, toLinkJSON(l))
}

// updateLink PUT /api/v1/links/:id：更新友链，成功后热更新站点。
func (h *Handler) updateLink(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "非法 id")
		return
	}
	existing, err := h.store.GetLink(id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		Fail(c, http.StatusNotFound, "友链不存在")
		return
	}
	var req linkJSON
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Name == "" || req.URL == "" {
		Fail(c, http.StatusBadRequest, "名称与链接不能为空")
		return
	}
	l := req.toStore()
	if err := h.store.UpdateLink(id, l); err != nil {
		Fail(c, http.StatusInternalServerError, "更新失败: " + err.Error())
		return
	}
	l.ID = id
	h.afterLinksChanged()
	c.JSON(http.StatusOK, toLinkJSON(l))
}

// deleteLink DELETE /api/v1/links/:id：删除友链，成功后热更新站点。
func (h *Handler) deleteLink(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "非法 id")
		return
	}
	if err := h.store.DeleteLink(id); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.afterLinksChanged()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// afterLinksChanged 友链增删改后：以 DB 为准重建站点，并触发聚合器即时刷新（若已注册）。
func (h *Handler) afterLinksChanged() {
	_ = config.RebuildSite(h.store, h.site)
	if h.onLinksChanged != nil {
		h.onLinksChanged()
	}
}

// linkCheckHTTPClient 连通性检测专用 client（短超时，不跟随过多重定向）。
var linkCheckHTTPClient = &http.Client{Timeout: 8 * time.Second}

// checkLinks POST /api/v1/links/check：并发检测各友链 URL 可达性，返回每条 {id, ok, status}。
// status：HTTP 状态码；网络错误/超时为 0。
func (h *Handler) checkLinks(c *gin.Context) {
	links, err := h.store.ListLinks()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	type result struct {
		ID     int64 `json:"id"`
		OK     bool  `json:"ok"`
		Status int   `json:"status"`
	}
	results := make([]result, len(links))
	var wg sync.WaitGroup
	for i, l := range links {
		if l.URL == "" {
			results[i] = result{ID: l.ID, OK: false}
			continue
		}
		wg.Add(1)
		go func(idx int, id int64, url string) {
			defer wg.Done()
			ok, status := probeURL(url)
			results[idx] = result{ID: id, OK: ok, Status: status}
		}(i, l.ID, l.URL)
	}
	wg.Wait()
	c.JSON(http.StatusOK, gin.H{"items": results})
}

// probeURL HEAD 探测（失败回退 GET），返回 (可达, HTTP 状态码)；2xx/3xx 视为可达，网络错误状态码为 0。
func probeURL(url string) (bool, int) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return false, 0
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; StellarBlog/1.0)")
	resp, err := linkCheckHTTPClient.Do(req)
	if err != nil {
		// 部分站点不支持 HEAD，回退 GET
		req2, e2 := http.NewRequest(http.MethodGet, url, nil)
		if e2 != nil {
			return false, 0
		}
		req2.Header.Set("User-Agent", "Mozilla/5.0 (compatible; StellarBlog/1.0)")
		resp, err = linkCheckHTTPClient.Do(req2)
		if err != nil {
			return false, 0
		}
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400, resp.StatusCode
}
