package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"blog/internal/post"
	"blog/internal/settings"
	"blog/internal/stellarhtml"
	"blog/modules/stellar/clientconfig"
)

// Engine 模板渲染引擎。把各子目录下所有 .html 解析进同一个
// template.Template 集合，模板名取相对 root 的路径（去 .html，分隔符 /），
// 如 layout/base、partial/comments、admin/comments，供 {{template "name"}} 嵌套。
// 注意：必须用路径前缀命名——不同子目录允许同名文件（如前台与管理页评论区），
// 若只用基名会互相覆盖（曾为 admin/comments 被 partial/comments 覆盖的 bug）。
type Engine struct {
	tpl                   *template.Template
	icons                 *settings.IconRegistry
	imageEnricher         func(string, *StellarRenderData) (string, error)
	stellarMarkdownRender func(string, post.StellarRenderContext) (string, error)
	host                  StellarHost
}

// SetStellarHost 在启动时注入二进制版本；不从文章或主题参数推导身份。
func (e *Engine) SetStellarHost(host StellarHost) { e.host = host }

// SetStellarMarkdownRender 在服务启动前装配宿主严格渲染器；每次调用接收
// 当前请求的主题、公开数据、页面与语言，不读取全局模板 Markdown 回调。
func (e *Engine) SetStellarMarkdownRender(render func(string, post.StellarRenderContext) (string, error)) {
	e.stellarMarkdownRender = render
}

// SetStellarImageEnricher 在装配时设置缓存回填；请求渲染不抓取图片。
func (e *Engine) SetStellarImageEnricher(f func(string, *StellarRenderData) (string, error)) {
	e.imageEnricher = f
}

// templateFS 嵌入 layout/partial 两目录的 .html 模板，部署时无需外部文件。
// （原服务端渲染后台 admin/ 模板已随后台退役删除，管理界面统一到 Vue Console。）
//
//go:embed templates
var templateFS embed.FS

// tplEntry 单个模板文件的名字（路径化，去 .html）与内容。
type tplEntry struct {
	name    string
	content []byte
}

// NewEngine 从 root 目录加载所有 .html 模板（开发期从磁盘读，go run -dev 用）。
// root 形如 modules/stellar/web/templates，下含 layout/ 与 partial/ 子目录。
func NewEngine(root string, reg *settings.IconRegistry) (*Engine, error) {
	var entries []tplEntry
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		// 模板名取相对 root 的路径（去 .html，分隔符统一 /），子目录做前缀防冲突。
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("rel path %s: %w", path, err)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		entries = append(entries, tplEntry{
			name:    strings.TrimSuffix(filepath.ToSlash(rel), ".html"),
			content: b,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk template dir: %w", err)
	}
	return buildEngine(entries, reg)
}

// NewEngineFromEmbed 从 go:embed 嵌入的模板加载（部署用，单二进制无外部模板文件）。
// 命名方案与 NewEngine 一致：embed FS 路径本身即相对 template 包根（如 layout/base.html），
// 去掉 .html 后缀即为路径化模板名（layout/base）。
func NewEngineFromEmbed(reg *settings.IconRegistry) (*Engine, error) {
	var entries []tplEntry
	for _, dir := range []string{"templates/layout", "templates/partial"} {
		err := fs.WalkDir(templateFS, dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".html") {
				return nil
			}
			b, err := templateFS.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			entries = append(entries, tplEntry{
				name:    strings.TrimSuffix(strings.TrimPrefix(filepath.ToSlash(path), "templates/"), ".html"),
				content: b,
			})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk embed template dir %s: %w", dir, err)
		}
	}
	return buildEngine(entries, reg)
}

