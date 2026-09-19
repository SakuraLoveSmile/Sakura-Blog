package store

import "blog/internal/maintenance"

// Maintenance 返回与此业务库关联的唯一维护协调器。
func (s *Store) Maintenance() *maintenance.Coordinator {
	s.maintenanceOnce.Do(func() { s.maintenance = maintenance.New() })
	return s.maintenance
}
