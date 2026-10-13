package routes

import (
	"context"

	"blog/internal/content"
	"blog/internal/post"
	"blog/internal/stellarv2"
)

func (s *Server) RenderStellarMarkdown(input content.StellarBodyInput) (string, error) {
	if s.web == nil {
		return "", content.ErrStellarRendererUnavailable
	}
	return s.web.renderStellarMarkdown(input)
}

func (s *webServer) renderStellarMarkdown(input content.StellarBodyInput) (string, error) {
	if s.posts == nil {
		return "", content.ErrStellarRendererUnavailable
	}
	parsed, err := stellarv2.ParsePage(input.Page, stellarv2.Options{Source: "editor.stellar_config"})
	if err != nil {
		return "", err
	}
	snap, err := s.stellarPublicContextAt(context.Background(), s.site.Load())
	if err != nil {
		return "", err
	}
	language := input.Language
	if language == "" {
		language = stellarLanguage(snap.input, parsed.Value)
	}
	return s.posts.RenderStellarV2WithContext(input.Markdown, post.StellarRenderContext{Theme: snap.input.Theme, Data: snap.input.RuntimeData, Page: parsed.Value, Language: language, BlogExtensions: true})
}
