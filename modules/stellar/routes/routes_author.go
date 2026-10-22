package routes

import "github.com/gin-gonic/gin"

func (s *webServer) handleAuthor(c *gin.Context) { s.serveStellarAuthor(c, c.Param("id")) }
