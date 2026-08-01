package module

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// Manager 持有模块启停状态（内存 + DB 持久化），供 Gate 中间件与 manifest API 使用。
type Manager struct {
	store   *store.Store
	mu      sync.RWMutex
	enabled map[string]bool // 模块 id -> 是否启用（已合并 DB 覆盖与 DefaultOn）
	metas   []Meta          // 注册顺序快照
}

// NewManager 从注册表 + DB 状态构建 Manager。无 DB 记录的模块回退 Meta.DefaultOn。
func NewManager(st *store.Store) (*Manager, error) {
	states, err := st.GetModuleStates()
	if err != nil {
		return nil, err
	}
	m := &Manager{store: st, enabled: map[string]bool{}}
	for _, mod := range All() {
		meta := mod.Meta()
		m.metas = append(m.metas, meta)
		if v, ok := states[meta.ID]; ok {
			m.enabled[meta.ID] = v
		} else {
			m.enabled[meta.ID] = meta.DefaultOn
		}
	}
	return m, nil
}

// Enabled 返回模块当前是否启用。
func (m *Manager) Enabled(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled[id]
}

// SetEnabled 更新启停状态并持久化。
func (m *Manager) SetEnabled(id string, enabled bool) error {
	m.mu.Lock()
	m.enabled[id] = enabled
	m.mu.Unlock()
	return m.store.SetModuleState(id, enabled)
}

// Has 返回是否存在该 id 的已注册模块。
func (m *Manager) Has(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, meta := range m.metas {
		if meta.ID == id {
			return true
		}
	}
	return false
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
