package module

import "sync"

// registry 全局模块注册表。各模块包在 init() 里调用 Register 自注册，
// main 通过 blank import 触发。注册顺序即遍历顺序（决定菜单/装配顺序）。
var (
	regMu    sync.Mutex
	registry []Module
)

// Register 注册一个模块（模块包 init() 内调用）。
func Register(m Module) {
	regMu.Lock()
	defer regMu.Unlock()
	registry = append(registry, m)
}

// All 返回已注册的全部模块（按注册顺序）。
func All() []Module {
	regMu.Lock()
	defer regMu.Unlock()
	out := make([]Module, len(registry))
	copy(out, registry)
	return out
}
