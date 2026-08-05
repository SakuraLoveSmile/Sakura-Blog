package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	ghpkg "blog/internal/github"
	"blog/internal/store"
)

// friendsRepo 友链申请仓库（与 resource/friends 快照同源）。
const friendsRepo = "sakurasep/Friends"

// linkImportRequest 友链导入请求体。
type linkImportRequest struct {
	Group string `json:"group"` // 导入分组标题，空则默认「GitHub 友链」
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

// importLinks POST /api/v1/links/import：从 sakurasep/Friends 仓库拉取 open issues，
// 解析每个 issue body 的友链 JSON 块，按 URL 去重后批量写入 links 表。
func (h *Handler) importLinks(c *gin.Context) {
	if h.gh == nil {
		Fail(c, http.StatusServiceUnavailable, "GitHub 代理未启用，无法导入")
		return
	}
	var req linkImportRequest
	_ = c.ShouldBindJSON(&req)
	group := strings.TrimSpace(req.Group)
	if group == "" {
		group = "GitHub 友链"
	}

	issues, err := h.gh.GetIssues("sakurasep", "Friends", "state=open&per_page=100")
	if err != nil {
		Fail(c, http.StatusBadGateway, "拉取 GitHub issues 失败: "+err.Error())
		return
	}

	existing, err := h.store.ListLinks()
	if err != nil {
		Fail(c, http.StatusInternalServerError, "读取现有友链失败: "+err.Error())
		return
	}
	seen := make(map[string]bool, len(existing))
	for _, l := range existing {
		seen[normalizeLinkURL(l.URL)] = true
	}

	var res linkImportResult
	for _, issue := range issues {
		link, perr := parseFriendIssue(issue)
		if perr != nil {
			res.Failed = append(res.Failed, formatImportFail(issue.Number, perr.Error()))
			continue
		}
		link.GroupTitle = group
		key := normalizeLinkURL(link.URL)
		if seen[key] {
			res.Skipped++
			continue
		}
		if _, err := h.store.CreateLink(link); err != nil {
			res.Failed = append(res.Failed, formatImportFail(issue.Number, "写入失败: "+err.Error()))
			continue
		}
		seen[key] = true
		res.Imported++
	}

	h.afterLinksChanged()
	c.JSON(http.StatusOK, res)
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
