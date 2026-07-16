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

// FileMeta 从 Markdown frontmatter 解析出的元数据，供 seed 导入跨包使用。
type FileMeta struct {
	Title   string
	Date    string
	Excerpt string
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

// Render 把 Markdown 正文渲染成 HTML。抽成公共函数，供文件加载与 admin 保存/seed 复用同一 goldmark 实例。
func (s *Service) Render(body string) (string, error) {
	var sb strings.Builder
	if err := s.md.Convert([]byte(body), &sb); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// ParseFile 读 md 文件并分离 frontmatter 与正文，供 seed 导入复用。
// 返回 (meta, body, error)。
func (s *Service) ParseFile(slug string) (FileMeta, string, error) {
	if strings.ContainsAny(slug, "/\\..") {
		return FileMeta{}, "", fmt.Errorf("invalid slug: %s", slug)
	}
	path := filepath.Join(s.mdDir, slug+".md")
	src, err := os.ReadFile(path)
	if err != nil {
		return FileMeta{}, "", fmt.Errorf("read post %s: %w", slug, err)
	}
	meta, body := s.parser.Parse(src)
	return FileMeta{Title: meta.Title, Date: meta.Date, Excerpt: meta.Excerpt}, body, nil
}

// Load 读取并渲染指定 slug 的文章。文件路径：mdDir/slug.md。
func (s *Service) Load(slug string) (*Post, error) {
	meta, body, err := s.ParseFile(slug)
	if err != nil {
		return nil, err
	}

	html, err := s.Render(body)
	if err != nil {
		return nil, fmt.Errorf("render markdown %s: %w", slug, err)
	}

	title := meta.Title
	if title == "" {
		title = slug
	}
	return &Post{
		Slug:    slug,
		Title:   title,
		HTML:    html,
		Excerpt: meta.Excerpt,
		Date:    meta.Date,
	}, nil
}

// ListSlugs 扫描 mdDir 下所有 *.md 文件，返回 slug 列表（去扩展名）。供 seed 批量导入用。
func (s *Service) ListSlugs() ([]string, error) {
	entries, err := os.ReadDir(s.mdDir)
	if err != nil {
		return nil, fmt.Errorf("read posts dir %s: %w", s.mdDir, err)
	}
	var slugs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		slugs = append(slugs, strings.TrimSuffix(name, ".md"))
	}
	return slugs, nil
}
