package maintenance

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOperationLockConflictsAndRejectsSymlinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operation.lock")
	release, err := LockOperation(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LockOperation(path); !errors.Is(err, ErrBusy) {
		t.Fatalf("second operation got %v", err)
	}
	release()
	release, err = LockOperation(path)
	if err != nil {
		t.Fatal(err)
	}
	release()
	link := filepath.Join(t.TempDir(), "lock")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := LockOperation(link); err == nil {
		t.Fatal("followed lock symlink")
	}
}

func TestServiceUserCanTakeExistingOperationLock(t *testing.T) {
	path := os.Getenv("BLOG_TEST_OPERATION_LOCK")
	if path == "" {
		t.Skip("requires isolated systemd updater-first fixture")
	}
	release, err := LockOperation(path)
	if err != nil {
		t.Fatal(err)
	}
	release()
}
