package template

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Engine 模板渲染引擎。把各子目录下所有 .html 解析进同一个
// template.Template 集合，模板名取相对 root 的路径（去 .html，分隔符 /），
// 如 layout/base、partial/comments、admin/comments，供 {{template "name"}} 嵌套。
// 注意：必须用路径前缀命名——不同子目录允许同名文件（如前台与管理页评论区），
// 若只用基名会互相覆盖（曾为 admin/comments 被 partial/comments 覆盖的 bug）。
type Engine struct {
	tpl *template.Template
}

// templateFS 嵌入 layout/partial 两目录的 .html 模板，部署时无需外部文件。
// （原服务端渲染后台 admin/ 模板已随后台退役删除，管理界面统一到 Vue Console。）
//
//go:embed layout partial
var templateFS embed.FS

// tplEntry 单个模板文件的名字（路径化，去 .html）与内容。
type tplEntry struct {
	name    string
	content []byte
}

// NewEngine 从 root 目录加载所有 .html 模板（开发期从磁盘读，go run -dev 用）。
// root 形如 internal/template，下含 layout/ 与 partial/ 子目录。
func NewEngine(root string, reg *IconRegistry) (*Engine, error) {
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
func NewEngineFromEmbed(reg *IconRegistry) (*Engine, error) {
	var entries []tplEntry
	for _, dir := range []string{"layout", "partial"} {
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
				name:    strings.TrimSuffix(filepath.ToSlash(path), ".html"),
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
func buildEngine(entries []tplEntry, reg *IconRegistry) (*Engine, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("no html templates found")
	}
	tpl := template.New("").Funcs(FuncMap(reg))
	for _, e := range entries {
		if _, err := tpl.New(e.name).Parse(string(e.content)); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.name, err)
		}
	}
	return &Engine{tpl: tpl}, nil
}

// Render 渲染指定模板名（如 "layout/base"），返回 HTML 字符串。
func (e *Engine) Render(name string, data *ViewData) (string, error) {
	t := e.tpl.Lookup(name)
	if t == nil {
		return "", fmt.Errorf("template %q not found", name)
	}
	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("execute %s: %w", name, err)
	}
	return sb.String(), nil
}
