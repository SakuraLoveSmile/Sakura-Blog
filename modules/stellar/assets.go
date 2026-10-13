package stellar

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"blog/modules/stellar/web/assets"
)

// NewAssetVersioner hashes Stellar assets so disk-mode changes invalidate
// browser caches. The implementation stays inside the Stellar module.
func NewAssetVersioner(devMode bool, root string) func(string) string {
	rel := func(p string) string { return strings.TrimPrefix(strings.TrimPrefix(p, "/assets/"), "assets/") }
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
		if devMode {
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
