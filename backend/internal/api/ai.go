package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
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
	"summary":  true, // Spec 53 AI 文章摘要
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
	Summary  string   `json:"summary,omitempty"` // Spec 53 AI 文章摘要
}

// aiGenerate POST /api/v1/ai/generate：按请求字段生成文章属性。
func (h *Handler) aiGenerate(c *gin.Context) {
	var req aiGenerateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	if len(req.Fields) == 0 {
		Fail(c, http.StatusBadRequest, "未指定生成字段")
		return
	}
	for _, f := range req.Fields {
		if !aiAllowedFields[f] {
			Fail(c, http.StatusBadRequest, fmt.Sprintf("不支持的字段: %s", f))
			return
		}
	}
	if strings.TrimSpace(req.Content) == "" && strings.TrimSpace(req.Title) == "" {
		Fail(c, http.StatusBadRequest, "正文与标题均为空，无法生成")
		return
	}

	cfg, err := config.LoadAI(h.store)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if cfg.BaseURL == "" || cfg.Model == "" {
		Fail(c, http.StatusBadRequest, "AI 未配置，请到 设置 → AI 中填写接口地址与模型")
		return
	}

	system, user := buildAIPrompt(req)
	content, err := callChatCompletion(cfg, system, user, nil)
	if err != nil {
		Fail(c, http.StatusBadGateway, "AI 调用失败: " + err.Error())
		return
	}
	result, err := parseAIContent(content)
	if err != nil {
		Fail(c, http.StatusBadGateway, "AI 返回内容解析失败: " + err.Error())
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
	"summary":  `"summary": 120-200 字的中文导读摘要，面向读者概括全文亮点与结论，语气自然流畅，不同于 excerpt 的列表摘要`,
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
// client 为空时用默认 aiHTTPClient（管理员配置场景：base_url 与 SMTP 同信任级，
// 本地模型如 Ollama 的 localhost 是明确使用场景，故默认 client 不做内网限制）；
// guest 场景传入 newGuestAIHTTPClient 的加固客户端（IP 固定拨号 + 禁重定向）。
func callChatCompletion(cfg config.AIConfig, system, user string, client *http.Client) (string, error) {
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
	if client == nil {
		client = aiHTTPClient
	}
	resp, err := client.Do(httpReq)
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

// --- Spec 53 读者自助生成：临时接入自己的 OpenAI 兼容 API ---

// aiGuestReq 读者自助生成请求：端点/Key/模型均由读者提供，服务端不存。
type aiGuestReq struct {
	Endpoint string `json:"endpoint"` // OpenAI 兼容 base_url
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	Content  string `json:"content"` // 文章正文（前端截断后传入）
}

const (
	aiGuestContentLimit = 40000 // 送入模型的正文 rune 上限
	aiGuestResultLimit  = 2000  // 返回摘要的 rune 上限
)

// aiSummaryGuest POST /api/v1/ai/summary/guest：用读者自己的 API 生成摘要（公开端点）。
// 安全基线：https-only + 私网段拒绝（SSRF）+ per-IP 限流；api_key 仅透传不落不落日志。
func (h *Handler) aiSummaryGuest(c *gin.Context) {
	cfg, err := config.LoadAI(h.store)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !cfg.UserAPIOn() {
		Fail(c, http.StatusForbidden, "本站未开启读者自助生成")
		return
	}
	if !aiGuestRateAllow(c.ClientIP()) {
		Fail(c, http.StatusTooManyRequests, "请求过于频繁，请稍后再试")
		return
	}
	var req aiGuestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	if strings.TrimSpace(req.Endpoint) == "" || strings.TrimSpace(req.Model) == "" || strings.TrimSpace(req.Content) == "" {
		Fail(c, http.StatusBadRequest, "接口地址、模型与正文均必填")
		return
	}
	ips, err := validateGuestAIEndpoint(req.Endpoint)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errPrivateHost) {
			status = http.StatusForbidden // SSRF 私网段：403
		}
		Fail(c, status, "接口地址不可用："+err.Error())
		return
	}
	content := req.Content
	if runes := []rune(content); len(runes) > aiGuestContentLimit {
		content = string(runes[:aiGuestContentLimit])
	}
	guest := config.AIConfig{BaseURL: req.Endpoint, APIKey: req.APIKey, Model: req.Model}
	// 固定拨号已校验 IP + 禁重定向（封堵 TOCTOU 重绑定与 302 内网跳转）
	epURL, _ := url.Parse(strings.TrimSpace(req.Endpoint))
	client := newGuestAIHTTPClient(epURL.Hostname(), ips)
	text, err := callChatCompletion(guest,
		"你是博客文章摘要助手。根据用户提供的文章正文生成一段导读摘要。只输出摘要文本本身，不要任何解释、标题或 Markdown 标记。",
		"请为下面的文章生成 120-200 字的中文导读摘要，概括全文亮点与结论：\n"+content, client)
	if err != nil {
		Fail(c, http.StatusBadGateway, "AI 调用失败: " + err.Error())
		return
	}
	summary := strings.TrimSpace(text)
	if runes := []rune(summary); len(runes) > aiGuestResultLimit {
		summary = string(runes[:aiGuestResultLimit])
	}
	c.JSON(http.StatusOK, gin.H{"summary": summary})
}

// validateGuestAIEndpoint 校验读者提供的端点：仅 https，且域名解析后不得落在私网/环回/链路本地段（SSRF 防护）。
// 返回解析所得的全部公网 IP，供后续 DialContext 固定拨号（封堵 TOCTOU 重绑定）。
// 私网命中返回 errPrivateHost（handler 映射 403），其余格式问题 400。
func validateGuestAIEndpoint(endpoint string) ([]net.IP, error) {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || u.Host == "" {
		return nil, errors.New("地址格式错误")
	}
	if u.Scheme != "https" {
		return nil, errors.New("仅支持 https 地址")
	}
	ips, err := net.LookupIP(u.Hostname())
	if err != nil || len(ips) == 0 {
		return nil, errors.New("域名无法解析")
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
			return nil, errPrivateHost
		}
	}
	return ips, nil
}

