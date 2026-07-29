package main

import (
	"path/filepath"
	"testing"

	"blog/internal/og"
	"blog/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestSetupSearchIndex(t *testing.T) {
	st := newTestStore(t)
	idx := setupSearchIndex(st)
	if idx == nil {
		t.Fatal("fts index should be available on fresh sqlite store")
	}
	if n, err := idx.Count(); err != nil || n != 0 {
		t.Errorf("fresh index should be empty: n=%d err=%v", n, err)
	}
}

func TestNewPostWriteHookNilSearchIndex(t *testing.T) {
	st := newTestStore(t)
	ogGen := og.New("", t.TempDir())
	hook := newPostWriteHook(st, nil, ogGen)
	// searchIdx 降级为 nil 时钩子不 panic（只做 OG 失效）
	hook("update", "some-slug")
	hook("delete", "some-slug")
}
