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

// Engine 模板渲染引擎。把 layout/ 和 partial/ 下所有 .html 解析进同一个
// template.Template 集合，模板名取文件基名（去 .html），供 {{template "name"}} 嵌套。
type Engine struct {
	tpl *template.Template
}

// templateFS 嵌入 partial/layout/admin 三目录的 .html 模板，部署时无需外部文件。
//
//go:embed partial layout admin
var templateFS embed.FS

// NewEngine 从 root 目录加载所有 .html 模板（开发期从磁盘读，go run 用）。
// root 形如 internal/template，下含 layout/ 与 partial/ 子目录。
func NewEngine(root string, reg *IconRegistry) (*Engine, error) {
	return newEngineFromWalk(func(name string) ([]byte, error) {
		return os.ReadFile(filepath.Join(root, name))
	}, reg)
}

// NewEngineFromEmbed 从 go:embed 嵌入的模板加载（部署用，单二进制无外部模板文件）。
func NewEngineFromEmbed(reg *IconRegistry) (*Engine, error) {
	return newEngineFromWalk(func(name string) ([]byte, error) {
		return templateFS.ReadFile(name)
	}, reg)
}

// newEngineFromWalk 遍历 embed FS 的 partial/layout/admin 目录收集 .html 文件，
// 用 read 读内容解析进同一个 template.Template。模板名取文件基名（去 .html）。
func newEngineFromWalk(read func(name string) ([]byte, error), reg *IconRegistry) (*Engine, error) {
	tpl := template.New("").Funcs(FuncMap(reg))

	// 收集嵌入 FS 下三个目录的 .html 文件路径（相对 template 包根，如 partial/head.html）
	var files []string
	for _, dir := range []string{"partial", "layout", "admin"} {
		err := fs.WalkDir(templateFS, dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".html") {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk embed template dir %s: %w", dir, err)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no html templates found in embed FS")
	}

	for _, f := range files {
		// 模板名取文件基名（去 .html）。各子目录文件名已保证不冲突
		// （layout/base、partial/head、admin/list 等基名唯一）。
		name := strings.TrimSuffix(filepath.Base(f), ".html")
		b, err := read(f)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f, err)
		}
		if _, err := tpl.New(name).Parse(string(b)); err != nil {
			return nil, fmt.Errorf("parse %s: %w", f, err)
		}
	}
	return &Engine{tpl: tpl}, nil
}

// Render 渲染指定模板名（如 "base"），返回 HTML 字符串。
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

// RenderBytes 同 Render，返回字节切片（给 Gin 直接写响应）。
func (e *Engine) RenderBytes(name string, data *ViewData) ([]byte, error) {
	t := e.tpl.Lookup(name)
	if t == nil {
		return nil, fmt.Errorf("template %q not found", name)
	}
	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		return nil, fmt.Errorf("execute %s: %w", name, err)
	}
	return []byte(sb.String()), nil
}
