package module

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// Manager 持有模块启停状态（内存 + DB 持久化），供 Gate 中间件与 manifest API 使用。
// 依赖声明 / 生命周期钩子 / 配置命名空间经类型断言按需生效，旧模块零改动兼容。
type Manager struct {
	store   *store.Store
	mu      sync.RWMutex
	enabled map[string]bool // 模块 id -> 是否启用（已合并 DB 覆盖与 DefaultOn）
	metas   []Meta          // 注册顺序快照
	mods    map[string]Module
}

// NewManager 从注册表 + DB 状态构建 Manager。无 DB 记录的模块回退 Meta.DefaultOn。
// 构建时做拓扑校验（依赖已注册且已启用、无循环），失败返回含缺失清单的错误。
func NewManager(st *store.Store) (*Manager, error) {
	return newManagerWith(st, All())
}

// newManagerWith 用给定模块列表构建 Manager（不读全局注册表，测试注入用）。
func newManagerWith(st *store.Store, mods []Module) (*Manager, error) {
	states, err := st.GetModuleStates()
	if err != nil {
		return nil, err
	}
	m := &Manager{store: st, enabled: map[string]bool{}, mods: map[string]Module{}}
	for _, mod := range mods {
		meta := mod.Meta()
		m.metas = append(m.metas, meta)
		m.mods[meta.ID] = mod
		if v, ok := states[meta.ID]; ok {
			m.enabled[meta.ID] = v
		} else {
			m.enabled[meta.ID] = meta.DefaultOn
		}
	}
	if err := m.validateLocked(); err != nil {
		return nil, err
	}
	return m, nil
}

// Enabled 返回模块当前是否启用。
func (m *Manager) Enabled(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled[id]
}

// SetEnabled 更新启停状态并持久化，deps 供生命周期钩子使用。
// 启用：依赖必须已注册且已启用，先 OnEnable 后落库，失败回滚（含已调钩子逆序回滚）；
// 禁用：仍被其他已启用模块依赖时拒绝，OnDisable 失败不改变状态。
func (m *Manager) SetEnabled(id string, enabled bool, deps Deps) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.mods[id]; !ok {
		return fmt.Errorf("模块 %s 未注册", id)
	}
	if !enabled && m.isLocked(id) {
		return fmt.Errorf("模块 %s 不可停用", id)
	}
	if m.enabled[id] == enabled {
		return nil // 状态未变，跳过钩子
	}
	if enabled {
		return m.enableLocked(id, deps)
	}
	return m.disableLocked(id, deps)
}

func (m *Manager) isLocked(id string) bool {
	for _, meta := range m.metas {
		if meta.ID == id {
			return meta.Locked
		}
	}
	return false
}

// enableLocked 启用（调用方持写锁）。
func (m *Manager) enableLocked(id string, deps Deps) error {
	for _, dep := range DependenciesOf(m.mods[id]) {
		if _, ok := m.mods[dep]; !ok {
			return fmt.Errorf("启用 %s 失败: 依赖未注册: %s", id, dep)
		}
		if !m.enabled[dep] {
			return fmt.Errorf("启用 %s 失败: 依赖未启用: %s", id, dep)
		}
	}
	lm, hasHook := m.mods[id].(LifecycleModule)
	if hasHook {
		if err := lm.OnEnable(deps); err != nil {
			return fmt.Errorf("启用 %s 失败: %w", id, err)
		}
	}
	m.enabled[id] = true
	if err := m.store.SetModuleState(id, true); err != nil {
		m.enabled[id] = false
		if hasHook {
			_ = lm.OnDisable(deps) // 回滚 OnEnable 副作用
		}
		return fmt.Errorf("持久化启用状态 %s: %w", id, err)
	}
	return nil
}

// disableLocked 禁用（调用方持写锁）。
func (m *Manager) disableLocked(id string, deps Deps) error {
	for oid, om := range m.mods {
		if oid == id || !m.enabled[oid] {
			continue
		}
		for _, dep := range DependenciesOf(om) {
			if dep == id {
				return fmt.Errorf("禁用 %s 失败: 仍被已启用模块 %s 依赖", id, oid)
			}
		}
	}
	lm, hasHook := m.mods[id].(LifecycleModule)
	if hasHook {
		if err := lm.OnDisable(deps); err != nil {
			return fmt.Errorf("禁用 %s 失败: %w", id, err)
		}
	}
	m.enabled[id] = false
	if err := m.store.SetModuleState(id, false); err != nil {
		m.enabled[id] = true
		if hasHook {
			_ = lm.OnEnable(deps) // 尽力回滚 OnDisable 副作用
		}
		return fmt.Errorf("持久化禁用状态 %s: %w", id, err)
	}
	return nil
}

// Validate 拓扑校验：已启用模块的依赖必须已注册且已启用，且依赖图无环。
// 返回错误含缺失/未启用/循环清单（启动时由 NewManager 内部调用，运行时可复查）。
func (m *Manager) Validate() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.validateLocked()
}

// validateLocked 拓扑校验实现（调用方持读锁）。
func (m *Manager) validateLocked() error {
	var problems []string
	for id, mod := range m.mods {
		if !m.enabled[id] {
			continue
		}
		for _, dep := range DependenciesOf(mod) {
			if _, ok := m.mods[dep]; !ok {
				problems = append(problems, fmt.Sprintf("模块 %s 依赖未注册的模块 %s", id, dep))
				continue
			}
			if !m.enabled[dep] {
				problems = append(problems, fmt.Sprintf("模块 %s 依赖未启用的模块 %s", id, dep))
			}
		}
	}
	for id := range m.mods {
		if m.enabled[id] {
			if cyc := m.cycleFromLocked(id); len(cyc) > 1 {
				problems = append(problems, fmt.Sprintf("模块依赖存在循环: %s", strings.Join(cyc, " -> ")))
			}
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("模块依赖校验失败: %s", strings.Join(problems, "; "))
	}
	return nil
}

// cycleFromLocked 从 start 出发沿依赖图（仅已启用模块）DFS，返回环路径（无环 nil）。
func (m *Manager) cycleFromLocked(start string) []string {
	visiting := map[string]bool{}
	var dfs func(id string) []string
	dfs = func(id string) []string {
		mod, ok := m.mods[id]
		if !ok || !m.enabled[id] {
			return nil
		}
		if visiting[id] {
			return []string{id}
		}
		visiting[id] = true
		defer delete(visiting, id)
		for _, dep := range DependenciesOf(mod) {
			if r := dfs(dep); r != nil {
				return append([]string{id}, r...)
			}
		}
		return nil
	}
	return dfs(start)
}

// Has 返回是否存在该 id 的已注册模块。
func (m *Manager) Has(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.mods[id]
	return ok
}

// ManifestItem manifest API 的单个模块条目。
type ManifestItem struct {
	Meta
	Enabled bool `json:"enabled"`
}

// List 返回全部模块 meta + 当前启停（注册顺序），供 manifest API。
func (m *Manager) List() []ManifestItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ManifestItem, 0, len(m.metas))
	for _, meta := range m.metas {
		out = append(out, ManifestItem{Meta: meta, Enabled: m.enabled[meta.ID]})
	}
	return out
}

// Gate 返回中间件：模块禁用时对其 API 返回 403 JSON（运行时开关，无需重注册路由）。
func (m *Manager) Gate(id string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.Enabled(id) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "模块未启用"})
			return
		}
		c.Next()
	}
}
