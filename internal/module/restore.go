package module

import (
	"errors"
	"fmt"
)

// ReloadRestoredState 重载已恢复的模块状态；调用方须持维护窗口。
// 只记录已执行的生命周期，使随后补偿不会反向操作尚未处理的模块。
func (m *Manager) ReloadRestoredState(deps Deps) error {
	mods := make([]Module, 0, len(m.metas))
	for _, meta := range m.metas {
		mods = append(mods, m.mods[meta.ID])
	}
	next, err := newManagerWith(m.store, mods)
	if err != nil {
		return err
	}
	for _, meta := range m.metas {
		previous := m.Enabled(meta.ID)
		target := next.enabled[meta.ID]
		if previous == target {
			continue
		}
		lifecycle, ok := m.mods[meta.ID].(LifecycleModule)
		if ok {
			if target {
				err = lifecycle.OnEnable(deps)
			} else {
				err = lifecycle.OnDisable(deps)
			}
			if err != nil {
				var rollbackErr error
				if target {
					rollbackErr = lifecycle.OnDisable(deps)
				} else {
					rollbackErr = lifecycle.OnEnable(deps)
				}
				if rollbackErr != nil {
					m.mu.Lock()
					m.enabled[meta.ID] = target
					m.mu.Unlock()
				}
				return fmt.Errorf("restore module %s lifecycle: %w", meta.ID, errors.Join(err, rollbackErr))
			}
		}
		m.mu.Lock()
		m.enabled[meta.ID] = target
		m.mu.Unlock()
	}
	return nil
}
