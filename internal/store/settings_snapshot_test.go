package store

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func newSettingsSnapshotStore(t *testing.T) *Store {
	t.Helper()
	st, err := New(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.EnsureSettingsTable(); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestSettingsSnapshotRollbackOnSecondWriteFailure(t *testing.T) {
	st := newSettingsSnapshotStore(t)
	before := map[string]string{"a": `{"old":true}`, "z": `{}`, "unrelated": `{"keep":true}`}
	if err := st.PutSettingsSnapshot(before); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`CREATE TRIGGER reject_z BEFORE UPDATE ON settings WHEN NEW.section = 'z' BEGIN SELECT RAISE(ABORT, 'forced failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := st.PutSettingsSnapshot(map[string]string{"a": `{"new":true}`, "z": `{"new":true}`}); err == nil {
		t.Fatal("expected injected second write failure")
	}
	after, err := st.ReadSettingsSnapshot()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("transaction did not roll back all segments: %v %v", after, err)
	}
}

func TestSettingsSnapshotValidationAndGuardBeforeWrite(t *testing.T) {
	st := newSettingsSnapshotStore(t)
	before := map[string]string{"a": "{}"}
	if err := st.PutSettingsSnapshot(before); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []map[string]string{{"a": `{"changed":true}`, "z": "invalid"}, {" ": "{}"}} {
		if err := st.PutSettingsSnapshot(candidate); err == nil {
			t.Fatal("invalid candidate accepted")
		}
	}
	wantError := errors.New("guard rejected")
	err := st.UpdateSettingsSnapshot(map[string]string{"a": `{"changed":true}`}, func(snapshot map[string]string) error {
		if !reflect.DeepEqual(snapshot, before) {
			t.Fatalf("guard did not receive existing snapshot: %v", snapshot)
		}
		return wantError
	})
	if !errors.Is(err, wantError) {
		t.Fatalf("guard error lost: %v", err)
	}
	after, _ := st.ReadSettingsSnapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("validation/guard failure wrote settings")
	}
}
