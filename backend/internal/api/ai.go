package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/config"
)

// AI 属性生成代理（Spec 30）：Console 编辑器"AI 属性生成"弹窗的后端。
// 读取 settings "ai" 段的 OpenAI 兼容配置，调用 {base_url}/chat/completions，
// 由模型按要求返回 JSON，解析后回传给前端。

// aiAllowedFields 可生成的字段白名单。
var aiAllowedFields = map[string]bool{
	"title":    true,
	"slug":     true,
	"category": true,
	"tags":     true,
	"excerpt":  true,
}

type aiGenerateReq struct {
	Fields     []string `json:"fields"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Categories []string `json:"categories"`
	Tags       []string `json:"tags"`
}

// aiResult 模型返回的结构化结果（字段与请求 fields 对应，未请求的为空）。
type aiResult struct {
	Title    string   `json:"title,omitempty"`
	Slug     string   `json:"slug,omitempty"`
	Category string   `json:"category,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Excerpt  string   `json:"excerpt,omitempty"`
}

// aiGenerate POST /api/v1/ai/generate：按请求字段生成文章属性。
func (h *Handler) aiGenerate(c *gin.Context) {
	var req aiGenerateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if len(req.Fields) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未指定生成字段"})
		return
	}
	for _, f := range req.Fields {
		if !aiAllowedFields[f] {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("不支持的字段: %s", f)})
			return
		}
	}
	if strings.TrimSpace(req.Content) == "" && strings.TrimSpace(req.Title) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "正文与标题均为空，无法生成"})
		return
	}

	cfg, err := config.LoadAI(h.store)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if cfg.BaseURL == "" || cfg.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AI 未配置，请到 设置 → AI 中填写接口地址与模型"})
		return
	}

	system, user := buildAIPrompt(req)
	content, err := callChatCompletion(cfg, system, user)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "AI 调用失败: " + err.Error()})
		return
	}
	result, err := parseAIContent(content)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "AI 返回内容解析失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// aiFieldSpecs 各字段的生成指令（拼入 user prompt）。
var aiFieldSpecs = map[string]string{
	"title":    `"title": 一个简洁有吸引力的中文标题（20 字以内，不带书名号）`,
	"slug":     `"slug": URL 别名，小写英文单词用连字符连接（如 my-first-post），根据文章主题意译`,
	"category": `"category": 一个最贴切的分类名`,
	"tags":     `"tags": 3-5 个标签组成的字符串数组`,
	"excerpt":  `"excerpt": 100 字以内的中文摘要，概括文章核心内容`,
}

// aiContentLimit 送入模型的正文长度上限（rune），控制 token 消耗。
const aiContentLimit = 6000

// buildAIPrompt 组装 system/user 提示词：只输出 JSON，分类/标签优先复用现有 taxonomy。
func buildAIPrompt(req aiGenerateReq) (system, user string) {
	system = "你是博客写作助手。根据用户提供的文章内容生成指定的属性字段。" +
		"必须只输出一个合法的 JSON 对象，不要输出任何解释、前后缀或 Markdown 代码块标记。"

	var b strings.Builder
	b.WriteString("请为下面的文章生成以下字段，输出 JSON 对象：\n")
	for _, f := range req.Fields {
		if spec, ok := aiFieldSpecs[f]; ok {
			b.WriteString("- " + spec + "\n")
		}
	}
	if contains(req.Fields, "category") && len(req.Categories) > 0 {
		b.WriteString("\n现有分类（优先从中选择，都不合适才创建新的）：" + strings.Join(req.Categories, "、") + "\n")
	}
	if contains(req.Fields, "tags") && len(req.Tags) > 0 {
		b.WriteString("现有标签（优先从中选择，可补充新标签）：" + strings.Join(req.Tags, "、") + "\n")
	}
	if t := strings.TrimSpace(req.Title); t != "" {
		b.WriteString("\n文章标题：" + t + "\n")
	}
	content := req.Content
	if runes := []rune(content); len(runes) > aiContentLimit {
		content = string(runes[:aiContentLimit]) + "\n…（正文已截断）"
	}
	b.WriteString("\n文章正文（Markdown）：\n" + content)
	return system, b.String()
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// chatCompletionResp OpenAI 兼容响应（只取需要的字段）。
type chatCompletionResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// aiHTTPClient AI 请求专用 client（30s 超时，生成较慢的本地模型可覆盖大多数场景）。
var aiHTTPClient = &http.Client{Timeout: 30 * time.Second}

// callChatCompletion 调用 OpenAI 兼容 /chat/completions，返回首个 choice 的文本内容。
// base_url 仅由已认证管理员在设置中配置（与 SMTP 配置同信任级），
// 且本地模型（如 Ollama 的 localhost）是明确使用场景，故不做内网地址限制。
func callChatCompletion(cfg config.AIConfig, system, user string) (string, error) {
	payload := map[string]any{
		"model": cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.3,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	url := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	resp, err := aiHTTPClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var cr chatCompletionResp
	if err := json.Unmarshal(data, &cr); err != nil {
		return "", fmt.Errorf("响应非 JSON（HTTP %d）", resp.StatusCode)
	}
	if cr.Error != nil && cr.Error.Message != "" {
		return "", fmt.Errorf("%s", cr.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("响应缺少 choices")
	}
	return cr.Choices[0].Message.Content, nil
}

// parseAIContent 解析模型输出为结构化结果：容忍 ```json 代码块包裹与前后杂文。
func parseAIContent(content string) (aiResult, error) {
	var res aiResult
	s := strings.TrimSpace(content)
	// 剥离代码块围栏
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
	}
	// 截取首个 { 到最后一个 } 之间的内容，容忍前后杂文
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return res, fmt.Errorf("未找到 JSON 对象")
	}
	if err := json.Unmarshal([]byte(s[start:end+1]), &res); err != nil {
		return res, err
	}
	return res, nil
}
