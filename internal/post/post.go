package post

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	mdparser "github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	"blog/internal/plugin"
	tmpl "blog/internal/settings"
)

// Post 文章渲染结果。
type Post struct {
	Slug       string
	Title      string
	HTML       string // goldmark 渲染后的正文 HTML
	Excerpt    string
	Banner     string   // 文首横幅图 URL
	Categories []string // 多分类路径（frontmatter categories）
	Date       string
	Status     string   // draft / published，空=由旧文件回退链路按公开内容兼容
	References []string // 参考文献列表（frontmatter references 字段）
	Tags       []string // 标签列表（1.41 article.tags 末尾标签行）
	AILabel    string   // AI 成分标签档位（1.41 ai_label）
	Authors    []string // 作者 ID 列表
	Author     string   // 第一位作者，兼容旧 frontmatter
}

// FileMeta 从 Markdown frontmatter 解析出的元数据，供 seed 导入跨包使用。
type FileMeta struct {
	Slug       string
	Title      string
	Date       string
	Excerpt    string
	Cover      string
	Banner     string // 文首横幅图 URL
	Category   string
	Categories []string
	Topic      string // 专栏名
	Tags       []string
	AILabel    string
	// 海报封面（poster_headline 等平铺 key，对齐原版 post.poster）
	PosterHeadline string
	PosterTopic    string
	PosterCaption  string
	PosterColor    string
	// 参考文献（对齐原版 article_footer.ejs page.references）
	References      []string
	MetaDescription string
	MetaKeywords    string
	// 备份导出/导入扩展字段（seed 源文件一般不写，缺失时保持零值）
	Status  string   // draft / published，空=由导入方决定
	Pinned  bool     // 置顶
	Authors []string // 作者 ID 列表，按 frontmatter 顺序
	Author  string   // 第一位作者，兼容旧 frontmatter
}

