package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"blog/internal/safefetch"
)

const (
	// DefaultStreamIdleTimeout 是流式响应里两次收到数据之间允许的最长间隔。
	// 推理类模型首个 token 前可能静默很久，所以取值偏宽；任何一行数据（含 SSE 注释）都会重置计时。
	DefaultStreamIdleTimeout = 90 * time.Second

	// streamTotalTimeout 只是一次流式请求的绝对上限，真正的断流判断靠空闲超时。
	streamTotalTimeout = 10 * time.Minute
	// streamMaxLine 限制单行 SSE 数据的长度，防止异常上游撑爆内存。
	streamMaxLine = 1 << 20
	// streamErrorBodyLimit 限制读取非 2xx 响应体的长度。
	streamErrorBodyLimit = 64 << 10
	// streamJSONBodyLimit 限制上游忽略 stream 参数、直接返回整段 JSON 时读取的长度。
	streamJSONBodyLimit = 4 << 20
)

// ErrStreamIdle 表示上游在空闲超时内没有返回任何数据。
var ErrStreamIdle = errors.New("LLM stream produced no data before the idle timeout")

// StreamOptions 调整一次流式对话调用；零值使用与 ChatCompletion 相同的默认温度。
type StreamOptions struct {
	// Temperature 为空时使用 0.2。
	Temperature *float32
	// IdleTimeout 为 0 时使用 DefaultStreamIdleTimeout。
	IdleTimeout time.Duration
	// MaxOutputRunes 大于 0 时限制输出总字数（rune）；达到上限后丢弃剩余内容、结束读取，
	// 并在结果里标记 Truncated，不算错误。
	MaxOutputRunes int
}

// StreamResult 描述一次流式调用的结束状态。
type StreamResult struct {
	// FinishReason 是上游返回的最后一个 finish_reason，可能为空。
	FinishReason string
	// Truncated 表示输出因 MaxOutputRunes 被截断。
	Truncated bool
	// Runes 是已交给 onDelta 的字数（rune）。
	Runes int
}

type streamDelta struct {
	Content json.RawMessage `json:"content"`
}

// text 只接受字符串形态的 content；null、数组等形态一律视为没有文本。
func (d streamDelta) text() string {
	if len(d.Content) == 0 || d.Content[0] != '"' {
		return ""
	}
	var s string
	if err := json.Unmarshal(d.Content, &s); err != nil {
		return ""
	}
	return s
}

type streamChunk struct {
	Choices []struct {
		Delta        streamDelta `json:"delta"`
		FinishReason *string     `json:"finish_reason"`
	} `json:"choices"`
	Error json.RawMessage `json:"error"`
}

