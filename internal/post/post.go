package post

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	mdparser "github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// Post 文章渲染结果。
type Post struct {
	Slug       string
	Title      string
	HTML       string // goldmark 渲染后的正文 HTML
	Excerpt    string
	Date       string
	References []string // 参考文献列表（frontmatter references 字段）
}

// FileMeta 从 Markdown frontmatter 解析出的元数据，供 seed 导入跨包使用。
type FileMeta struct {
	Title    string
	Date     string
	Excerpt  string
	Cover    string
	Category string
	Topic    string // 专栏名
	Tags     []string
	// 海报封面（poster_headline 等平铺 key，对齐原版 post.poster）
	PosterHeadline string
	PosterTopic    string
	PosterCaption  string
	PosterColor    string
	// 参考文献（对齐原版 article_footer.ejs page.references）
	References []string
	// 备份导出/导入扩展字段（seed 源文件一般不写，缺失时保持零值）
	Status string // draft / published，空=由导入方决定
	Pinned bool   // 置顶
}

// Service 文章服务，从 markdown 目录加载并渲染文章。
type Service struct {
	mdDir    string
	md       goldmark.Markdown
	parser   parser
	iconFunc func(string) string // 渲染内联 SVG（供 quot 等 shortcode 用），由 main.go 用 IconRegistry 装配
}

// SetIconFunc 注入图标渲染函数（key → 内联 SVG）。未设置时 quot 等图标降级为空占位。
func (s *Service) SetIconFunc(f func(string) string) {
	s.iconFunc = f
}

// NewService 创建文章服务。mdDir 是 markdown 文件根目录（如 content/posts）。
// AutoHeadingID：标题渲染带 id，供 TOC 锚点与 scroll-spy 使用。
// 代码块经 Chroma 服务端高亮（CSS class 模式，class 前缀 chroma，无行号）。
func NewService(mdDir string) *Service {
	return &Service{
		mdDir: mdDir,
		md: goldmark.New(
			goldmark.WithExtensions(
				// GFM 表格与删除线（编辑器工具栏提供对应语法，渲染侧同步支持）
				extension.Table,
				extension.Strikethrough,
				highlighting.NewHighlighting(
					highlighting.WithGuessLanguage(true),
					highlighting.WithFormatOptions(
						chromahtml.WithClasses(true),
						chromahtml.WithLineNumbers(false),
					),
				),
			),
			goldmark.WithParserOptions(mdparser.WithAutoHeadingID()),
			goldmark.WithRendererOptions(html.WithUnsafe()),
		),
		parser: newParser(),
	}
}

// Render 把 Markdown 正文渲染成 HTML。抽成公共函数，供文件加载与 admin 保存/seed 复用同一 goldmark 实例。
// 渲染流程：先预处理 shortcodes（note/folding → HTML），再 goldmark 渲染整篇。
func (s *Service) Render(body string) (string, error) {
	// renderMD 闭包：用同一 goldmark 实例渲染子片段（folding body 等）
	renderMD := func(md string) string {
		md = protectMathDelimiters(md)
		var sb strings.Builder
		if err := s.md.Convert([]byte(md), &sb); err != nil {
			return md
		}
		return sb.String()
	}

	// 1. 预处理 shortcodes（正文与 folding 内嵌 md 均在 goldmark 前保护公式定界符）
	processed := processShortcodes(protectMathDelimiters(body), renderMD, s.iconFunc)

	// 2. goldmark 渲染整篇（HTML 块会被保留）
	var sb strings.Builder
	if err := s.md.Convert([]byte(processed), &sb); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// ParseBytes 解析 Markdown 原文（含可选 frontmatter），返回元数据与正文。
// 供备份导入等内存数据源复用，与 ParseFile 共享同一轻量 parser。
func (s *Service) ParseBytes(src []byte) (FileMeta, string) {
	meta, body := s.parser.Parse(src)
	return FileMeta{
		Title: meta.Title, Date: meta.Date, Excerpt: meta.Excerpt, Cover: meta.Cover, Category: meta.Category, Topic: meta.Topic, Tags: meta.Tags,
		PosterHeadline: meta.PosterHeadline, PosterTopic: meta.PosterTopic, PosterCaption: meta.PosterCaption, PosterColor: meta.PosterColor,
		References: meta.References,
		Status:     meta.Status,
		Pinned:     meta.Pinned,
	}, body
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
	meta, body := s.ParseBytes(src)
	return meta, body, nil
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
		Slug:       slug,
		Title:      title,
		HTML:       html,
		Excerpt:    meta.Excerpt,
		Date:       meta.Date,
		References: meta.References,
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
