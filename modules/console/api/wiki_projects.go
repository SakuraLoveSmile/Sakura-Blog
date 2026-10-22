package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

type wikiProjectRequest struct {
	StellarContentMetadata
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Title       string          `json:"title"`
	Headline    string          `json:"headline"`
	Subtitle    string          `json:"subtitle"`
	Description string          `json:"description"`
	Icon        string          `json:"icon"`
	Cover       string          `json:"cover"`
	Available   string          `json:"available"`
	URL         string          `json:"url"`
	Repo        string          `json:"repo"`
	Branch      string          `json:"branch"`
	Sort        int             `json:"sort"`
	Pin         bool            `json:"pin"`
	Visible     *bool           `json:"visible"`
	Tags        []string        `json:"tags"`
	Hero        *store.WikiHero `json:"hero"`
}

func (h *Handler) wikiProjectFromRequest(req wikiProjectRequest, id string, current *store.WikiProject) *store.WikiProject {
	visible := true
	if current != nil {
		visible = current.Visible
	}
	if req.Visible != nil {
		visible = *req.Visible
	}
	if id == "" {
		id = strings.TrimSpace(req.ID)
	}
	if req.StellarVersion == 2 {
		raw, blog := stellarFormObject(req.StellarConfig), stellarFormObject(req.BlogExtensions)
		stellarFormString(raw, "name", &req.Name)
		req.Title = req.Name
		stellarFormString(raw, "headline", &req.Headline)
		stellarFormString(raw, "tagline", &req.Subtitle)
		stellarFormString(raw, "description", &req.Description)
		stellarFormString(raw, "icon", &req.Icon)
		stellarFormString(raw, "cover", &req.Cover)
		stellarFormString(raw, "audience", &req.Available)
		stellarFormString(raw, "source.repository", &req.Repo)
		stellarFormString(raw, "source.branch", &req.Branch)
		stellarFormString(blog, "content.wiki_external_url", &req.URL)
		stellarFormInt(blog, "content.collection_order", &req.Sort)
		if tags, exists := stellarFormStrings(raw, "tags"); exists {
			req.Tags = tags
		}
	}
	return &store.WikiProject{
		StellarVersion: req.StellarVersion, StellarConfig: req.StellarConfig, BlogExtensions: req.BlogExtensions,
		ID: id, Name: strings.TrimSpace(req.Name), Title: strings.TrimSpace(req.Title),
		Headline: strings.TrimSpace(req.Headline), Subtitle: strings.TrimSpace(req.Subtitle),
		Description: strings.TrimSpace(req.Description), Icon: strings.TrimSpace(req.Icon),
		Cover: strings.TrimSpace(req.Cover), Available: strings.TrimSpace(req.Available),
		URL: strings.TrimSpace(req.URL), Repo: strings.TrimSpace(req.Repo), Branch: strings.TrimSpace(req.Branch),
		Sort: req.Sort, Pin: req.Pin, Visible: visible, Tags: req.Tags, Hero: req.Hero,
	}
}

func (h *Handler) listWikiProjects(c *gin.Context) {
	items, err := h.store.ListWikiProjectsWithCounts()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if items == nil {
		items = []*store.WikiProject{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) getWikiProject(c *gin.Context) {
	item, err := h.store.GetWikiProject(c.Param("id"))
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if item == nil {
		Fail(c, http.StatusNotFound, "Wiki 项目不存在")
		return
	}
	if err := h.store.PopulateWikiProjectCounts(item); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) createWikiProject(c *gin.Context) {
	var req wikiProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if _, err := validateEditorMetadata(req.StellarContentMetadata, "wiki", true); err != nil {
		failEditorStellar(c, err)
		return
	}
	p := h.wikiProjectFromRequest(req, "", nil)
	if err := h.store.CreateWikiProject(p); err != nil {
		switch {
		case errors.Is(err, store.ErrWikiProjectInvalid):
			Fail(c, http.StatusBadRequest, "项目 ID 只能包含字母、数字、下划线和短横线")
		case errors.Is(err, store.ErrWikiProjectConflict):
			Fail(c, http.StatusConflict, "项目 ID 已存在")
		default:
			Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}
	h.publishContentChanged()
	c.JSON(http.StatusCreated, p)
}

func (h *Handler) updateWikiProject(c *gin.Context) {
	id := c.Param("id")
	current, err := h.store.GetWikiProject(id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if current == nil {
		Fail(c, http.StatusNotFound, "Wiki 项目不存在")
		return
	}
	var req wikiProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if strings.TrimSpace(req.ID) != "" && strings.TrimSpace(req.ID) != id {
		Fail(c, http.StatusBadRequest, "项目 ID 创建后不可修改")
		return
	}
	if _, err := validateEditorMetadata(req.StellarContentMetadata, "wiki", true); err != nil {
		failEditorStellar(c, err)
		return
	}
	p := h.wikiProjectFromRequest(req, id, current)
	if err := h.store.UpdateWikiProject(id, p); err != nil {
		if errors.Is(err, store.ErrWikiProjectInvalid) {
			Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.publishContentChanged()
	c.JSON(http.StatusOK, p)
}

func (h *Handler) deleteWikiProject(c *gin.Context) {
	if err := h.store.DeleteWikiProject(c.Param("id")); err != nil {
		switch {
		case errors.Is(err, store.ErrWikiProjectNotFound):
			Fail(c, http.StatusNotFound, "Wiki 项目不存在")
		case errors.Is(err, store.ErrWikiProjectConflict):
			Fail(c, http.StatusConflict, "项目仍包含 Wiki 文档，不能删除")
		default:
			Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}
	h.publishContentChanged()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
