package search

import (
	"errors"
	"sync"
	"testing"
)

func TestFailedRebuildBlocksOldIndexUntilSuccessfulCommit(t *testing.T) {
	ix := newTestIndex(t)
	if err := ix.IndexDoc(TypePost, "old", "OldPrivateNeedle", "OldPrivateNeedle"); err != nil {
		t.Fatal(err)
	}
	if _, err := ix.db.Exec(`PRAGMA query_only=ON`); err != nil {
		t.Fatal(err)
	}
	if err := ix.Rebuild([]Item{{Type: TypePost, Slug: "new", Title: "CurrentNeedle", Content: "CurrentNeedle"}}); err == nil {
		t.Fatal("read-only rebuild succeeded")
	}
	if _, err := ix.db.Exec(`PRAGMA query_only=OFF`); err != nil {
		t.Fatal(err)
	}
	var physical int
	if err := ix.db.QueryRow(`SELECT count(*) FROM posts_fts`).Scan(&physical); err != nil || physical != 1 {
		t.Fatalf("failed rebuild did not preserve transaction: %d %v", physical, err)
	}
	for _, query := range []string{"OldPrivateNeedle", ""} {
		if got, err := ix.Search(query, "", 20); !errors.Is(err, ErrUnavailable) || len(got) != 0 {
			t.Fatalf("old search available: %+v %v", got, err)
		}
	}
	if _, err := ix.Count(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unavailable count: %v", err)
	}
	if err := ix.IndexDoc(TypePost, "partial", "Partial", "Partial"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("incremental write restored unusable index: %v", err)
	}
	if err := ix.Unindex(TypePost, "old"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("incremental remove bypassed unavailable gate: %v", err)
	}
	if err := ix.Ensure(); err != nil {
		t.Fatal(err)
	}
	if _, err := ix.Count(); !errors.Is(err, ErrUnavailable) {
		t.Fatal("Ensure restored stale index without rebuild")
	}
	if err := ix.Rebuild([]Item{{Type: TypePost, Slug: "new", Title: "CurrentNeedle", Content: "CurrentNeedle"}}); err != nil {
		t.Fatal(err)
	}
	if got, err := ix.Search("CurrentNeedle", "", 20); err != nil || len(got) != 1 {
		t.Fatalf("full rebuild did not restore search: %+v %v", got, err)
	}
	if got, err := ix.Search("OldPrivateNeedle", "", 20); err != nil || len(got) != 0 {
		t.Fatalf("stale result survived full recovery: %+v %v", got, err)
	}
}

func TestUnavailableAndRebuildSynchronizeWithSearchConsumers(t *testing.T) {
	ix := newTestIndex(t)
	items := []Item{{Type: TypePost, Slug: "current", Title: "CurrentNeedle", Content: "CurrentNeedle"}}
	if err := ix.Rebuild(items); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	failures := make(chan error, 240)
	for range 4 {
		workers.Go(func() {
			for range 20 {
				got, err := ix.Search("CurrentNeedle", "", 20)
				if err != nil && !errors.Is(err, ErrUnavailable) {
					failures <- err
				}
				if err == nil && len(got) != 1 {
					failures <- errors.New("partial index became visible")
				}
				if _, err := ix.Count(); err != nil && !errors.Is(err, ErrUnavailable) {
					failures <- err
				}
			}
		})
	}
	for range 10 {
		ix.SetUnavailable()
		if err := ix.Rebuild(items); err != nil {
			t.Fatal(err)
		}
	}
	workers.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
}
