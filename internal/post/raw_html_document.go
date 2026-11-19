package post

import (
	"html"
	"regexp"
	"strconv"
	"strings"
)

// 整页 HTML 文档不能作为片段直接内联进正文：浏览器会丢弃 html/head/body 标签，
// 留下的页面级 CSS（body 背景、通配 reset、html/body 高度等）会改写宿主站点版式；
// 小写 <!doctype 还会被 goldmark 当成普通段落转义成可见文本（goldmark 只把
// "<!" + 大写字母识别为 HTML 块）。这里把这类整页文档整体渲染成隔离的 sandbox iframe，
// 效果与单独打开文件一致，且不影响站点样式。

const (
	// 首屏高度：回传脚本就绪前先给一个常见页面高度，避免插入瞬间塌成一条线。
	rawHTMLDocumentInitialHeight = 520
	rawHTMLDocumentMinHeight     = 120
	rawHTMLDocumentMaxHeight     = 5000
)

// rawHTMLDocumentStart 匹配整页文档的首行：doctype 或 <html ...>。
var rawHTMLDocumentStart = regexp.MustCompile(`(?i)^<!doctype\s+html|^<html(?:\s|>)`)

// rawHTMLDocumentReportScript 在 iframe 文档内回传内容高度（opaque origin，只发消息，不碰父文档）。
const rawHTMLDocumentReportScript = `<script data-stellar-raw-html-report="1">(function(){function r(){var d=document.documentElement,b=document.body;var h=b?b.scrollHeight:0;if(!h&&d){h=d.scrollHeight}try{parent.postMessage({type:"stellar-raw-html-height",height:h},"*")}catch(e){}}window.addEventListener("load",r);if(window.ResizeObserver){try{new ResizeObserver(r).observe(document.documentElement||document.body)}catch(e){}}setTimeout(r,120);setTimeout(r,600);setTimeout(r,2000)})();</script>`

// rawHTMLDocumentResizeScript 在外层文档里把 iframe 高度同步为回传值。
// 用 document.currentScript 定位同组的 iframe，多个整页文档互不影响。
var rawHTMLDocumentResizeScript = `(function(){var s=document.currentScript,w=s&&s.parentNode;var f=w&&w.querySelector("iframe[data-raw-html-frame]");if(!f)return;var apply=function(h){h=Math.ceil(Number(h));if(!isFinite(h)||h<=0)return;f.style.height=Math.max(` + strconv.Itoa(rawHTMLDocumentMinHeight) + `,Math.min(` + strconv.Itoa(rawHTMLDocumentMaxHeight) + `,h))+"px"};window.addEventListener("message",function(e){var d=e.data;if(!d||d.type!=="stellar-raw-html-height")return;if(e.source!==f.contentWindow)return;apply(d.height)})})();`

// isolateRawHTMLDocuments 把正文里的整页文档替换成隔离 iframe，其余内容保持原样。
// 代码块内、缩进块内以及片段级 HTML 不受影响。
func isolateRawHTMLDocuments(body string) string {
	ranges := rawHTMLDocumentRanges(body)
	if len(ranges) == 0 {
		return body
	}
	var out strings.Builder
	prev := 0
	for _, r := range ranges {
		out.WriteString(body[prev:r[0]])
		out.WriteString("\n\n")
		out.WriteString(renderRawHTMLDocumentFrame(body[r[0]:r[1]]))
		out.WriteString("\n\n")
		prev = r[1]
	}
	out.WriteString(body[prev:])
	return out.String()
}

