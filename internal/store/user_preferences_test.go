package store

import (
	"path/filepath"
	"testing"
)

func newUserPreferenceStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := s.EnsureUserPreferencesTable(); err != nil {
		t.Fatalf("ensure user_preferences table: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUserPreferencesTableIsIdempotent(t *testing.T) {
	s := newUserPreferenceStore(t)
	for i := 0; i < 3; i++ {
		if err := s.EnsureUserPreferencesTable(); err != nil {
			t.Fatalf("ensure user_preferences table (iter %d): %v", i, err)
		}
	}
}

func TestUserPreferencesAreIsolatedAndUpserted(t *testing.T) {
	s := newUserPreferenceStore(t)
	if got, err := s.GetUserPreference(1, "dashboard-widgets"); err != nil || got != nil {
		t.Fatalf("missing preference: got=%+v err=%v", got, err)
	}

	if err := s.PutUserPreference(1, "dashboard-widgets", `{"version":10,"owner":"one"}`); err != nil {
		t.Fatalf("put user 1: %v", err)
	}
	if err := s.PutUserPreference(2, "dashboard-widgets", `{"version":10,"owner":"two"}`); err != nil {
		t.Fatalf("put user 2: %v", err)
	}
	if err := s.PutUserPreference(1, "dashboard-widgets", `{"version":10,"owner":"one-updated"}`); err != nil {
		t.Fatalf("upsert user 1: %v", err)
	}

	first, err := s.GetUserPreference(1, " dashboard-widgets ")
	if err != nil {
		t.Fatalf("get user 1: %v", err)
	}
	if first == nil || first.Value != `{"version":10,"owner":"one-updated"}` || first.UpdatedAt.IsZero() {
		t.Fatalf("user 1 preference = %+v", first)
	}
	second, err := s.GetUserPreference(2, "dashboard-widgets")
	if err != nil {
		t.Fatalf("get user 2: %v", err)
	}
	if second == nil || second.Value != `{"version":10,"owner":"two"}` {
		t.Fatalf("user 2 preference = %+v", second)
	}
	compat, err := s.GetUserPreference(0, "dashboard-widgets")
	if err != nil || compat != nil {
		t.Fatalf("user 0 should be an independent empty bucket: got=%+v err=%v", compat, err)
	}

	if err := s.PutUserPreference(-1, "dashboard-widgets", "{}"); err == nil {
		t.Fatal("negative user id should be rejected")
	}
	if err := s.PutUserPreference(1, " ", "{}"); err == nil {
		t.Fatal("empty preference group should be rejected")
	}
}
