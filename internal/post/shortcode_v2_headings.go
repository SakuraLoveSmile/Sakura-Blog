package post

import (
	"fmt"
	"html"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	mdhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// 复用默认标题输出，保留既有 ID；官方 tech 样式依赖标题首个 headerlink。
type stellarHeadingLinks struct{}

func (stellarHeadingLinks) Extend(markdown goldmark.Markdown) {
	collector := headingRendererCollector{}
	mdhtml.NewRenderer().RegisterFuncs(&collector)
	markdown.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(&stellarHeadingRenderer{fallback: collector.heading}, 200)))
}

type headingRendererCollector struct{ heading renderer.NodeRendererFunc }

func (r *headingRendererCollector) Register(kind ast.NodeKind, fn renderer.NodeRendererFunc) {
	if kind == ast.KindHeading {
		r.heading = fn
	}
}

type stellarHeadingRenderer struct{ fallback renderer.NodeRendererFunc }

func (r *stellarHeadingRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHeading, r.render)
}

func (r *stellarHeadingRenderer) render(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	status, err := r.fallback(w, source, node, entering)
	if err != nil || !entering {
		return status, err
	}
	id, ok := node.AttributeString("id")
	if !ok {
		return status, nil
	}
	var title strings.Builder
	_ = ast.Walk(node, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			switch text := child.(type) {
			case *ast.Text:
				title.Write(util.UnescapePunctuations(text.Value(source)))
			case *ast.String:
				title.Write(text.Value)
			}
		}
		return ast.WalkContinue, nil
	})
	fragment := fmt.Sprint(id)
	if bytes, ok := id.([]byte); ok {
		fragment = string(bytes)
	}
	_, err = fmt.Fprintf(w, `<a href="#%s" class="headerlink" title="%s"></a>`, html.EscapeString(fragment), html.EscapeString(html.UnescapeString(title.String())))
	return status, err
}
