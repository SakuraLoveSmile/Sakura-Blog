package routes

import (
	"fmt"
	"html/template"
	"net/http"

	"blog/internal/comments"
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
	"github.com/gin-gonic/gin"
)

// 瞬间是独立Blog扩展，不添加上游profile或PageModel字段。
func (s *webServer) stellarMomentsView(snap *stellarSnapshot, page int) (*tmpl.ViewData, error) {
	total := (len(snap.inventory.moments) + pageSize - 1) / pageSize
	if total == 0 {
		total = 1
	}
	if page < 1 || page > total {
		return nil, errStellarNotFound
	}
	data := s.stellarAggregate(snap, "page", "瞬间", "/moments", "moments")
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > len(snap.inventory.moments) {
		end = len(snap.inventory.moments)
	}
	for _, m := range snap.inventory.moments[start:end] {
		body, err := s.stellarBody(snap, &stellarRecord{markdown: m.ContentMD, config: map[string]any{}})
		if err != nil {
			return nil, fmt.Errorf("render public moment %d: %w", m.ID, err)
		}
		data.Moments = append(data.Moments, &tmpl.MomentItem{ID: m.ID, ContentHTML: template.HTML(body), Images: store.DecodeMomentImages(m.Images), TimeAgo: comments.TimeAgo(m.CreatedAt), Date: m.CreatedAt.Format("2006-01-02 15:04")})
	}
	numbers := make([]int, total)
	for i := range numbers {
		numbers[i] = i + 1
	}
	data.Paginator = &tmpl.Paginator{Current: page, Total: total, Pages: numbers}
	return data, nil
}
func (s *webServer) serveStellarMoments(c *gin.Context, page int) {
	snap, err := s.stellarSnapshot(nil)
	if err != nil {
		stellarError(c, err)
		return
	}
	if raw := c.Query("page"); raw != "" {
		page, err = stellarPageNumber(raw)
		if err != nil {
			s.render404(c)
			return
		}
	}
	data, err := s.stellarMomentsView(snap, page)
	if err != nil {
		s.render404(c)
		return
	}
	s.outputStellar(c, data, http.StatusOK)
}
