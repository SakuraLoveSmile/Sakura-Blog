// Package storage defines the attachment persistence boundary and its built-in
// local/S3-compatible implementations.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	ProviderLocal = "local"
	ProviderS3    = "s3"
	MaskValue     = "****"
)

// AttachmentStorage stores attachment bytes under a relative path.
type AttachmentStorage interface {
	Save(relativePath string, data []byte) error
	Thumb(relativePath string, data []byte) error
	Remove(relativePath string) error
}

// URLProvider maps a stored relative path to the URL returned to Console and
// content clients. Local storage uses the application's /uploads route;
// object storage uses public_url or an endpoint-derived URL.
type URLProvider interface {
	URL(relativePath string) string
}

// Config is the persisted storage configuration. SecretKey is never returned
// directly by the storage ConfigModule; it is kept here so the runtime can
// rebuild an S3 client without another process restart.
type Config struct {
	Provider  string `json:"provider"`
	Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	Bucket    string `json:"bucket"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	PublicURL string `json:"public_url"`
	PathStyle bool   `json:"path_style"`
	Prefix    string `json:"prefix"`
}

func DefaultConfig() Config {
	return Config{Provider: ProviderLocal, Region: "us-east-1"}
}

// ValidateConfig normalizes harmless formatting while rejecting incomplete or
// unsafe object-storage configuration before a client is constructed.
func ValidateConfig(cfg Config) (Config, error) {
	cfg.Provider = strings.ToLower(strings.TrimSpace(cfg.Provider))
	if cfg.Provider == "" {
		cfg.Provider = ProviderLocal
	}
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.AccessKey = strings.TrimSpace(cfg.AccessKey)
	cfg.SecretKey = strings.TrimSpace(cfg.SecretKey)
	cfg.PublicURL = strings.TrimRight(strings.TrimSpace(cfg.PublicURL), "/")
	prefix, err := cleanRelativePath(cfg.Prefix, true)
	if err != nil {
		return Config{}, fmt.Errorf("invalid storage prefix: %w", err)
	}
	cfg.Prefix = prefix

	switch cfg.Provider {
	case ProviderLocal:
		return cfg, nil
	case ProviderS3:
		if cfg.Endpoint == "" {
			return Config{}, errors.New("s3 endpoint is required")
		}
		if _, err := parseEndpoint(cfg.Endpoint); err != nil {
			return Config{}, fmt.Errorf("invalid s3 endpoint: %w", err)
		}
		if cfg.Bucket == "" {
			return Config{}, errors.New("s3 bucket is required")
		}
		if cfg.AccessKey == "" || cfg.SecretKey == "" {
			return Config{}, errors.New("s3 access key and secret key are required")
		}
		if cfg.Region == "" {
			cfg.Region = "us-east-1"
		}
		if cfg.PublicURL != "" {
			if _, err := parsePublicURL(cfg.PublicURL); err != nil {
				return Config{}, fmt.Errorf("invalid public url: %w", err)
			}
		}
		return cfg, nil
	default:
		return Config{}, fmt.Errorf("unsupported storage provider %q", cfg.Provider)
	}
}

// Manager is a stable AttachmentStorage pointer whose backend can be swapped
// after a ConfigModule update. Existing handlers keep using the same manager.
type Manager struct {
	mu        sync.RWMutex
	localRoot string
	config    Config
	backend   AttachmentStorage
}

func NewManager(localRoot string, cfg Config) (*Manager, error) {
	if strings.TrimSpace(localRoot) == "" {
		return nil, errors.New("local attachment root is required")
	}
	normalized, err := ValidateConfig(cfg)
	if err != nil {
		return nil, err
	}
	backend, err := newBackend(localRoot, normalized)
	if err != nil {
		return nil, err
	}
	return &Manager{localRoot: localRoot, config: normalized, backend: backend}, nil
}

func (m *Manager) Config() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// Reconfigure constructs the new backend before swapping it into service, so
// a malformed S3 endpoint cannot interrupt the current backend.
func (m *Manager) Reconfigure(cfg Config) error {
	normalized, err := ValidateConfig(cfg)
	if err != nil {
		return err
	}
	backend, err := newBackend(m.localRoot, normalized)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.config = normalized
	m.backend = backend
	m.mu.Unlock()
	return nil
}

func (m *Manager) current() AttachmentStorage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.backend
}

func (m *Manager) Save(relativePath string, data []byte) error {
	return m.current().Save(relativePath, data)
}

func (m *Manager) Thumb(relativePath string, data []byte) error {
	return m.current().Thumb(relativePath, data)
}

func (m *Manager) Remove(relativePath string) error {
	return m.current().Remove(relativePath)
}

func (m *Manager) URL(relativePath string) string {
	m.mu.RLock()
	backend := m.backend
	m.mu.RUnlock()
	if provider, ok := backend.(URLProvider); ok {
		return provider.URL(relativePath)
	}
	return "/uploads/" + strings.TrimLeft(strings.ReplaceAll(relativePath, "\\", "/"), "/")
}

func newBackend(localRoot string, cfg Config) (AttachmentStorage, error) {
	if cfg.Provider == ProviderS3 {
		return NewS3AttachmentStorage(cfg)
	}
	return NewLocalAttachmentStorage(localRoot), nil
}

type localAttachmentStorage struct{ root string }

func NewLocalAttachmentStorage(root string) AttachmentStorage {
	return &localAttachmentStorage{root: root}
}

func (s *localAttachmentStorage) path(relativePath string) (string, error) {
	clean, err := cleanRelativePath(relativePath, false)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.root, filepath.FromSlash(clean)), nil
}

func (s *localAttachmentStorage) Save(relativePath string, data []byte) error {
	return s.write(relativePath, data)
}

func (s *localAttachmentStorage) Thumb(relativePath string, data []byte) error {
	return s.write(relativePath, data)
}

func (s *localAttachmentStorage) write(relativePath string, data []byte) error {
	filePath, err := s.path(relativePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0o644)
}

func (s *localAttachmentStorage) Remove(relativePath string) error {
	filePath, err := s.path(relativePath)
	if err != nil {
		return err
	}
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *localAttachmentStorage) URL(relativePath string) string {
	clean, err := cleanRelativePath(relativePath, false)
	if err != nil {
		return ""
	}
	return "/uploads/" + clean
}

// S3AttachmentStorage is compatible with AWS S3, MinIO, and OSS-style S3
// gateways. It intentionally uses the provider's object API only; public URL
// construction is kept separate so a CDN/custom domain can be configured.
type S3AttachmentStorage struct {
	client    *minio.Client
	bucket    string
	prefix    string
	endpoint  *url.URL
	publicURL string
	pathStyle bool
}

func NewS3AttachmentStorage(cfg Config) (*S3AttachmentStorage, error) {
	normalized, err := ValidateConfig(cfg)
	if err != nil {
		return nil, err
	}
	endpoint, err := parseEndpoint(normalized.Endpoint)
	if err != nil {
		return nil, err
	}
	bucketLookup := minio.BucketLookupAuto
	if normalized.PathStyle {
		bucketLookup = minio.BucketLookupPath
	}
	client, err := minio.New(endpoint.Host, &minio.Options{
		Creds:        credentials.NewStaticV4(normalized.AccessKey, normalized.SecretKey, ""),
		Secure:       endpoint.Scheme == "https",
		Region:       normalized.Region,
		BucketLookup: bucketLookup,
	})
	if err != nil {
		return nil, fmt.Errorf("create s3 client: %w", err)
	}
	return &S3AttachmentStorage{
		client: client, bucket: normalized.Bucket, prefix: normalized.Prefix,
		endpoint: endpoint, publicURL: normalized.PublicURL, pathStyle: normalized.PathStyle,
	}, nil
}

func (s *S3AttachmentStorage) objectKey(relativePath string) (string, error) {
	clean, err := cleanRelativePath(relativePath, false)
	if err != nil {
		return "", err
	}
	if s.prefix == "" {
		return clean, nil
	}
	return s.prefix + "/" + clean, nil
}

func (s *S3AttachmentStorage) Save(relativePath string, data []byte) error {
	return s.put(relativePath, data)
}

func (s *S3AttachmentStorage) Thumb(relativePath string, data []byte) error {
	return s.put(relativePath, data)
}

func (s *S3AttachmentStorage) put(relativePath string, data []byte) error {
	key, err := s.objectKey(relativePath)
	if err != nil {
		return err
	}
	contentType := mime.TypeByExtension(path.Ext(relativePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err = s.client.PutObject(
		context.Background(), s.bucket, key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: contentType},
	)
	return err
}

func (s *S3AttachmentStorage) Remove(relativePath string) error {
	key, err := s.objectKey(relativePath)
	if err != nil {
		return err
	}
	return s.client.RemoveObject(context.Background(), s.bucket, key, minio.RemoveObjectOptions{})
}

func (s *S3AttachmentStorage) URL(relativePath string) string {
	key, err := s.objectKey(relativePath)
	if err != nil {
		return ""
	}
	if s.publicURL != "" {
		return strings.TrimRight(s.publicURL, "/") + "/" + escapeObjectKey(key)
	}
	if s.pathStyle {
		return s.endpoint.Scheme + "://" + s.endpoint.Host + "/" + url.PathEscape(s.bucket) + "/" + escapeObjectKey(key)
	}
	return s.endpoint.Scheme + "://" + url.PathEscape(s.bucket) + "." + s.endpoint.Host + "/" + escapeObjectKey(key)
}

func escapeObjectKey(key string) string {
	parts := strings.Split(key, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func parseEndpoint(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("endpoint must be an http(s) URL without path, query, or credentials")
	}
	return u, nil
}

func parsePublicURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("public url must be an http(s) URL")
	}
	return u, nil
}

func cleanRelativePath(raw string, allowEmpty bool) (string, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if raw == "" {
		if allowEmpty {
			return "", nil
		}
		return "", errors.New("storage path is empty")
	}
	if strings.HasPrefix(raw, "/") || strings.Contains(raw, "\x00") {
		return "", errors.New("storage path must be relative")
	}
	for _, part := range strings.Split(raw, "/") {
		if part == ".." {
			return "", errors.New("storage path traversal is not allowed")
		}
	}
	clean := path.Clean(raw)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("storage path traversal is not allowed")
	}
	return strings.TrimPrefix(clean, "./"), nil
}

var _ AttachmentStorage = (*Manager)(nil)
var _ URLProvider = (*Manager)(nil)
var _ AttachmentStorage = (*S3AttachmentStorage)(nil)
var _ URLProvider = (*S3AttachmentStorage)(nil)