// Service 文章服务，从 markdown 目录加载并渲染文章。
type Service struct {
	mdDir    string
	md       goldmark.Markdown
	inlineMD goldmark.Markdown // 仅行内语法渲染器（供标签插件 body 使用）
	parser   parser
	settings *tmpl.Manager
	iconFunc func(string) string // 渲染内联 SVG（供 quot 等 shortcode 用），由 main.go 用 IconRegistry 装配
	ghClient GHClient            // GitHub 数据客户端（供 ghrepo/ghuser shortcode 用），nil 则降级纯链接
	// linksFunc 返回站点友链分组与缺省头像（default.link），供 {% links %} 服务端直出；nil 时渲染空态。
	linksFunc func() ([]tmpl.LinkGroup, string)
	// albumsLinksFunc 返回站点友链分组与缺省封面（default.cover），供 albums/posters group 直出；nil 时回退 linksFunc 或空态。
	albumsLinksFunc func() ([]tmpl.LinkGroup, string)
	// wikiTreeProvider 查询指定项目的已发布公开文档树，供 {% toc wiki:xxx %} 渲染；nil 时对齐原版输出空。
	wikiTreeProvider func(projectID string) (bool, []tmpl.WikiTreeNode)
	// momentsFunc 返回最新公开瞬间，供 {% moments %} shortcode 服务端直出；nil 时渲染空态。
	momentsFunc func(limit int) []*tmpl.MomentItem
	plugins     *plugin.Runtime
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

// SetAlbumsLinksFunc 注入友链分组与默认封面提供器（供 {% albums %} / {% posters %} shortcode 服务端直出）。
func (s *Service) SetAlbumsLinksFunc(f func() ([]tmpl.LinkGroup, string)) {
	s.albumsLinksFunc = f
}

// SetWikiTreeProvider 注入 Wiki 文档树提供器（供 {% toc wiki:xxx %} shortcode 服务端直出）。
func (s *Service) SetWikiTreeProvider(f func(projectID string) (bool, []tmpl.WikiTreeNode)) {
	s.wikiTreeProvider = f
}

func (s *Service) renderAlbumsPosters(kind, argsStr string) string {
	var groups []tmpl.LinkGroup
	defaultCover := ""
	if s.albumsLinksFunc != nil {
		groups, defaultCover = s.albumsLinksFunc()
	} else if s.linksFunc != nil {
		groups, _ = s.linksFunc()
	}
	return renderAlbumsPostersHTML(kind, argsStr, groups, defaultCover)
}

func (s *Service) renderTOC(argsStr string) string {
	return renderTOCTag(argsStr, s.wikiTreeProvider, false)
}

// SetMomentsFunc 注入最新公开瞬间提供器（供 {% moments %} shortcode 服务端直出）。
func (s *Service) SetMomentsFunc(f func(limit int) []*tmpl.MomentItem) {
	s.momentsFunc = f
}

// SetGHClient 注入 GitHub 数据客户端（供 ghrepo/ghuser shortcode 渲染）。未设置时降级纯链接。
func (s *Service) SetGHClient(c GHClient) {
	s.ghClient = c
}

// SetPluginRuntime enables the declared shortcode/content-filter hooks. A nil
// runtime preserves the normal renderer for tests and minimal embedders.
func (s *Service) SetPluginRuntime(runtime *plugin.Runtime) { s.plugins = runtime }

// SetSettingsManager supplies the current site snapshot to shortcode rendering
// without relying on package-global tag plugin state.
func (s *Service) SetSettingsManager(m *tmpl.Manager) { s.settings = m }

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
				stellarCodeBlocks{},
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
	if s.plugins != nil {
		value, err := s.plugins.Apply(context.Background(), plugin.HookShortcode, body)
		if err != nil {
			return "", fmt.Errorf("plugin shortcode hook: %w", err)
		}
		if transformed, ok := value.(string); ok {
			body = transformed
		}
	}
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
	var tagPlugins tmpl.TagPluginsConfig
	if s.settings != nil && s.settings.Current() != nil {
		tagPlugins = s.settings.Current().TagPlugins
	}
	processed := processShortcodes(protectMathDelimiters(body), renderMD, s.renderInlineMD, s.iconFunc, s.ghClient, s.renderLinks, s.renderMoments, s.renderAlbumsPosters, s.renderTOC, tagPlugins)

	// 2. goldmark 渲染整篇（HTML 块会被保留）
	var sb strings.Builder
	if err := s.md.Convert([]byte(processed), &sb); err != nil {
		return "", err
	}
	// 3. 普通表格包 .md-table-scroll 横向滚动容器（1.41，对齐上游构建期 filter）
	result := wrapTables(sb.String())
	if s.plugins != nil {
		value, err := s.plugins.Apply(context.Background(), plugin.HookContentFilter, result)
		if err != nil {
			return "", fmt.Errorf("plugin content filter hook: %w", err)
		}
		if transformed, ok := value.(string); ok {
			result = transformed
		}
	}
	return result, nil
}

// ParseBytes 解析 Markdown 原文（含可选 frontmatter），返回元数据与正文。
// 供备份导入等内存数据源复用，与 ParseFile 共享同一轻量 parser。
func (s *Service) ParseBytes(src []byte) (FileMeta, string) {
	meta, body := s.parser.Parse(src)
	return FileMeta(meta), body
}

// ParseImport validates and parses a Markdown document for editor import.
// Unlike ParseBytes it reports malformed front matter and exposes the fields
// that were present so callers can warn before applying the snapshot.
func (s *Service) ParseImport(src []byte) (MarkdownImportResult, error) {
	return s.parser.ParseImport(src)
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
		Tags:       meta.Tags,
		AILabel:    meta.AILabel,
		Title:      title,
		HTML:       html,
		Excerpt:    meta.Excerpt,
		Banner:     meta.Banner,
		Date:       meta.Date,
		Status:     meta.Status,
		References: meta.References,
		Authors:    append([]string(nil), meta.Authors...),
		Author:     meta.Author,
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
