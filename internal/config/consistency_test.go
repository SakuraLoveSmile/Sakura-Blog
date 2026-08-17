package config

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/goccy/go-yaml"
)

// TestExampleConfigKeyConsistency 机械校验 config/*.example.yml 与真值配置的键位一致性。
// 背景：app.yml/site.yml 含密钥不入库（.gitignore），行为配置变更失去 diff 与 review；
// 本测试保证 example 与真值的键结构不漂移——任一侧新增/删除键而另一侧未同步时，
// make test（go test ./...）即失败。只比较键名，不读取/比较值，密钥内容不进入断言输出。
// 真值文件不存在时跳过该组（fresh clone / CI 场景无真值配置）。
func TestExampleConfigKeyConsistency(t *testing.T) {
	configDir := filepath.Join("..", "..", "config")
	pairs := []struct {
		example string
		real    string
	}{
		{"app.example.yml", "app.yml"},
		{"site.example.yml", "site.yml"},
	}

	for _, p := range pairs {
		t.Run(p.real, func(t *testing.T) {
			examplePath := filepath.Join(configDir, p.example)
			realPath := filepath.Join(configDir, p.real)

			exampleKeys, err := yamlKeyPaths(examplePath)
			if err != nil {
				t.Fatalf("load example config %s: %v", p.example, err)
			}
			if _, err := os.Stat(realPath); os.IsNotExist(err) {
				t.Skipf("%s 不存在（fresh clone / CI），跳过键位校验", p.real)
			}
			realKeys, err := yamlKeyPaths(realPath)
			if err != nil {
				t.Fatalf("load real config %s: %v", p.real, err)
			}

			if missing := diffKeys(exampleKeys, realKeys); len(missing) > 0 {
				t.Errorf("%s 缺少 %s 中的键（真值配置未同步 example 新增项）: %v", p.real, p.example, missing)
			}
			if extra := diffKeys(realKeys, exampleKeys); len(extra) > 0 {
				t.Errorf("%s 缺少 %s 中的键（example 模板未同步真值新增项，行为变更失去 review 可见性）: %v", p.example, p.real, extra)
			}
		})
	}
}

// yamlKeyPaths 解析 yaml 文件并收集全部映射键路径（如 comments.notify.smtp_host）。
func yamlKeyPaths(path string) (map[string]bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc any
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	keys := map[string]bool{}
	collectKeyPaths("", doc, keys)
	return keys, nil
}

// collectKeyPaths 递归收集映射键路径。序列视为叶子：列表元素（wiki/links/menubar 条目等）
// 属于站点数据而非配置结构，条目数量与内部字段随内容变化，不纳入键位一致性比较。
func collectKeyPaths(prefix string, node any, out map[string]bool) {
	switch m := node.(type) {
	case map[string]any:
		for k, v := range m {
			p := k
			if prefix != "" {
				p = prefix + "." + k
			}
			out[p] = true
			collectKeyPaths(p, v, out)
		}
	case map[any]any:
		for k, v := range m {
			ks, ok := k.(string)
			if !ok {
				continue
			}
			p := ks
			if prefix != "" {
				p = prefix + "." + ks
			}
			out[p] = true
			collectKeyPaths(p, v, out)
		}
	}
}

// diffKeys 返回存在于 a 但不存在于 b 的键路径（排序保证输出稳定）。
func diffKeys(a, b map[string]bool) []string {
	var missing []string
	for k := range a {
		if !b[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	return missing
}
