package stellar

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"runtime"

	"blog/internal/stellarcli"
	"blog/modules/stellar/web"
)

//go:embed web/build/stylus/upstream.json
var toolUpstream []byte

// ToolEnvironment 报告真实 Go 运行版本及二进制嵌入资源指纹，不伪造 Hexo 环境。
func ToolEnvironment() (stellarcli.Environment, error) {
	var upstream struct{ Tag, Commit string }
	if err := json.Unmarshal(toolUpstream, &upstream); err != nil {
		return stellarcli.Environment{}, err
	}
	hash := sha256.New()
	if err := fs.WalkDir(web.FS, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		b, err := fs.ReadFile(web.FS, path)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(hash, "%s\x00%d\x00", path, len(b)); err != nil {
			return err
		}
		_, err = hash.Write(b)
		return err
	}); err != nil {
		return stellarcli.Environment{}, err
	}
	return stellarcli.Environment{Adapter: "go-ssr-sqlite", Go: runtime.Version(), RequiredGo: "go1.26.6", Theme: upstream.Tag, Commit: upstream.Commit, ResourceSHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}
