package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"blog/internal/config"
	"blog/internal/settings"
	"blog/internal/stellarv2"

	"github.com/gin-gonic/gin"
)

// StellarSettingsDocument 是完整公开配置，主题使用上游 snake_case 键。
// 数值保持 JSON 原始精度，不经过 Console 的结构化表单转换。
type StellarSettingsDocument struct {
	Version        int            `json:"version"`
	Theme          map[string]any `json:"theme"`
	BlogExtensions map[string]any `json:"blog_extensions"`
}

type StellarSettingsIssue struct {
	Code       string `json:"code"`
	Source     string `json:"source"`
	Path       string `json:"path"`
	Expected   string `json:"expected,omitempty"`
	ActualType string `json:"actual_type,omitempty"`
	Severity   string `json:"severity,omitempty"`
}

type StellarSettingsMigration struct {
	Source   string   `json:"source"`
	Path     string   `json:"path"`
	Reason   string   `json:"reason"`
	Sections []string `json:"sections"`
}

type StellarSettingsProblem struct {
	Type      string                    `json:"type"`
	Title     string                    `json:"title"`
	Status    int                       `json:"status"`
	Detail    string                    `json:"detail"`
	Error     string                    `json:"error"`
	Instance  string                    `json:"instance"`
	Issues    []StellarSettingsIssue    `json:"issues"`
	Migration *StellarSettingsMigration `json:"migration,omitempty"`
}

type StellarSettingsValidationResponse struct {
	Valid  bool                   `json:"valid"`
	Issues []StellarSettingsIssue `json:"issues"`
}

func (h *Handler) getStellarSettings(c *gin.Context) {
	loaded, err := config.LoadStellarV2(h.store)
	if err != nil {
		stellarSettingsError(c, err, http.StatusInternalServerError)
		return
	}
	stellarSettingsDocument(c, loaded)
}

func (h *Handler) updateStellarSettings(c *gin.Context) {
	document, ok := readStellarSettingsDocument(c)
	if !ok {
		return
	}
	saved, err := config.SaveAndPublishStellarV2(h.store, h.site, document.Theme, document.BlogExtensions)
	if err != nil {
		stellarSettingsError(c, err, http.StatusBadRequest)
		return
	}
	h.publishContentChanged()
	if h.onLinksChanged != nil {
		h.onLinksChanged()
	}
	stellarSettingsDocument(c, saved)
}

// 校验始终只读，旧库也能检查候选，不初始化或迁移真实配置。
func (h *Handler) validateStellarSettings(c *gin.Context) {
	document, ok := readStellarSettingsDocument(c)
	if !ok {
		return
	}
	validated, err := config.ValidateStellarV2(document.Theme, document.BlogExtensions)
	if err != nil {
		stellarSettingsError(c, err, http.StatusBadRequest)
		return
	}
	parsed, err := stellarv2.ParseTheme(validated.RawTheme, stellarv2.Options{Source: "settings.stellar_v2.theme"})
	if err != nil {
		stellarSettingsError(c, err, http.StatusBadRequest)
		return
	}
	c.JSON(http.StatusOK, StellarSettingsValidationResponse{Valid: true, Issues: stellarSettingsIssues(parsed.Issues)})
}

func readStellarSettingsDocument(c *gin.Context) (StellarSettingsDocument, bool) {
	var document StellarSettingsDocument
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSettingsBodyBytes)
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		status := http.StatusBadRequest
		if errors.As(err, &tooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		stellarSettingsError(c, fmt.Errorf("读取 Stellar 2 配置失败：%w", err), status)
		return document, false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&document)
	if err == nil {
		if trailing := decoder.Decode(new(any)); trailing != io.EOF {
			if trailing == nil {
				err = fmt.Errorf("expected one JSON document")
			} else {
				err = trailing
			}
		}
	}
	if err != nil {
		stellarSettingsError(c, fmt.Errorf("stellar 2 配置 JSON 无效：%w", err), http.StatusBadRequest)
		return document, false
	}
	if document.Version != config.StellarV2Version || document.Theme == nil || document.BlogExtensions == nil {
		stellarSettingsError(c, fmt.Errorf("配置须包含 version: 2、theme 对象和 blog_extensions 对象"), http.StatusBadRequest)
		return document, false
	}
	return document, true
}

func stellarSettingsDocument(c *gin.Context, loaded *settings.StellarV2Config) {
	blog, err := config.MaskStellarV2Blog(loaded.Blog)
	if err != nil {
		stellarSettingsError(c, err, http.StatusInternalServerError)
		return
	}
	c.IndentedJSON(http.StatusOK, StellarSettingsDocument{Version: config.StellarV2Version, Theme: loaded.RawTheme, BlogExtensions: blog})
}

func stellarSettingsIssues(issues []stellarv2.Issue) []StellarSettingsIssue {
	out := make([]StellarSettingsIssue, 0, len(issues))
	for _, issue := range issues {
		out = append(out, StellarSettingsIssue{Code: issue.Code, Source: issue.Source, Path: issue.Path, Expected: issue.Expected, ActualType: issue.ActualType, Severity: issue.Severity})
	}
	return out
}

func stellarSettingsError(c *gin.Context, err error, status int) {
	problem := StellarSettingsProblem{Type: problemType(status), Title: http.StatusText(status), Status: status, Detail: err.Error(), Error: err.Error(), Instance: c.Request.URL.Path, Issues: []StellarSettingsIssue{}}
	var migration *config.MigrationRequiredError
	var schema *stellarv2.SchemaError
	if errors.As(err, &migration) {
		problem.Type, problem.Status, problem.Title = "stellar_migration_required", http.StatusConflict, "需要先迁移 Stellar 配置"
		problem.Migration = &StellarSettingsMigration{Source: migration.Source, Path: migration.Path, Reason: migration.Reason, Sections: migration.Sections}
	} else if errors.As(err, &schema) {
		problem.Type, problem.Status, problem.Title = "stellar_schema_error", http.StatusBadRequest, "Stellar 配置不符合上游规则"
		problem.Issues = stellarSettingsIssues(schema.Issues)
	}
	c.JSON(problem.Status, problem)
}
