package routes

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"blog/internal/store"
	stellari18n "blog/modules/stellar/i18n"
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
	language := stellarLanguage(snap.input, nil)
	data := s.stellarAggregate(snap, "page", stellarMomentsTitle(language), "/moments", "moments")
	now := time.Now()
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
		data.Moments = append(data.Moments, &tmpl.MomentItem{ID: m.ID, ContentHTML: template.HTML(body), Images: store.DecodeMomentImages(m.Images), TimeAgo: stellarMomentTimeAgo(m.CreatedAt, now, language), Date: m.CreatedAt.Format("2006-01-02 15:04")})
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

// 瞬间标题属于独立 Blog 文案，语言选择与请求快照和官方目录保持一致。
func stellarMomentsTitle(language string) string {
	locale := strings.ToLower(strings.ReplaceAll(language, "_", "-"))
	if strings.HasPrefix(locale, "en") {
		return "Moments"
	}
	switch locale {
	case "zh-tw", "zh-hk", "zh-hant":
		return "瞬間"
	}
	return "瞬间"
}

// 保留既有瞬间的时间边界，后缀使用锁定上游语言目录；同一请求只取一次 now。
func stellarMomentTimeAgo(created, now time.Time, language string) string {
	age := now.Sub(created)
	key, count := "just", 0
	switch {
	case age < time.Minute:
	case age < time.Hour:
		key, count = "min", int(age.Minutes())
	case age < 24*time.Hour:
		key, count = "hour", int(age.Hours())
	case age < 30*24*time.Hour:
		key, count = "day", int(age.Hours()/24)
	default:
		return created.Format("2006-01-02")
	}
	label := stellari18n.Text(language, "meta.date_suffix."+key)
	if key == "just" {
		return label
	}
	return fmt.Sprintf("%d %s", count, label)
}
