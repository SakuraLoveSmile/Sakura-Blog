package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	yaml "github.com/goccy/go-yaml"

	ghpkg "blog/internal/github"
	"blog/internal/store"
)

// friendsRepo 友链申请仓库（与 resource/friends 快照同源）。
const friendsRepo = "sakurasep/Friends"

// linkImportRequest 友链导入请求体（Spec 56：来源泛化）。
type linkImportRequest struct {
	Source  string `json:"source"`  // 导入来源："github"（默认）/ "text"（粘贴 JSON 或 YAML）
	Group   string `json:"group"`   // 导入分组标题，空则用当前来源默认值
	Content string `json:"content"` // text 来源的导入内容
}

// linkImportResult 导入结果。
type linkImportResult struct {
	Imported int      `json:"imported"` // 新写入条数
	Skipped  int      `json:"skipped"`  // 已存在跳过条数
	Failed   []string `json:"failed"`   // 解析失败条目，格式 "issue #N: 原因"
}

// friendIssueJSON issue body ```json 块的友链字段（兼容 avatar/icon、feed/rss 两代格式）。
type friendIssueJSON struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Avatar      string `json:"avatar"`
	Icon        string `json:"icon"` // 旧格式别名
	Description string `json:"description"`
	Feed        string `json:"feed"`
	RSS         string `json:"rss"`
}

// importLinks POST /api/v1/links/import：按来源批量导入友链（Spec 56）。
// github：拉取 sakurasep/Friends 仓库 open issues 解析友链 JSON 块；
// text：解析粘贴的 JSON 数组或 YAML 列表。均按 URL 去重写入 links 表。
func (h *Handler) importLinks(c *gin.Context) {
	var req linkImportRequest
	_ = c.ShouldBindJSON(&req)
	source := strings.ToLower(strings.TrimSpace(req.Source))
	if source == "" {
		source = "github"
	}

	var items []*store.Link
	var res linkImportResult
	var group string
	switch source {
	case "github":
		if h.gh == nil {
			Fail(c, http.StatusServiceUnavailable, "GitHub 代理未启用，无法导入")
			return
		}
		group = strings.TrimSpace(req.Group)
		if group == "" {
			group = "GitHub 友链"
		}
		issues, err := h.gh.GetIssues("sakurasep", "Friends", "state=open&per_page=100")
		if err != nil {
			Fail(c, http.StatusBadGateway, "拉取 GitHub issues 失败: "+err.Error())
			return
		}
		for _, issue := range issues {
			link, perr := parseFriendIssue(issue)
			if perr != nil {
				res.Failed = append(res.Failed, formatImportFail(issue.Number, perr.Error()))
				continue
			}
			items = append(items, link)
		}
	case "text":
		group = strings.TrimSpace(req.Group)
		if group == "" {
			group = "导入友链"
		}
		parsed, failed, err := parseTextLinks(req.Content)
		if err != nil {
			Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		items = parsed
		res.Failed = failed
	default:
		Fail(c, http.StatusBadRequest, "未知导入来源: "+source)
		return
	}

	if err := h.writeImportedLinks(items, group, &res); err != nil {
		Fail(c, http.StatusInternalServerError, "读取现有友链失败: "+err.Error())
		return
	}
	h.afterLinksChanged()
	c.JSON(http.StatusOK, res)
}

// writeImportedLinks 按 URL 对库内既有友链去重后批量写入，更新 res 计数（重复跳过、写入失败附原因）。
func (h *Handler) writeImportedLinks(items []*store.Link, group string, res *linkImportResult) error {
	existing, err := h.store.ListLinks()
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(existing))
	for _, l := range existing {
		seen[normalizeLinkURL(l.URL)] = true
	}
	for _, link := range items {
		link.GroupTitle = group
		key := normalizeLinkURL(link.URL)
		if seen[key] {
			res.Skipped++
			continue
		}
		if _, err := h.store.CreateLink(link); err != nil {
			res.Failed = append(res.Failed, fmt.Sprintf("写入失败: %s (%s): %v", link.Name, link.URL, err))
			continue
		}
		seen[key] = true
		res.Imported++
	}
	return nil
}

// textLinkEntry 粘贴导入条目字段（别名与 issue JSON 块一致，另透传 label/color/sort）。
type textLinkEntry struct {
	Name        string `json:"name" yaml:"name"`
	Title       string `json:"title" yaml:"title"`
	URL         string `json:"url" yaml:"url"`
	Avatar      string `json:"avatar" yaml:"avatar"`
	Icon        string `json:"icon" yaml:"icon"`
	Description string `json:"description" yaml:"description"`
	Feed        string `json:"feed" yaml:"feed"`
	RSS         string `json:"rss" yaml:"rss"`
	Label       string `json:"label" yaml:"label"`
	Color       string `json:"color" yaml:"color"`
	Sort        int    `json:"sort" yaml:"sort"`
}

