package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/goccy/go-yaml"
)

// App 应用级配置（监听地址、DB 路径、seed 开关等），独立于 site.yml。
// 全部字段支持环境变量覆盖：BLOG_LISTEN / BLOG_DB / BLOG_ROOT / BLOG_SEED。
// 优先级：环境变量 > app.yml > 字段默认值。
type App struct {
	ListenAddr   string `yaml:"listen_addr"`   // HTTP 监听地址，默认 :8080
	DBPath       string `yaml:"db_path"`       // SQLite 文件路径，默认 data/blog.db（相对 root）
	Root         string `yaml:"root"`          // 工作根目录，定位 site.yml 与 DB；空则用 cwd
	SeedOnStart  bool   `yaml:"seed_on_start"` // 启动时把 content/posts/*.md 导入 DB（-seed flag 等价）
	TrustedProxy string `yaml:"trusted_proxy"` // 反向代理可信 IP/CIDR，空则不设（仅本机）
}

// DefaultApp 构造默认应用配置。Root 留空由 main.go 用 cwd 兜底。
func DefaultApp() *App {
	return &App{
		ListenAddr: ":8080",
		DBPath:     "data/blog.db",
	}
}

// LoadApp 从 app.yml 加载应用配置，缺失文件返回 DefaultApp（不报错，便于零配置启动）。
// 解析失败才报错。环境变量在 ResolveApp 中覆盖 yaml 值。
func LoadApp(path string) (*App, error) {
	app := DefaultApp()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return app, nil // 无 app.yml 走默认值 + 环境变量
		}
		return nil, fmt.Errorf("read app config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(b, app); err != nil {
		return nil, fmt.Errorf("parse app config %s: %w", path, err)
	}
	return app, nil
}

// ResolveApp 应用环境变量覆盖 + 路径解析。返回处理后的 App。
// 环境变量：BLOG_LISTEN / BLOG_DB / BLOG_ROOT / BLOG_SEED / BLOG_TRUSTED_PROXY。
// DBPath 若是相对路径，按 root 解析成绝对路径（便于 SQLite 父目录创建）。
func ResolveApp(app *App) *App {
	if v := os.Getenv("BLOG_LISTEN"); v != "" {
		app.ListenAddr = v
	}
	if v := os.Getenv("BLOG_DB"); v != "" {
		app.DBPath = v
	}
	if v := os.Getenv("BLOG_ROOT"); v != "" {
		app.Root = v
	}
	if v := os.Getenv("BLOG_SEED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			app.SeedOnStart = b
		}
	}
	if v := os.Getenv("BLOG_TRUSTED_PROXY"); v != "" {
		app.TrustedProxy = v
	}

	// root 绝对化：空则用 cwd
	root := app.Root
	if root == "" {
		root, _ = os.Getwd()
	}
	root, _ = filepath.Abs(root)
	app.Root = root

	// DBPath 绝对化：相对路径按 root 解析
	if !filepath.IsAbs(app.DBPath) {
		app.DBPath = filepath.Join(root, app.DBPath)
	}
	return app
}
