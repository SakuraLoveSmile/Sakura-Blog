// Package storage exposes the attachment storage backend as a compile-time
// module with a database-backed, schema-driven configuration surface.
package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/module"
	attachmentstorage "blog/internal/storage"
	"blog/internal/store"
)

const settingKey = "storage"

type Module struct{}

func init() { module.Register(&Module{}) }

func (m *Module) Meta() module.Meta {
	return module.Meta{
		ID:            "storage",
		Title:         "附件存储",
		Description:   "配置本地或 S3-compatible 附件存储与公开 URL",
		Category:      "系统",
		DefaultOn:     true,
		Locked:        true,
		ConfigSection: settingKey,
		ConfigSchema: []module.ConfigField{
			{Key: "provider", Type: "select", Label: "存储后端", LabelKey: "schema.storage.provider", Default: attachmentstorage.ProviderLocal, Options: []string{attachmentstorage.ProviderLocal, attachmentstorage.ProviderS3}, Hint: "S3 兼容后端包括 AWS S3、MinIO 与 OSS 的 S3 网关。"},
			{Key: "endpoint", Type: "text", Label: "Endpoint", LabelKey: "schema.storage.endpoint", Placeholder: "https://s3.example.com", Hint: "仅允许 http(s) URL，不要填写路径、查询参数或账号密码。"},
			{Key: "region", Type: "text", Label: "Region", LabelKey: "schema.storage.region", Default: "us-east-1"},
			{Key: "bucket", Type: "text", Label: "Bucket", LabelKey: "schema.storage.bucket"},
			{Key: "access_key", Type: "text", Label: "Access key", LabelKey: "schema.storage.access_key"},
			{Key: "secret_key", Type: "password", Label: "Secret key", LabelKey: "schema.storage.secret_key", Placeholder: "留空表示保持原值", Hint: "已保存的密钥不会回显。"},
			{Key: "public_url", Type: "text", Label: "公开 URL", LabelKey: "schema.storage.public_url", Placeholder: "https://cdn.example.com/blog", Hint: "可选；填写 CDN 或桶的公开 URL 前缀。"},
			{Key: "path_style", Type: "switch", Label: "使用 path-style", LabelKey: "schema.storage.path_style", Default: false, Hint: "部分 S3 网关或 OSS 兼容服务需要开启。"},
			{Key: "prefix", Type: "text", Label: "对象前缀", LabelKey: "schema.storage.prefix", Placeholder: "attachments", Hint: "可选；用于把附件放进桶内的独立目录。"},
		},
	}
}

func (m *Module) Migrate(deps module.Deps) error {
	return deps.Store.EnsureSettingsTable()
}

func (m *Module) RegisterRoutes(authed *gin.RouterGroup, deps module.Deps) {
	module.MountConfigRoutes(authed, deps, m)
}

func (m *Module) RegisterPublic(*gin.Engine, module.Deps) {}

// NewManager loads the persisted configuration. A missing storage row means
// local storage at the application's configured upload directory.
func NewManager(st *store.Store, localRoot string) (*attachmentstorage.Manager, error) {
	cfg := attachmentstorage.DefaultConfig()
	raw, err := st.GetSetting(settingKey)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return nil, fmt.Errorf("decode storage config: %w", err)
		}
	}
	return attachmentstorage.NewManager(localRoot, cfg)
}

func (m *Module) ConfigSection() string { return settingKey }

func (m *Module) GetConfig(deps module.Deps) (any, error) {
	if deps.Storage == nil {
		return nil, errors.New("storage manager is not initialized")
	}
	cfg := deps.Storage.Config()
	if cfg.SecretKey != "" {
		cfg.SecretKey = attachmentstorage.MaskValue
	}
	return cfg, nil
}

func (m *Module) PutConfig(deps module.Deps, raw []byte) error {
	if deps.Storage == nil {
		return errors.New("storage manager is not initialized")
	}
	var incoming attachmentstorage.Config
	if err := json.Unmarshal(raw, &incoming); err != nil {
		return fmt.Errorf("配置格式错误: %w", err)
	}
	existing := deps.Storage.Config()
	if incoming.SecretKey == "" || incoming.SecretKey == attachmentstorage.MaskValue {
		incoming.SecretKey = existing.SecretKey
	}
	if incoming.Provider == "" {
		incoming.Provider = existing.Provider
	}
	if incoming.Region == "" {
		incoming.Region = existing.Region
	}
	if incoming.Provider == attachmentstorage.ProviderS3 && incoming.AccessKey == "" {
		incoming.AccessKey = existing.AccessKey
	}
	if err := deps.Storage.Reconfigure(incoming); err != nil {
		return err
	}
	stored := deps.Storage.Config()
	encoded, err := json.Marshal(stored)
	if err != nil {
		_ = deps.Storage.Reconfigure(existing)
		return fmt.Errorf("encode storage config: %w", err)
	}
	if err := deps.Store.PutSetting(settingKey, string(encoded)); err != nil {
		_ = deps.Storage.Reconfigure(existing)
		return fmt.Errorf("persist storage config: %w", err)
	}
	return nil
}

var _ module.Module = (*Module)(nil)
var _ module.ConfigModule = (*Module)(nil)