// parseTextLinks 解析粘贴文本（JSON 数组或 YAML 列表，均支持 links 顶层键），
// 返回可解析条目与失败原因（"第 N 条: 原因"）；整体无法解析或无条目返回 error。
func parseTextLinks(content string) ([]*store.Link, []string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, nil, errors.New("导入内容为空")
	}
	entries, err := decodeTextEntries(content)
	if err != nil {
		return nil, nil, errors.New("无法解析导入内容，需为 JSON 数组或 YAML 列表")
	}
	if len(entries) == 0 {
		return nil, nil, errors.New("导入内容中没有友链条目")
	}
	var links []*store.Link
	var failed []string
	for i, e := range entries {
		name := strings.TrimSpace(firstNonEmpty(e.Name, e.Title))
		url := strings.TrimSpace(e.URL)
		switch {
		case url == "":
			failed = append(failed, fmt.Sprintf("第 %d 条: 缺 url", i+1))
			continue
		case name == "":
			failed = append(failed, fmt.Sprintf("第 %d 条: 缺 name/title", i+1))
			continue
		}
		links = append(links, &store.Link{
			Name:        name,
			URL:         url,
			Avatar:      strings.TrimSpace(firstNonEmpty(e.Avatar, e.Icon)),
			Description: strings.TrimSpace(e.Description),
			RSS:         strings.TrimSpace(firstNonEmpty(e.Feed, e.RSS)),
			Label:       strings.TrimSpace(e.Label),
			Color:       strings.TrimPrefix(strings.TrimSpace(e.Color), "#"),
			Sort:        e.Sort,
		})
	}
	return links, failed, nil
}

// decodeTextEntries 先按 JSON（数组或 {links:[...]}）再按 YAML 同构解析。
func decodeTextEntries(content string) ([]textLinkEntry, error) {
	var arr []textLinkEntry
	if err := json.Unmarshal([]byte(content), &arr); err == nil {
		return arr, nil
	}
	var wrap struct {
		Links []textLinkEntry `json:"links" yaml:"links"`
	}
	if err := json.Unmarshal([]byte(content), &wrap); err == nil && len(wrap.Links) > 0 {
		return wrap.Links, nil
	}
	if err := yaml.Unmarshal([]byte(content), &arr); err == nil {
		return arr, nil
	}
	if err := yaml.Unmarshal([]byte(content), &wrap); err == nil && len(wrap.Links) > 0 {
		return wrap.Links, nil
	}
	return nil, errors.New("decode failed")
}

// parseFriendIssue 从单个 issue 提取友链条目。body 须含 ```json 块；
// 返回 *store.Link（不含 ID/分组），JSON 块缺失、解析失败或缺 url 时报错。
func parseFriendIssue(issue ghpkg.Issue) (*store.Link, error) {
	block := extractJSONBlock(issue.Body)
	if block == "" {
		return nil, errNoJSONBlock
	}
	var raw friendIssueJSON
	if err := json.Unmarshal([]byte(block), &raw); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(raw.Title)
	if name == "" {
		name = strings.TrimSpace(issue.Title)
	}
	url := strings.TrimSpace(raw.URL)
	if url == "" {
		return nil, errNoURL
	}
	link := &store.Link{
		Name:        name,
		URL:         url,
		Avatar:      strings.TrimSpace(firstNonEmpty(raw.Avatar, raw.Icon)),
		Description: strings.TrimSpace(raw.Description),
		RSS:         strings.TrimSpace(firstNonEmpty(raw.Feed, raw.RSS)),
	}
	if len(issue.Labels) > 0 && strings.TrimSpace(issue.Labels[0].Name) != "" {
		link.Label = strings.TrimSpace(issue.Labels[0].Name)
	}
	return link, nil
}

var (
	errNoJSONBlock = &importParseError{"未找到 ```json 友链信息块"}
	errNoURL       = &importParseError{"友链信息缺 url"}
)

// importParseError 导入解析错误（附 issue 号拼接用）。
type importParseError struct{ msg string }

func (e *importParseError) Error() string { return e.msg }

// formatImportFail "issue #N: 原因"。
func formatImportFail(number int, reason string) string {
	return fmt.Sprintf("issue #%d: %s", number, reason)
}

// extractJSONBlock 提取 body 中第一个 ```json 代码块内容；无则返回空串。
func extractJSONBlock(body string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "```json" {
			continue
		}
		var buf []string
		for _, l := range lines[i+1:] {
			if strings.TrimSpace(l) == "```" {
				return strings.Join(buf, "\n")
			}
			buf = append(buf, l)
		}
		return strings.Join(buf, "\n")
	}
	return ""
}

// normalizeLinkURL 用于去重的 URL 规范化：去空白与尾部斜杠。
func normalizeLinkURL(u string) string {
	return strings.TrimRight(strings.TrimSpace(u), "/")
}

// firstNonEmpty 返回首个非空串（trim 后判断）。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
