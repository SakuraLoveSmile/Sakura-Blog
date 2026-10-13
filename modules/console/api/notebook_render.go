package api

import (
	"blog/internal/content"
	"blog/internal/store"

	"github.com/gin-gonic/gin"
)

func (h *Handler) renderNotebookNote(c *gin.Context, note *store.Note, page map[string]any) bool {
	rendered, err := h.renderStellarBody(content.StellarBodyInput{Markdown: note.ContentMD, Page: page})
	if err != nil {
		failEditorStellar(c, err)
		return false
	}
	note.ContentHTML = rendered
	return true
}
