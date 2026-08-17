package identity

import "blog/internal/store"

// Service is the shared identity owner. HTTP handlers use it as the common
// lifecycle boundary while user persistence remains in internal/store.
type Service struct {
	store *store.Store
}

// NewService creates an identity service backed by the shared store.
func NewService(st *store.Store) *Service { return &Service{store: st} }

// Store exposes the persistence owner to internal identity consumers without
// duplicating a second database handle.
func (s *Service) Store() *store.Store {
	if s == nil {
		return nil
	}
	return s.store
}
