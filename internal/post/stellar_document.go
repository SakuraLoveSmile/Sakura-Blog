package post

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"blog/internal/stellarv2"
	"github.com/goccy/go-yaml"
)

// StellarDocument 保留作者原始配置与正文，Page 是严格解析后的上游运行时配置。
// 独立 Blog 扩展不传入上游 schema；新导入路径不能复用旧 FileMeta 的平铺字段。
type StellarDocument struct {
	Raw    map[string]any    `json:"raw"`
	Page   map[string]any    `json:"page"`
	Blog   map[string]any    `json:"blog"`
	Body   string            `json:"body"`
	Issues []stellarv2.Issue `json:"issues"`
}

type StellarDocumentOptions struct {
	Source   string
	Profile  string
	Kind     string
	PagePath string
	Registry *stellarv2.CollectionRegistry
}

// ParseStellarDocument 对新导入使用 strict，不把旧 topic/pin/style 等当作迁移。
// 支持 Hexo 的 YAML 与 JSON front matter；无 front matter 时正文逐字保留。
func ParseStellarDocument(src []byte, options StellarDocumentOptions) (StellarDocument, error) {
	raw := map[string]any{}
	body := string(src)
	content := bytes.TrimPrefix(src, []byte{0xef, 0xbb, 0xbf})
	first, rest, found := bytes.Cut(content, []byte("\n"))
	delimiter := strings.TrimSuffix(string(first), "\r")
	if delimiter == "---" || delimiter == ";;;" {
		if !found {
			return StellarDocument{}, fmt.Errorf("%s: front matter is not closed", options.Source)
		}
		var header []byte
		closed := false
		for len(rest) > 0 {
			line, next, newline := bytes.Cut(rest, []byte("\n"))
			if strings.TrimSuffix(string(line), "\r") == delimiter {
				body = string(next)
				closed = true
				break
			}
			header = append(header, line...)
			if newline {
				header = append(header, '\n')
			}
			rest = next
		}
		if !closed {
			return StellarDocument{}, fmt.Errorf("%s: front matter is not closed", options.Source)
		}
		if len(bytes.TrimSpace(header)) > 0 {
			if delimiter == "---" {
				if err := yaml.Unmarshal(header, &raw); err != nil {
					return StellarDocument{}, fmt.Errorf("%s: invalid YAML front matter: %w", options.Source, err)
				}
			} else {
				decoder := json.NewDecoder(bytes.NewReader(header))
				decoder.UseNumber()
				if err := decoder.Decode(&raw); err != nil {
					return StellarDocument{}, fmt.Errorf("%s: invalid JSON front matter: %w", options.Source, err)
				}
				if err := decoder.Decode(new(any)); err != io.EOF {
					return StellarDocument{}, fmt.Errorf("%s: front matter must contain one JSON object", options.Source)
				}
			}
			if raw == nil {
				return StellarDocument{}, fmt.Errorf("%s: front matter must be an object", options.Source)
			}
		}
	}
	// 将 YAML 的数值统一到 JSON number，保留开放参数袋的大整数。
	serialized, err := json.Marshal(raw)
	if err != nil {
		return StellarDocument{}, fmt.Errorf("%s: front matter is not JSON compatible: %w", options.Source, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(serialized))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return StellarDocument{}, err
	}
	blog := map[string]any{}
	if extension, exists := raw["blog"]; exists {
		var ok bool
		blog, ok = extension.(map[string]any)
		if !ok {
			return StellarDocument{}, fmt.Errorf("%s: blog extension must be an object", options.Source)
		}
		delete(raw, "blog")
	}
	parsed, err := stellarv2.ParsePage(raw, stellarv2.Options{Source: options.Source})
	if err != nil {
		return StellarDocument{}, err
	}
	result := StellarDocument{Raw: raw, Page: parsed.Value, Blog: blog, Body: body, Issues: parsed.Issues}
	if options.Registry != nil {
		membership := stellarv2.ResolveContentMembership(stellarv2.MembershipInput{Kind: options.Kind, Source: options.Source, PagePath: options.PagePath, Config: result.Page, Registry: options.Registry})
		result.Issues = append(result.Issues, membership.Issues...)
		if membership.Config == nil {
			return StellarDocument{}, &stellarv2.SchemaError{Issues: membership.Issues}
		}
		result.Page = membership.Config
	}
	if options.Profile != "" {
		profile := stellarv2.ValidatePageProfile(result.Page, options.Source, options.Profile)
		result.Page = profile.Value
		result.Issues = append(result.Issues, profile.Issues...)
	}
	return result, nil
}

// SerializeStellarDocument 导出原始 snake_case 配置，避免把 runtime camelCase 写回作者文件。
func SerializeStellarDocument(document StellarDocument) ([]byte, error) {
	raw := make(map[string]any, len(document.Raw)+1)
	for key, value := range document.Raw {
		raw[key] = value
	}
	if len(document.Blog) > 0 {
		raw["blog"] = document.Blog
	}
	// 导出前复验上游部分；不会通过导出重建来静默修复未知字段。
	if _, err := stellarv2.ParsePage(document.Raw, stellarv2.Options{Source: "export"}); err != nil {
		return nil, err
	}
	// Hexo 原生 JSON front matter 避免 YAML 编码器将 json.Number 写成字符串，
	// 也不将开放参数袋的大整数缩为 float64。
	header, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode front matter: %w", err)
	}
	return []byte(";;;\n" + string(header) + "\n;;;\n" + document.Body), nil
}
