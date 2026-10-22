package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"blog/internal/settings"
	"blog/internal/stellarv2"
	"blog/internal/store"
)

// StellarBlogRuntime 仅用于本地服务，不向前台公开凭据。
type StellarBlogRuntime struct {
	Comments settings.CommentsConfig
	Friends  settings.FriendsConfig
	Pages    settings.PagesConfig
	SEO      settings.SEOConfig
	Authors  map[string]settings.AuthorDef
	AI       AIConfig
}

// 上游已有的 canonical、Open Graph 与 preconnect 留在严格主题字段中。
// 此处只接宿主专属的元信息默认值、站长验证和 robots.txt 规则。
type stellarBlogSEO struct {
	DefaultDescription string `json:"default_description"`
	DefaultKeywords    string `json:"default_keywords"`
	RobotsExtra        string `json:"robots_extra"`
	NoIndex            bool   `json:"no_index"`
	GoogleVerification string `json:"google_verification"`
	BingVerification   string `json:"bing_verification"`
	BaiduVerification  string `json:"baidu_verification"`
}

type stellarBlogAI struct {
	BaseURL        string `json:"base_url"`
	APIKey         string `json:"api_key"`
	Model          string `json:"model"`
	EmbeddingModel string `json:"embedding_model"`
	Summary        struct {
		Enabled      bool `json:"enabled"`
		AllowUserAPI bool `json:"allow_user_api"`
	} `json:"summary"`
	Ask struct {
		Enabled      bool `json:"enabled"`
		AllowUserLLM bool `json:"allow_user_llm"`
	} `json:"ask"`
}

// ParseStellarBlogRuntime 校验真实服务字段，未知的独立扩展保持原样。
// 缺失字段不读取历史分段；所有公开 AI 开关缺省关闭。
func ParseStellarBlogRuntime(blog map[string]any) (StellarBlogRuntime, error) {
	var runtime StellarBlogRuntime
	decode := func(value any, path string, target any) error {
		if err := validateBlogTypedValue(value, reflect.TypeOf(target).Elem(), path); err != nil {
			return err
		}
		raw, err := json.Marshal(value)
		if err != nil || json.Unmarshal(raw, target) != nil {
			return blogRuntimeIssue(path, "valid service configuration", "invalid")
		}
		return nil
	}
	if value, exists := blog["native_comments"]; exists {
		if err := decode(value, "native_comments", &runtime.Comments); err != nil {
			return runtime, err
		}
	}
	if runtime.Comments.Notify.SMTPPort < 0 || runtime.Comments.Notify.SMTPPort > 65535 {
		return runtime, blogRuntimeIssue("native_comments.notify.smtp_port", "integer between 0 and 65535", "number")
	}
	for _, field := range []struct{ path, value string }{
		{"native_comments.admin_email", runtime.Comments.AdminEmail},
		{"native_comments.notify.smtp_host", runtime.Comments.Notify.SMTPHost},
		{"native_comments.notify.username", runtime.Comments.Notify.Username},
		{"native_comments.notify.from", runtime.Comments.Notify.From},
	} {
		if strings.ContainsAny(field.value, "\r\n") {
			return runtime, blogRuntimeIssue(field.path, "string without CR or LF", "string")
		}
	}
	applyCommentDefaults(&runtime.Comments)
	if value, exists := blog["friends"]; exists {
		if err := decode(value, "friends", &runtime.Friends); err != nil {
			return runtime, err
		}
	}
	if value, exists := blog["pages"]; exists {
		if err := decode(value, "pages", &runtime.Pages); err != nil {
			return runtime, err
		}
	}
	var seo stellarBlogSEO
	if value, exists := blog["seo"]; exists {
		if err := decode(value, "seo", &seo); err != nil {
			return runtime, err
		}
	}
	runtime.SEO = settings.SEOConfig{DefaultDescription: seo.DefaultDescription, DefaultKeywords: seo.DefaultKeywords,
		RobotsExtra: seo.RobotsExtra, NoIndex: seo.NoIndex, GoogleVerification: seo.GoogleVerification,
		BingVerification: seo.BingVerification, BaiduVerification: seo.BaiduVerification}
	if value, exists := blog["data"]; exists {
		data, ok := value.(map[string]any)
		if !ok {
			return runtime, blogRuntimeIssue("data", "object", fmt.Sprintf("%T", value))
		}
		if authors, exists := data["authors"]; exists {
			if err := decode(authors, "data.authors", &runtime.Authors); err != nil {
				return runtime, err
			}
		}
	}
	var ai stellarBlogAI
	if value, exists := blog["ai"]; exists {
		if err := decode(value, "ai", &ai); err != nil {
			return runtime, err
		}
	}
	// 开关控制实际公开副作用；只设置 allow_user_api 不能跳过摘要总开关。
	allowUserAPI := ai.Summary.Enabled && ai.Summary.AllowUserAPI
	runtime.AI = AIConfig{BaseURL: ai.BaseURL, APIKey: ai.APIKey, Model: ai.Model, EmbeddingModel: ai.EmbeddingModel,
		SummaryEnabled: &ai.Summary.Enabled, AllowUserAPI: &allowUserAPI, QAEnabled: &ai.Ask.Enabled, QAAllowUserLLM: &ai.Ask.AllowUserLLM}
	return runtime, nil
}

