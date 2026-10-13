package routes

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
)

// 权限在全站严格解析前判定，避免另一个未迁移记录改变私有自定义路径的404。
func (s *webServer) preflightStellarCustomPath(c *gin.Context, path string) bool {
	rows, err := s.st.DB().QueryContext(c.Request.Context(), `SELECT 'wiki',id,visible,stellar_config FROM wiki_projects UNION ALL SELECT 'notebook',id,1,stellar_config FROM notebooks`)
	if err != nil {
		return false
	}
	type collection struct {
		profile, id, raw string
		visible          bool
	}
	candidates := []collection{}
	for rows.Next() {
		var item collection
		if err := rows.Scan(&item.profile, &item.id, &item.visible, &item.raw); err != nil {
			rows.Close()
			stellarError(c, err)
			return true
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		stellarError(c, err)
		return true
	}
	rows.Close()
	for _, item := range candidates {
		var raw map[string]any
		if err := json.Unmarshal([]byte(item.raw), &raw); err != nil {
			continue
		}
		base := stellarPathKey(stellarString(stellarGet(raw, "route.path")))
		if base == "" {
			continue
		}
		if path != base && !strings.HasPrefix(path, base+"/") {
			continue
		}
		if !item.visible {
			s.render404(c)
			return true
		}
		if path == base {
			return false
		}
		slug := strings.TrimPrefix(path, base+"/")
		if item.profile == "notebook" && (strings.HasPrefix(slug, "tags/") || strings.HasPrefix(slug, "page/")) {
			return false
		}
		if item.profile == "wiki" {
			doc, err := s.st.GetWikidocBySlug(item.id, slug)
			if err != nil {
				stellarError(c, err)
				return true
			}
			if doc == nil || doc.Status != "published" {
				s.render404(c)
				return true
			}
		} else {
			note, err := s.st.GetPublishedNote(item.id, slug)
			if err != nil {
				stellarError(c, err)
				return true
			}
			if note == nil {
				s.render404(c)
				return true
			}
		}
		return false
	}
	return false
}
