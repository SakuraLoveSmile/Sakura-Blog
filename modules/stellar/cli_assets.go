package stellar

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"unicode/utf8"

	"blog/internal/settings"
	"blog/modules/stellar/web"
	"blog/modules/stellar/web/assets"
)

// ToolLocalAssets 只读解析公开 uploads、嵌入静态资源与同一图标注册表的派生资产。
// 路径只解码一次；os.Root 将文件访问限制在上传目录，包括符号链接竞态。
func ToolLocalAssets(uploadDir string, icons *settings.IconRegistry) (func(context.Context, string) (io.ReadCloser, error), func(), error) {
	if icons == nil {
		return nil, nil, fmt.Errorf("tool icon registry is required")
	}
	generated, err := web.GeneratedStellarAssets(icons)
	if err != nil {
		return nil, nil, err
	}
	lookup := map[string][]byte{}
	for _, asset := range generated {
		lookup[asset.Path] = asset.Body
		if strings.HasPrefix(asset.Path, "js/icons/") {
			lookup["assets/"+asset.Path] = asset.Body
		}
	}
	var uploads *os.Root
	if uploadDir != "" {
		uploads, err = os.OpenRoot(uploadDir)
		if err != nil && !os.IsNotExist(err) {
			return nil, nil, err
		}
	}
	closeAssets := func() {
		if uploads != nil {
			_ = uploads.Close()
		}
	}
	resolve := func(ctx context.Context, escaped string) (io.ReadCloser, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		lower := strings.ToLower(escaped)
		for _, dangerous := range []string{"%2f", "%5c", "%25", "%00"} {
			if strings.Contains(lower, dangerous) {
				return nil, fmt.Errorf("ambiguous escaped local asset path")
			}
		}
		path, err := url.PathUnescape(escaped)
		if err != nil || !utf8.ValidString(path) || !fs.ValidPath(path) || strings.ContainsAny(path, "\\?#:\x00") {
			return nil, fmt.Errorf("invalid local asset path")
		}
		for _, r := range path {
			if r < 32 {
				return nil, fmt.Errorf("invalid local asset path")
			}
		}
		if b, ok := lookup[path]; ok {
			return io.NopCloser(bytes.NewReader(b)), nil
		}
		if name, ok := strings.CutPrefix(path, "assets/"); ok {
			return assets.FS.Open(name)
		}
		if name, ok := strings.CutPrefix(path, "uploads/"); ok && uploads != nil {
			file, err := uploads.Open(name)
			if err != nil {
				return nil, err
			}
			info, err := file.Stat()
			if err != nil || !info.Mode().IsRegular() {
				file.Close()
				return nil, fmt.Errorf("local upload must be a regular file")
			}
			return file, nil
		}
		return nil, fmt.Errorf("local image route not found: %s", path)
	}
	return resolve, closeAssets, nil
}