// rawErrorMessage 兼容 {"error":{"message":"…"}} 与 {"error":"…"} 两种形态。
func rawErrorMessage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	switch raw[0] {
	case '{':
		var obj struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(raw, &obj) == nil {
			return strings.TrimSpace(obj.Message)
		}
	case '"':
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// upstreamError 把非 2xx 响应体整理成可读错误；解析不出消息时只报状态码。
func upstreamError(status int, body []byte) error {
	var envelope struct {
		Error   json.RawMessage `json:"error"`
		Message string          `json:"message"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		if msg := rawErrorMessage(envelope.Error); msg != "" {
			return errors.New(msg)
		}
		if msg := strings.TrimSpace(envelope.Message); msg != "" {
			return errors.New(msg)
		}
	}
	return fmt.Errorf("HTTP %d", status)
}

// ChatCompletionStream 以 SSE 流式调用 /chat/completions，把每段增量文本交给 onDelta。
//
// 与 ChatCompletion 的区别：不设整体超时，只在 IdleTimeout 内收不到任何数据时中断；
// ctx 取消（例如浏览器断开）会立刻中止上游读取。onDelta 返回错误会中止并原样返回该错误。
// 上游忽略 stream 参数、直接返回整段 JSON 时，按单个增量处理，调用方无需区分。
// 思维链字段（reasoning_content 等）不属于正文，会被忽略。
func (c *Client) ChatCompletionStream(ctx context.Context, cfg Config, messages []Message, opts StreamOptions, onDelta func(string) error) (StreamResult, error) {
	var result StreamResult
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.Model) == "" {
		return result, errors.New("LLM endpoint and model are required")
	}
	if onDelta == nil {
		return result, errors.New("onDelta is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	temperature := float32(0.2)
	if opts.Temperature != nil {
		temperature = *opts.Temperature
	}
	body, err := json.Marshal(struct {
		Model       string    `json:"model"`
		Messages    []Message `json:"messages"`
		Temperature float32   `json:"temperature"`
		Stream      bool      `json:"stream"`
	}{Model: cfg.Model, Messages: messages, Temperature: temperature, Stream: true})
	if err != nil {
		return result, err
	}

	endpoint := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/") + "/chat/completions"
	client := c.HTTPClient
	if client == nil {
		validated, err := safefetch.ValidateURL(endpoint)
		if err != nil {
			return result, err
		}
		endpoint = validated.String()
		client = safefetch.New(safefetch.Options{Timeout: streamTotalTimeout})
	}

	idle := opts.IdleTimeout
	if idle <= 0 {
		idle = DefaultStreamIdleTimeout
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var idled atomic.Bool
	watchdog := time.AfterFunc(idle, func() {
		idled.Store(true)
		cancel()
	})
	defer watchdog.Stop()

	// failure 把空闲超时从普通取消里区分出来，让调用方能给出准确的提示。
	failure := func(err error) error {
		if idled.Load() {
			return ErrStreamIdle
		}
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	if cfg.APIKey != "" {
		request.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	response, err := client.Do(request)
	if err != nil {
		return result, failure(err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(response.Body, streamErrorBodyLimit))
		return result, upstreamError(response.StatusCode, data)
	}

	// emit 统一处理字数上限；返回 stop=true 表示不应再继续读取。
	emit := func(text string) (stop bool, err error) {
		if text == "" {
			return false, nil
		}
		if opts.MaxOutputRunes > 0 {
			remaining := opts.MaxOutputRunes - result.Runes
			if utf8.RuneCountInString(text) > remaining {
				result.Truncated = true
				text = firstRunes(text, remaining)
				stop = true
			}
		}
		if text != "" {
			result.Runes += utf8.RuneCountInString(text)
			if err := onDelta(text); err != nil {
				return true, err
			}
		}
		return stop, nil
	}

	contentType := strings.ToLower(response.Header.Get("Content-Type"))
	if strings.Contains(contentType, "json") && !strings.Contains(contentType, "event-stream") {
		content, err := readJSONContent(response.Body)
		if err != nil {
			return result, failure(err)
		}
		if _, err := emit(content); err != nil {
			return result, err
		}
		return result, nil
	}

	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 0, 64<<10), streamMaxLine)
	for scanner.Scan() {
		watchdog.Reset(idle)
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue // 空行、注释（": ping"）以及 event:/id:/retry: 字段
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		if data == "" || data[0] != '{' {
			continue // 个别供应商会在 data: 里放非 JSON 的心跳
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return result, fmt.Errorf("malformed stream chunk: %w", err)
		}
		if msg := rawErrorMessage(chunk.Error); msg != "" {
			return result, errors.New(msg)
		}
		if len(chunk.Choices) == 0 {
			continue // 仅携带 usage 的收尾块
		}
		choice := chunk.Choices[0]
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			result.FinishReason = *choice.FinishReason
		}
		stop, err := emit(choice.Delta.text())
		if err != nil {
			return result, err
		}
		if stop {
			return result, nil
		}
	}
	if err := scanner.Err(); err != nil {
		if idled.Load() {
			return result, ErrStreamIdle
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return result, ctxErr
		}
		return result, err
	}
	return result, nil
}

// readJSONContent 读取上游无视 stream 参数、一次性返回的完整 JSON，取出首个 choice 的文本。
func readJSONContent(body io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(body, streamJSONBodyLimit))
	if err != nil {
		return "", err
	}
	var parsed chatResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", errors.New("response is not JSON")
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", errors.New(parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("response has no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

// firstRunes 返回 s 的前 n 个 rune；n<=0 时返回空串。
func firstRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	for i := range s {
		if n == 0 {
			return s[:i]
		}
		n--
	}
	return s
}
