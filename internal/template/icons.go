package template

import (
	"bufio"
	_ "embed"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
)

// iconsYML 是编译期嵌入的 Stellar icons.yml（从 resource/ 复制到 data/，避免部署时依赖 gitignore 的 resource/）。
// 路径相对本包：data/icons.yml。
//
//go:embed data/icons.yml
var iconsYML []byte

// IconRegistry 从 Stellar _data/icons.yml 加载的内联 SVG 集合。
// icons.yml 格式为 flat map：每行 "key: <svg...>"，非标准 YAML（value 含大量特殊字符），
// 故用正则按行解析而非 yaml.Unmarshal。
type IconRegistry struct {
	mu   sync.RWMutex
	icon map[string]string
}

var iconLineRe = regexp.MustCompile(`^([a-zA-Z0-9:_-]+):\s*(.+)$`)

// LoadIcons 从指定路径加载 icons.yml，返回 IconRegistry。
// 保留给开发期从 resource/ 读取（go run -dev 时定位）；正式部署用 LoadIconsEmbed。
func LoadIcons(path string) (*IconRegistry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open icons.yml: %w", err)
	}
	defer f.Close()
	return parseIcons(f)
}

// LoadIconsEmbed 从 go:embed 嵌入的 icons.yml 加载。部署时无需外部 resource/ 目录。
func LoadIconsEmbed() (*IconRegistry, error) {
	return parseIcons(strings.NewReader(string(iconsYML)))
}

// parseIcons 从任意 reader 解析 icons.yml，逻辑与原 LoadIcons 一致。
func parseIcons(r io.Reader) (*IconRegistry, error) {
	reg := &IconRegistry{icon: make(map[string]string)}
	sc := bufio.NewScanner(r)
	// 单行 SVG 最长 ~3.4KB，默认 buffer 64KB 足够
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "#") || strings.TrimSpace(line) == "" {
			continue
		}
		m := iconLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		reg.icon[m[1]] = strings.TrimSpace(m[2])
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan icons.yml: %w", err)
	}
	return reg, nil
}

// Get 返回 key 对应的内联 SVG；未找到时返回占位 span，避免菜单出现空图标。
func (r *IconRegistry) Get(key string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if svg, ok := r.icon[key]; ok {
		return svg
	}
	return fmt.Sprintf(`<span class="icon-missing" data-icon="%s"></span>`, key)
}

// Has 判断 key 是否存在。
func (r *IconRegistry) Has(key string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.icon[key]
	return ok
}

// Len 返回已加载的图标数量。
func (r *IconRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.icon)
}
