// Package moments publishes the Console moments module manifest.
package moments

import (
	"github.com/gin-gonic/gin"

	"blog/internal/module"
)

type Module struct{}

func init() { module.Register(&Module{}) }

func (m *Module) Meta() module.Meta {
	return module.Meta{
		ID:          "moments",
		Title:       "瞬间",
		Description: "发布和管理短内容动态",
		Category:    "内容",
		DefaultOn:   true,
		Nav:         []module.Nav{{Path: "/moments", Label: "瞬间", Icon: "ri:quill-pen-line", Group: "内容"}},
	}
}

func (m *Module) Migrate(deps module.Deps) error               { return deps.Store.EnsureMomentsTable() }
func (m *Module) RegisterRoutes(*gin.RouterGroup, module.Deps) {}
func (m *Module) RegisterPublic(*gin.Engine, module.Deps)      {}
