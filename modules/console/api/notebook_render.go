package api

import (
	"blog/internal/content"
	"blog/internal/store"

	"github.com/gin-gonic/gin"
)

func (h *Handler) renderNotebookNote(c *gin.Context, note *store.Note) bool {
	// prepareNotebookNote 已校验归属；正文 callback 仍须接收原始上游 schema。
	page, err := decodeStellarJSONObject(note.StellarConfig)
	if err != nil {
		failEditorStellar(c, err)
		return false
	}
	rendered, err := h.renderStellarBody(content.StellarBodyInput{Markdown: note.ContentMD, Page: page})
	if err != nil {
		failEditorStellar(c, err)
		return false
	}
	note.ContentHTML = rendered
	return true
}
