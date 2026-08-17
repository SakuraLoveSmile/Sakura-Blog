package stellar

import (
	"crypto/sha256"
	"encoding/hex"
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
