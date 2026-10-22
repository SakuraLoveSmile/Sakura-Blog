package routes

import (
	"github.com/gin-gonic/gin"
)

// handleLinksPage 友链卡片页（内容页化）：渲染 pages 表 slug=links 系统页。
// 正文 {% links %} 服务端直出分组友链卡片；banner/TOC 同内容页。
func (s *webServer) handleLinksPage(c *gin.Context) {
	p, err := s.st.GetPublished(LinksSlug)
	if err != nil {
		stellarError(c, err)
		return
	}
	s.serveStellarPage(c, p)
}

// handleFriendsPage 友链页（内容页化）：渲染 pages 表 slug=friends 系统页。
// 正文 md 控制台可编辑（{% friends posts:true %} 嵌入友链组件）；
// banner 标题/封面/副标题取 pages.friends 设置（缺省回退页面标题）；TOC 与评论同内容页。
func (s *webServer) handleFriendsPage(c *gin.Context) {
	p, err := s.st.GetPublished(FriendsSlug)
	if err != nil {
		stellarError(c, err)
		return
	}
	s.serveStellarPage(c, p)
}
