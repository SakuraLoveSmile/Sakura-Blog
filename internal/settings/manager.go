// Package settings owns the shared site configuration state.
package settings

import (
	"sync/atomic"
)

// Manager publishes immutable site snapshots through one small interface. The
// atomic pointer stays private so callers cannot accidentally mutate lifecycle
// state or replace it without the manager contract.
type Manager struct {
	current atomic.Pointer[Site]
	// mirror is transitional glue for callers that still own an atomic site
	// pointer. New services should construct Manager with NewManager.
	mirror *atomic.Pointer[Site]
}

// NewManager creates a settings manager with an initial snapshot.
func NewManager(initial *Site) *Manager {
	m := &Manager{}
	m.Replace(initial)
	return m
}

// NewManagerFromPointer adapts an existing site publisher during migration.
// Current and Replace still go through Manager, while legacy readers observe
// the same published snapshots until they are migrated.
func NewManagerFromPointer(pointer *atomic.Pointer[Site]) *Manager {
	m := &Manager{mirror: pointer}
	if pointer != nil {
		m.current.Store(pointer.Load())
	}
	return m
}

// Current returns the currently published site snapshot.
func (m *Manager) Current() *Site {
	if m == nil {
		return nil
	}
	if m.mirror != nil {
		return m.mirror.Load()
	}
	return m.current.Load()
}

// Replace atomically publishes a new site snapshot.
func (m *Manager) Replace(next *Site) {
	if m == nil {
		return
	}
	if m.mirror != nil {
		m.mirror.Store(next)
		return
	}
	m.current.Store(next)
}
