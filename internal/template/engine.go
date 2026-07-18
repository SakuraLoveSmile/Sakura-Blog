package template

import (
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

// NewEngine 从 root 目录加载所有 .html 模板。
// root 形如 internal/template，下含 layout/ 与 partial/ 子目录。
func NewEngine(root string, reg *IconRegistry) (*Engine, error) {
	tpl := template.New("").Funcs(FuncMap(reg))

	// 收集所有 .html 文件，保证 base.html 最后解析或先注册关联名
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
		return nil, fmt.Errorf("walk template dir: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no html templates found under %s", root)
	}

	for _, f := range files {
		// 模板名取相对 root 的路径（去 .html，分隔符统一 /），子目录做前缀防冲突。
		rel, err := filepath.Rel(root, f)
		if err != nil {
			return nil, fmt.Errorf("rel path %s: %w", f, err)
		}
		name := strings.TrimSuffix(filepath.ToSlash(rel), ".html")
		b, err := os.ReadFile(f)
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