// buildEngine 把收集到的模板条目解析进同一个 template.Template。
func buildEngine(entries []tplEntry, reg *settings.IconRegistry) (*Engine, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("no html templates found")
	}
	tpl := template.New("").Funcs(FuncMap(reg))
	for _, e := range entries {
		if _, err := tpl.New(e.name).Parse(string(e.content)); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.name, err)
		}
	}
	markdown := post.NewService("")
	if reg != nil {
		markdown.SetIconFunc(reg.Get)
	}
	return &Engine{tpl: tpl, icons: reg, stellarMarkdownRender: markdown.RenderStellarV2WithContext, host: StellarHost{Name: "Blog", Version: "dev", Homepage: "https://github.com/SakuraLoveSmile/Sakura-Blog"}}, nil
}

// Render 渲染指定布局模板名（如 "layout/base"），返回 HTML 字符串。
func (e *Engine) Render(name string, data *settings.ViewData) (string, error) {
	if name == "layout/base" && data != nil && data.Site != nil && data.Site.StellarV2 != nil {
		prepared, err := PrepareStellar(data, e.icons)
		if err != nil {
			return "", err
		}
		prepared.Host = e.host
		regions := []string{}
		for _, region := range []string{"topbar", "leftbar", "rightbar"} {
			if prepGet(prepared.Stellar.Regions, region+".enabled") == false {
				continue
			}
			prepared.Region = region
			body, err := e.render("partial/stellar/region_body", prepared)
			if err != nil {
				return "", err
			}
			if strings.TrimSpace(body) != "" {
				prepared.Stellar.RegionBodies[region] = template.HTML(body)
				regions = append(regions, region)
			}
		}
		prepared.Region = ""
		prepared.Stellar.RegionNames = strings.Join(regions, " ")
		prepared.Stellar.MobileBrand = prepared.Stellar.MobileBrand && prepared.Stellar.RegionBodies["topbar"] == ""
		output, err := e.render("layout/stellar", prepared)
		if err != nil {
			return "", err
		}
		output, err = stellarhtml.ProcessSite(output, stellarhtml.Context{Theme: prepared.Stellar.Theme, SiteURL: data.Site.URL, IconData: e.icons.Get})
		if err != nil {
			return "", err
		}
		if e.imageEnricher != nil {
			output, err = e.imageEnricher(output, prepared)
			if err != nil {
				return "", err
			}
		}
		if data.Stellar != nil && prepared.Stellar.Model != nil && prepGet(prepared.Stellar.Theme, "features.partialNavigation.enabled") == true && data.Stellar.Input.RuntimeData["preview"] != true {
			collection := prepared.Stellar.Model.Collection
			pageURL := prepString(data.Stellar.Input.Page["permalink"])
			if data.Stellar.RequestURL != "" {
				pageURL = data.Stellar.RequestURL
			}
			if pageURL == "" {
				pageURL = data.Page.CanonicalURL
			}
			return clientconfig.PrepareNavigation(output, clientconfig.NavigationOptions{
				CollectionProfile: prepString(collection["profile"]), CollectionID: prepString(collection["id"]),
				PageURL: pageURL, Root: prepared.Stellar.Root, Members: data.Stellar.NavigationMembers,
			})
		}
		return output, nil
	}
	return e.render(name, data)
}

// RenderValue renders a public partial with its local view model. Layout
// callers keep using the strongly typed Render method above, while collection
// and widget tests can exercise the shared partials directly.
func (e *Engine) RenderValue(name string, data any) (string, error) {
	return e.render(name, data)
}

func (e *Engine) render(name string, data any) (string, error) {
	if prepared, ok := data.(*StellarRenderData); ok {
		prepared.markdownRender = e.stellarMarkdownRender
		prepared.markdownError = nil
	}
	t := e.tpl.Lookup(name)
	if t == nil {
		return "", fmt.Errorf("template %q not found", name)
	}
	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("execute %s: %w", name, err)
	}
	if prepared, ok := data.(*StellarRenderData); ok && prepared.markdownError != nil {
		return "", fmt.Errorf("render %s Markdown: %w", name, prepared.markdownError)
	}
	return sb.String(), nil
}