// newGuestAIHTTPClient 构造 SSRF 加固的 guest AI 客户端（Spec 61 阶段 3）：
//   - DialContext 固定拨号校验时解析所得的公网 IP（保留 SNI/证书校验），封堵 TOCTOU DNS 重绑定；
//   - CheckRedirect 直接拒绝，封堵 302 跳转内网的绕过。
func newGuestAIHTTPClient(hostname string, ips []net.IP) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			_, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			var lastErr error
			for _, ip := range ips {
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			if lastErr == nil {
				lastErr = errors.New("无可用 IP")
			}
			return nil, lastErr
		},
		TLSClientConfig:   &tls.Config{ServerName: hostname}, // 保留 SNI 与证书校验
		ForceAttemptHTTP2: true,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("不允许重定向")
		},
	}
}

// errPrivateHost SSRF 私网段哨兵错误（映射 403）。
var errPrivateHost = errors.New("禁止访问内网地址")

// aiGuest 限流：per-IP 滑动窗口 10 次/分钟（内存 map，与 router 登录限流同模式）。
var (
	aiGuestMu   sync.Mutex
	aiGuestHits = map[string][]time.Time{}
)

func aiGuestRateAllow(ip string) bool {
	const limit = 10
	window := time.Minute
	now := time.Now()
	aiGuestMu.Lock()
	defer aiGuestMu.Unlock()
	kept := aiGuestHits[ip][:0]
	for _, t := range aiGuestHits[ip] {
		if now.Sub(t) < window {
			kept = append(kept, t)
		}
	}
	if len(kept) >= limit {
		aiGuestHits[ip] = kept
		return false
	}
	aiGuestHits[ip] = append(kept, now)
	return true
}
