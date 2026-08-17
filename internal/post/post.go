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

	"blog/internal/settings"
	tmpl "blog/internal/settings"
)

// Post 文章渲染结果。
type Post struct {
	Slug       string
	Title      string
	HTML       string // goldmark 渲染后的正文 HTML
	Excerpt    string
	Banner     string // 文首横幅图 URL
	Date       string
	Status     string   // draft / published，空=由旧文件回退链路按公开内容兼容
	References []string // 参考文献列表（frontmatter references 字段）
}

// FileMeta 从 Markdown frontmatter 解析出的元数据，供 seed 导入跨包使用。
type FileMeta struct {
	Title    string
	Date     string
	Excerpt  string
	Cover    string
	Banner   string // 文首横幅图 URL
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
	Author string // 作者 ID（Spec 38 多作者体系）
}

// Service 文章服务，从 markdown 目录加载并渲染文章。
type Service struct {
	mdDir    string
	md       goldmark.Markdown
	inlineMD goldmark.Markdown // 仅行内语法渲染器（供标签插件 body 使用）
	parser   parser
	settings *settings.Manager
	iconFunc func(string) string // 渲染内联 SVG（供 quot 等 shortcode 用），由 main.go 用 IconRegistry 装配
	ghClient GHClient            // GitHub 数据客户端（供 ghrepo/ghuser shortcode 用），nil 则降级纯链接
	// linksFunc 返回站点友链分组与缺省头像（default.link），供 {% links %} 服务端直出；nil 时渲染空态。
	linksFunc func() ([]tmpl.LinkGroup, string)
}

// GHClient GitHub 数据接口（由 internal/github.Client 实现，解耦循环依赖）。
type GHClient interface {
	GetRepo(owner, repo string) (*GHRepoInfo, error)
	GetUser(username string) (*GHUserInfo, error)
}

// GHRepoInfo 仓库信息（shortcode 渲染用）。
type GHRepoInfo struct {
	Name        string
	FullName    string
	Description string
	Stars       int
	Forks       int
	Language    string
	License     string
	OwnerAvatar string
	OwnerLogin  string
	HTMLURL     string
}

// GHUserInfo 用户信息（shortcode 渲染用）。
type GHUserInfo struct {
	Login     string
	Name      string
	Bio       string
	AvatarURL string
	Followers int
	Following int
	Repos     int
	HTMLURL   string
}

// SetIconFunc 注入图标渲染函数（key → 内联 SVG）。未设置时 quot 等图标降级为空占位。
func (s *Service) SetIconFunc(f func(string) string) {
	s.iconFunc = f
}

// SetLinksFunc 注入站点友链分组提供器（供 {% links %} shortcode 服务端直出）。
func (s *Service) SetLinksFunc(f func() ([]tmpl.LinkGroup, string)) {
	s.linksFunc = f
}

// SetGHClient 注入 GitHub 数据客户端（供 ghrepo/ghuser shortcode 渲染）。未设置时降级纯链接。
func (s *Service) SetGHClient(c GHClient) {
	s.ghClient = c
}

// SetSettingsManager supplies the current site snapshot to shortcode rendering
// without relying on package-global tag plugin state.
func (s *Service) SetSettingsManager(m *settings.Manager) { s.settings = m }

// NewService 创建文章服务。mdDir 是 markdown 文件根目录（如 content/posts）。
// AutoHeadingID：标题渲染带 id，供 TOC 锚点与 scroll-spy 使用。
// 代码块经 Chroma 服务端高亮（CSS class 模式，class 前缀 chroma，table 行号）。
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
						chromahtml.WithLineNumbers(true),
						chromahtml.LineNumbersInTable(true),
					),
				),
			),
			goldmark.WithParserOptions(mdparser.WithAutoHeadingID()),
			goldmark.WithRendererOptions(html.WithUnsafe()),
		),
		// 仅行内语法渲染器：无扩展，仅基础行内语法（粗体、斜体、链接、行内代码、图片）
		inlineMD: goldmark.New(
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
	var tagPlugins settings.TagPluginsConfig
	if s.settings != nil && s.settings.Current() != nil {
		tagPlugins = s.settings.Current().TagPlugins
	}
	processed := processShortcodes(protectMathDelimiters(body), renderMD, s.renderInlineMD, s.iconFunc, s.ghClient, s.renderLinks, tagPlugins)

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
		Title: meta.Title, Date: meta.Date, Excerpt: meta.Excerpt, Cover: meta.Cover, Banner: meta.Banner, Category: meta.Category, Topic: meta.Topic, Tags: meta.Tags,
		PosterHeadline: meta.PosterHeadline, PosterTopic: meta.PosterTopic, PosterCaption: meta.PosterCaption, PosterColor: meta.PosterColor,
		References: meta.References,
		Status:     meta.Status,
		Pinned:     meta.Pinned,
		Author:     meta.Author,
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
		Banner:     meta.Banner,
		Date:       meta.Date,
		Status:     meta.Status,
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
