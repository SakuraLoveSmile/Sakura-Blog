package stellar

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"blog/internal/settings"
	"blog/internal/stellarhtml"
	"blog/modules/stellar/web"
	"blog/modules/stellar/web/assets"
)

// NewAssetVersioner hashes Stellar assets so disk-mode changes invalidate
// browser caches. The implementation stays inside the Stellar module.
func NewAssetVersioner(devMode bool, root string, generated ...stellarhtml.Asset) func(string) string {
	rel := func(p string) string { return strings.TrimPrefix(strings.TrimPrefix(p, "/assets/"), "assets/") }
	generated = append([]stellarhtml.Asset(nil), generated...)
	sort.Slice(generated, func(i, j int) bool {
		if generated[i].Path != generated[j].Path {
			return generated[i].Path < generated[j].Path
		}
		return bytes.Compare(generated[i].Body, generated[j].Body) < 0
	})
	generatedBodies := make(map[string][]byte, len(generated))
	var generatedData []byte
	for _, asset := range generated {
		generatedBodies[asset.Path] = append([]byte(nil), asset.Body...)
		generatedData = append(generatedData, []byte(asset.Path+"\x00")...)
		generatedData = append(generatedData, asset.Body...)
		generatedData = append(generatedData, 0)
	}
	var mu sync.Mutex
	cache := map[string]string{}
	return func(p string) string {
		if !devMode {
			mu.Lock()
			if v, ok := cache[p]; ok {
				mu.Unlock()
				return v
			}
			mu.Unlock()
		}
		var data []byte
		var err error
		if body, ok := generatedBodies[rel(p)]; ok {
			data = body
		} else if devMode {
			data, err = os.ReadFile(filepath.Join(root, "modules", "stellar", "web", "assets", rel(p)))
		} else {
			data, err = assets.FS.ReadFile(rel(p))
		}
		if err != nil {
			return "0"
		}
		// ESM、动态脚本/样式和延迟图标沿用入口 query，必须聚合本地加载资源。
		if rel(p) == "js/runtime/index.js" {
			var source fs.FS = assets.FS
			if devMode {
				source = os.DirFS(filepath.Join(root, "modules", "stellar", "web", "assets"))
			}
			data = nil
			err = fs.WalkDir(source, ".", func(name string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() || !(strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".css") ||
					(strings.HasPrefix(name, "js/icons/") && strings.HasSuffix(name, ".json"))) {
					return nil
				}
				content, readErr := fs.ReadFile(source, name)
				if readErr != nil {
					return readErr
				}
				data = append(data, []byte(name+"\x00")...)
				data = append(data, content...)
				data = append(data, 0)
				return nil
			})
			if err != nil {
				return "0"
			}
			data = append(data, generatedData...)
		}
		sum := sha256.Sum256(data)
		version := hex.EncodeToString(sum[:])[:8]
		if !devMode {
			mu.Lock()
			cache[p] = version
			mu.Unlock()
		}
		return version
	}
}

// newIconAssetVersioner 与公开路由使用同一图标派生器，不从磁盘猜测生产注册表。
func newIconAssetVersioner(devMode bool, root string, icons *settings.IconRegistry) (func(string) string, error) {
	if icons == nil {
		return nil, fmt.Errorf("stellar asset versioner: icon registry is required")
	}
	generated, err := web.GeneratedStellarAssets(icons)
	if err != nil {
		return nil, fmt.Errorf("generate stellar assets for versioning: %w", err)
	}
	return NewAssetVersioner(devMode, root, generated...), nil
}
