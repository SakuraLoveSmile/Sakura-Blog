package routes

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// registerHomeRoutes 注册首页域路由：/ 与 /page/:num。
func (s *webServer) registerHomeRoutes(r *gin.Engine) {
	// 首页：文章列表（对齐 Stellar layout/index.ejs）
	r.GET("/", func(c *gin.Context) {
		s.renderHome(c, 1)
	})
	// 首页分页：/page/N，非数字或 <1 返回 404
	r.GET("/page/:num", func(c *gin.Context) {
		n, err := strconv.Atoi(c.Param("num"))
		if err != nil || n < 1 {
			s.render404(c)
			return
		}
		s.renderHome(c, n)
	})
}

// renderHome 首页渲染：/ 与 /page/:num 共用。page 从 1 开始，超出末页回退到最后一页。
func (s *webServer) renderHome(c *gin.Context, page int) { s.serveStellarHome(c, page, "home") }
