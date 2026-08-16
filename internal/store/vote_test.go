package store

import (
	"path/filepath"
	"testing"
)

func newTestStoreVote(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := s.EnsureVotesTable(); err != nil {
		t.Fatalf("ensure votes table: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestVoteUpdateAndGet(t *testing.T) {
	s := newTestStoreVote(t)
	if err := s.UpdateVote("post-1", "up", "hash-a"); err != nil {
		t.Fatalf("update vote: %v", err)
	}
	if err := s.UpdateVote("post-1", "down", "hash-b"); err != nil {
		t.Fatalf("update vote: %v", err)
	}
	vc, err := s.GetVotes("post-1")
	if err != nil {
		t.Fatalf("get votes: %v", err)
	}
	if vc.Up != 1 || vc.Down != 1 {
		t.Fatalf("counts = %+v, want up=1 down=1", vc)
	}
}

func TestVoteSameHashOverwrites(t *testing.T) {
	s := newTestStoreVote(t)
	// 同 hash 同 target 覆盖更新：up → down 后 up 归零
	if err := s.UpdateVote("post-1", "up", "hash-a"); err != nil {
		t.Fatalf("first vote: %v", err)
	}
	if err := s.UpdateVote("post-1", "down", "hash-a"); err != nil {
		t.Fatalf("overwrite vote: %v", err)
	}
	vc, _ := s.GetVotes("post-1")
	if vc.Up != 0 || vc.Down != 1 {
		t.Fatalf("counts = %+v, want up=0 down=1", vc)
	}
	// 不同 target 互不影响
	if err := s.UpdateVote("post-2", "up", "hash-a"); err != nil {
		t.Fatalf("vote other target: %v", err)
	}
	vc2, _ := s.GetVotes("post-2")
	if vc2.Up != 1 || vc2.Down != 0 {
		t.Fatalf("counts = %+v, want up=1 down=0", vc2)
	}
}

func TestVoteInvalidValue(t *testing.T) {
	s := newTestStoreVote(t)
	if err := s.UpdateVote("post-1", "sideways", "hash-a"); err == nil {
		t.Fatal("expected error for invalid value, got nil")
	}
}

func TestVoteEmptyTarget(t *testing.T) {
	s := newTestStoreVote(t)
	vc, err := s.GetVotes("no-such-target")
	if err != nil {
		t.Fatalf("get votes: %v", err)
	}
	if vc.Up != 0 || vc.Down != 0 {
		t.Fatalf("counts = %+v, want zero", vc)
	}
}
