package routes

import (
	"blog/internal/store"
	"blog/internal/textutil"
	"html/template"
	"log"
	"strconv"
	"strings"

	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"

	"github.com/gin-gonic/gin"
)

// handleNotebooks 笔记本列表页（Spec 35b）：卡片网格（封面/名称/描述 + 笔记数），
// 对齐 wiki.html 的 post-list wiki 网格样式。
func (s *webServer) handleNotebooks(c *gin.Context) {
	notebooks, err := s.st.ListNotebooks()
	if err != nil {
		log.Printf("list notebooks: %v", err)
		s.render500(c)
		return
	}
	// Spec 62：笔记数从「每笔记本一次 COUNT」收敛为单条 GROUP BY 查询（CountNotesByNotebooks）。
	ids := make([]int64, 0, len(notebooks))
	for _, nb := range notebooks {
		ids = append(ids, nb.ID)
	}
	counts, err := s.st.CountNotesByNotebooks(ids)
	if err != nil {
		log.Printf("count notes of notebooks: %v", err)
		s.render500(c)
		return
	}
	views := make([]tmpl.NotebookView, 0, len(notebooks))
	for _, nb := range notebooks {
		views = append(views, tmpl.NotebookView{Notebook: *nb, NoteCount: counts[nb.ID]})
	}
	s.renderBase(c, &tmpl.ViewData{
		Site:      s.site.Load(),
		Page:      &tmpl.Page{Title: stellarweb.Lang("notebook.title"), Layout: "notebooks", Lang: "zh-CN"},
		Notebooks: views,
	})
}

// handleNotebook 笔记本详情页（Spec 35b）：笔记列表（updated_at DESC）+ 分页 + ?tag= 过滤。
// leftbar tagtree widget 数据由 TagTreeData 注入（见 assembleWidgets 的 case "tagtree"）。
func (s *webServer) handleNotebook(c *gin.Context) {
	name := c.Param("name")
	nb, err := s.st.GetNotebookByName(name)
	if err != nil {
		log.Printf("get notebook %q: %v", name, err)
		s.render500(c)
		return
	}
	if nb == nil {
		s.render404(c)
		return
	}
	// 标签聚合（tagtree 数据源）；未知 tag 过滤参数与 wiki 标签页一致走 404
	tag := c.Query("tag")
	tagCounts, err := s.st.CountNoteTagsByNotebook(nb.ID)
	if err != nil {
		log.Printf("count note tags of notebook %d: %v", nb.ID, err)
		s.render500(c)
		return
	}
	tags := tagCountItems(tagCounts)
	tagTree := s.tagTreeData(tags, nb.Name, tag)
	if tag != "" && !tagTree.HasTag(tag) {
		s.render404(c)
		return
	}
	// 分页参数
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	const perPage = 20
	var notes []*store.Note
	var total int
	if tag != "" {
		// 层级标签需要把父节点及所有后代一并纳入；先取同笔记本全集，
		// 再用 TagMatches 过滤，保持旧的精确标签也完全兼容。
		allNotes, _, listErr := s.st.ListNotesByNotebook(nb.ID, -1, 0)
		if listErr != nil {
			err = listErr
		} else {
			for _, note := range allNotes {
				for _, noteTag := range strings.Split(note.Tags, ",") {
					if tmpl.TagMatches(noteTag, tag) {
						notes = append(notes, note)
						break
					}
				}
			}
		}
		if err != nil {
			log.Printf("list notes of notebook %d: %v", nb.ID, err)
			s.render500(c)
			return
		}
		total = len(notes)
	} else {
		// 先取总数（LIMIT 0 仅 COUNT）再 clamp 页，对齐 handleAuthor 语义
		_, total, err = s.st.ListNotesByNotebook(nb.ID, 0, 1)
		if err != nil {
			log.Printf("count notes of notebook %d: %v", nb.ID, err)
			s.render500(c)
			return
		}
	}
	totalPages := (total + perPage - 1) / perPage
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}
	if tag != "" {
		// 内存分页切片
		start := (page - 1) * perPage
		if start > len(notes) {
			start = len(notes)
		}
		end := start + perPage
		if end > len(notes) {
			end = len(notes)
		}
		notes = notes[start:end]
	} else {
		notes, _, err = s.st.ListNotesByNotebook(nb.ID, perPage, (page-1)*perPage)
		if err != nil {
			log.Printf("list notes of notebook %d: %v", nb.ID, err)
			s.render500(c)
			return
		}
	}
	pages := make([]int, totalPages)
	for i := range pages {
		pages[i] = i + 1
	}
	s.renderBase(c, &tmpl.ViewData{
		Site:           s.site.Load(),
		Page:           &tmpl.Page{Title: nb.Name, Layout: "notebook", Lang: "zh-CN", Tag: tag},
		NotebookDetail: nb,
		Notes:          noteValues(notes, s.site.Load().Notebook.AutoExcerpt),
		NotesTotal:     total,
		NotesPage:      page,
		NotesPerPage:   perPage,
		Paginator:      &tmpl.Paginator{Current: page, Total: totalPages, Pages: pages},
		NotebookTags:   tags,
		TagTreeData:    tagTree,
	})
}