// renderRawHTMLDocumentFrame 生成单行 iframe 标记（单行是为了让 goldmark 把它当作一个 HTML 块）。
func renderRawHTMLDocumentFrame(document string) string {
	withReport := injectRawHTMLReport(ensureStandardsMode(document))
	var b strings.Builder
	b.WriteString(`<div class="md-raw-html-document"><iframe class="md-raw-html-document-frame" data-raw-html-frame="1" sandbox="allow-scripts" title="外部 HTML 页面" style="width:100%;display:block;border:0;background:transparent;height:`)
	b.WriteString(strconv.Itoa(rawHTMLDocumentInitialHeight))
	b.WriteString(`px" srcdoc="`)
	b.WriteString(html.EscapeString(withReport))
	b.WriteString(`"></iframe><script data-stellar-raw-html-resize="1">`)
	b.WriteString(rawHTMLDocumentResizeScript)
	b.WriteString(`</script></div>`)
	return b.String()
}

// ensureStandardsMode 保证 srcdoc 以 doctype 开头：编辑器保存时可能丢掉这一行，
// 少了它会退化成怪异模式（盒模型与高度计算都会变）。
func ensureStandardsMode(document string) string {
	trimmed := strings.TrimLeft(document, " \t\r\n")
	if strings.HasPrefix(strings.ToLower(trimmed), "<!doctype") {
		return document
	}
	return "<!doctype html>\n" + document
}

func injectRawHTMLReport(document string) string {
	if strings.Contains(document, rawHTMLDocumentReportScript) {
		return document
	}
	if at := lastIndexFold(document, "</body>"); at >= 0 {
		return document[:at] + rawHTMLDocumentReportScript + document[at:]
	}
	if at := lastIndexFold(document, "</html>"); at >= 0 {
		return document[:at] + rawHTMLDocumentReportScript + document[at:]
	}
	return document + rawHTMLDocumentReportScript
}

func lastIndexFold(source, sub string) int {
	return strings.LastIndex(strings.ToLower(source), strings.ToLower(sub))
}

// rawHTMLDocumentRanges 返回需要隔离的整页文档区间（左闭右开，含 </html>）。
func rawHTMLDocumentRanges(body string) [][2]int {
	lines := strings.SplitAfter(body, "\n")
	offsets := make([]int, len(lines))
	offset := 0
	for i, line := range lines {
		offsets[i] = offset
		offset += len(line)
	}
	code := v2CodeRanges(body)
	ranges := make([][2]int, 0, 1)
	for i := 0; i < len(lines); i++ {
		start := offsets[i]
		trimmed := strings.TrimLeft(lines[i], " \t")
		if !rawHTMLDocumentStart.MatchString(trimmed) {
			continue
		}
		if inRanges(code, start) || !blockBoundaryBefore(lines, i) {
			continue
		}
		end, lastLine := rawHTMLDocumentEnd(body, lines, offsets, i)
		if end <= start {
			continue
		}
		ranges = append(ranges, [2]int{start, end})
		i = lastLine
	}
	return ranges
}

// blockBoundaryBefore 要求文档首行在文首或空行之后，避免把正文里的行内 <html> 当块。
func blockBoundaryBefore(lines []string, index int) bool {
	return index == 0 || strings.TrimSpace(lines[index-1]) == ""
}

// rawHTMLDocumentEnd 从首行往后找 </html>；找不到时只有「整篇就是一篇文档」才隔离到底。
func rawHTMLDocumentEnd(body string, lines []string, offsets []int, first int) (int, int) {
	lower := strings.ToLower(body)
	if at := strings.Index(lower[offsets[first]:], "</html>"); at >= 0 {
		end := offsets[first] + at + len("</html>")
		lineEnd := strings.IndexByte(body[end:], '\n')
		if lineEnd < 0 {
			lineEnd = len(body) - end
		}
		if strings.TrimSpace(body[end:end+lineEnd]) == "" {
			end += lineEnd
		}
		last := first
		for last+1 < len(offsets) && offsets[last+1] < end {
			last++
		}
		return end, last
	}
	for i := 0; i < first; i++ {
		if strings.TrimSpace(lines[i]) != "" {
			return 0, first
		}
	}
	return len(body), len(lines) - 1
}

func inRanges(ranges [][2]int, pos int) bool {
	for _, r := range ranges {
		if pos >= r[0] && pos < r[1] {
			return true
		}
	}
	return false
}
