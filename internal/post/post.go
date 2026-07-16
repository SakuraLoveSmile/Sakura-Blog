package post

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
)

// Post 文章渲染结果。
type Post struct {
	Slug    string
	Title   string
	HTML    string // goldmark 渲染后的正文 HTML
	Excerpt string
	Date    string
}

// Service 文章服务，从 markdown 目录加载并渲染文章。
type Service struct {
	mdDir  string
	md     goldmark.Markdown
	parser parser
}

// NewService 创建文章服务。mdDir 是 markdown 文件根目录（如 content/posts）。
func NewService(mdDir string) *Service {
	return &Service{
		mdDir:  mdDir,
		md:     goldmark.New(),
		parser: newParser(),
	}
}

// Load 读取并渲染指定 slug 的文章。文件路径：mdDir/slug.md。
func (s *Service) Load(slug string) (*Post, error) {
	if strings.ContainsAny(slug, "/\\..") {
		return nil, fmt.Errorf("invalid slug: %s", slug)
	}
	path := filepath.Join(s.mdDir, slug+".md")
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read post %s: %w", slug, err)
	}

	meta, body := s.parser.Parse(src)

	var sb strings.Builder
	if err := s.md.Convert([]byte(body), &sb); err != nil {
		return nil, fmt.Errorf("render markdown %s: %w", slug, err)
	}

	title := meta.Title
	if title == "" {
		title = slug
	}
	return &Post{
		Slug:    slug,
		Title:   title,
		HTML:    sb.String(),
		Excerpt: meta.Excerpt,
		Date:    meta.Date,
	}, nil
}