// handleNote 笔记详情页（Spec 35b）：面包屑（首页/笔记本/{name}/{title}）+ 正文 + 上下篇。
// 上下篇为同笔记本内按 updated_at DESC 相邻的笔记；正文优先已渲染 ContentHTML，为空时渲染 Markdown。
func (s *webServer) handleNote(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		s.render404(c)
		return
	}
	note, err := s.st.GetNote(id)
	if err != nil {
		log.Printf("get note %d: %v", id, err)
		s.render500(c)
		return
	}
	if note == nil {
		s.render404(c)
		return
	}
	nb, err := s.st.GetNotebook(note.NotebookID)
	if err != nil {
		log.Printf("get notebook %d: %v", note.NotebookID, err)
		s.render500(c)
		return
	}
	if nb == nil {
		s.render404(c)
		return
	}
	// 正文：复用缓存 HTML；含 {% links %} 时按当前站点友链配置重新渲染。
	renderedContent, renderErr := renderStoredContent(s.posts, note.Content, note.ContentHTML)
	if renderErr != nil {
		log.Printf("render note %d: %v", id, renderErr)
		s.render500(c)
		return
	}
	contentHTML := template.HTML(renderedContent)
	// 上下篇：同笔记本内按 updated_at DESC 相邻（Spec 62：两条 LIMIT 1 邻居查询
	// GetPrevNote/GetNextNote，替代全量取回内存定位）
	var prev, next *tmpl.NoteNav
	if pn, err := s.st.GetPrevNote(nb.ID, note.ID); err == nil && pn != nil {
		prev = &tmpl.NoteNav{ID: pn.ID, Title: pn.Title}
	} else if err != nil {
		log.Printf("prev note of %d: %v", note.ID, err)
	}
	if nn, err := s.st.GetNextNote(nb.ID, note.ID); err == nil && nn != nil {
		next = &tmpl.NoteNav{ID: nn.ID, Title: nn.Title}
	} else if err != nil {
		log.Printf("next note of %d: %v", note.ID, err)
	}
	// tagtree 数据源：所属笔记本标签聚合
	tagCounts, err := s.st.CountNoteTagsByNotebook(nb.ID)
	if err != nil {
		log.Printf("count note tags of notebook %d: %v", nb.ID, err)
		s.render500(c)
		return
	}
	tags := tagCountItems(tagCounts)
	s.renderBase(c, &tmpl.ViewData{
		Site: s.site.Load(),
		Page: &tmpl.Page{
			Title:       note.Title,
			Layout:      "note",
			Lang:        "zh-CN",
			Description: textutil.Truncate(textutil.StripHTML(string(contentHTML)), 150),
		},
		NoteDetail: &tmpl.NoteView{
			Note:          *note,
			ContentHTML:   contentHTML,
			NotebookName:  nb.Name,
			NotebookTitle: nb.Name,
			PrevNote:      prev,
			NextNote:      next,
		},
		NotebookTags: tags,
		TagTreeData:  s.tagTreeData(tags, nb.Name, ""),
	})
}

func (s *webServer) tagTreeData(tags []tmpl.TagCount, notebookName, active string) *tmpl.TagTreeWidgetData {
	def := s.site.Load().Widgets.Lookup("tagtree")
	data := tmpl.BuildTagTree(tags, notebookName, active,
		def.Bool("expand_all", false),
		def.Bool("expand_active", true),
		def.Bool("show_tagcon", true))
	tmpl.ApplyTagcons(data, s.site.Load().Notebook.Tagcons)
	return data
}

// noteValues store.Note 指针列表 → 值列表（ViewData.Notes 用，模板直接 range）。
func noteValues(notes []*store.Note, autoExcerpt int) []store.Note {
	out := make([]store.Note, 0, len(notes))
	for _, n := range notes {
		v := *n
		if autoExcerpt > 0 {
			content := v.ContentHTML
			if content == "" {
				content = v.Content
			}
			v.Excerpt = textutil.Truncate(textutil.StripHTML(content), autoExcerpt)
		}
		out = append(out, v)
	}
	return out
}