func blogRuntimeIssue(path, expected, actual string) error {
	return &stellarv2.SchemaError{Issues: []stellarv2.Issue{{Code: "invalid_type", Source: "settings.blog_extensions", Path: path, Expected: expected, ActualType: actual, Severity: "error"}}}
}

// 已知 typed 字段必须有正确类型，本地可空布尔按实际服务结构读为关闭。
// 可空指针保留 nil，由消费者决定默认/继承语义，不强制转换成 false。
// 只有两处明确秘密支持 null 清空；未知公开数据仍由各扩展消费。
func validateBlogTypedValue(value any, kind reflect.Type, path string) error {
	if value == nil {
		if kind.Kind() == reflect.Bool || kind.Kind() == reflect.Pointer || path == "ai.api_key" || path == "native_comments.notify.password" {
			return nil
		}
		return blogRuntimeIssue(path, kind.String(), "null")
	}
	switch kind.Kind() {
	case reflect.Struct:
		obj, ok := value.(map[string]any)
		if !ok {
			return blogRuntimeIssue(path, "object", fmt.Sprintf("%T", value))
		}
		for i := 0; i < kind.NumField(); i++ {
			field := kind.Field(i)
			key := strings.Split(field.Tag.Get("json"), ",")[0]
			if key == "" || key == "-" {
				continue
			}
			if item, exists := obj[key]; exists {
				if err := validateBlogTypedValue(item, field.Type, path+"."+key); err != nil {
					return err
				}
			}
		}
	case reflect.Map:
		obj, ok := value.(map[string]any)
		if !ok {
			return blogRuntimeIssue(path, "object", fmt.Sprintf("%T", value))
		}
		for key, item := range obj {
			if err := validateBlogTypedValue(item, kind.Elem(), path+"."+key); err != nil {
				return err
			}
		}
	default:
		raw, err := json.Marshal(value)
		if err != nil || json.Unmarshal(raw, reflect.New(kind).Interface()) != nil {
			return blogRuntimeIssue(path, kind.String(), fmt.Sprintf("%T", value))
		}
	}
	return nil
}

// ApplyStellarBlogRuntime 为启动与热发布构造独立 typed bridge，不改原文数据。
func ApplyStellarBlogRuntime(site *settings.Site) error {
	if site == nil || site.StellarV2 == nil {
		return fmt.Errorf("stellar v2 configuration is missing")
	}
	runtime, err := ParseStellarBlogRuntime(site.StellarV2.Blog)
	if err != nil {
		return err
	}
	site.Comments, site.Friends, site.Authors = runtime.Comments, runtime.Friends, runtime.Authors
	site.Pages, site.SEO = runtime.Pages, runtime.SEO
	return nil
}

// CheckLegacySectionWrite 是生产旧 API 的门禁；历史读取仍可用于迁移预览。
// SaveSection 保留给历史配置工具，产品调用者必须先通过本门禁。
func CheckLegacySectionWrite(st *store.Store, section string) error {
	switch section {
	case SectionSite, SectionDashboard, SectionFeedback, "notify":
		return nil
	}
	if _, err := sectionTarget(section); err != nil {
		return err
	}
	snapshot, err := st.ReadSettingsSnapshot()
	if err != nil {
		return err
	}
	if _, err := LoadStellarV2Snapshot(snapshot); err != nil {
		return err
	}
	return &stellarv2.SchemaError{Issues: []stellarv2.Issue{{Code: "removed_field", Source: "settings", Path: section,
		Expected: "canonical Stellar 2 theme or independent Blog extension via /api/v1/settings/stellar", ActualType: "legacy section", Severity: "error"}}}
}
