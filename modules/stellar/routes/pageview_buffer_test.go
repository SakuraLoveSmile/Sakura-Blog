package routes

import (
	"testing"
	"time"

	"blog/internal/store"
)

// TestPageViewBufferFlushOnClose（Spec 61 阶段 5）：优雅退出时缓冲记录全部落库。
func TestPageViewBufferFlushOnClose(t *testing.T) {
	st, err := store.New(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.EnsurePageViewsTable(); err != nil {
		t.Fatalf("ensure table: %v", err)
	}
	b := newPageViewBuffer(st)
	for i := 0; i < 10; i++ {
		b.push(store.PageView{Path: "/post/a", Referrer: "", UAHash: "h"})
	}
	b.close() // 关闭即最终 flush

	n, err := st.CountPageViews()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 10 {
		t.Fatalf("after close: %d views, want 10", n)
	}
}

// TestPageViewBufferBatchThreshold（Spec 61 阶段 5）：达到批量阈值即落库（不等定时器）。
func TestPageViewBufferBatchThreshold(t *testing.T) {
	st, err := store.New(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.EnsurePageViewsTable(); err != nil {
		t.Fatalf("ensure table: %v", err)
	}
	b := newPageViewBuffer(st)
	defer b.close()
	for i := 0; i < pvBatchSize+5; i++ {
		b.push(store.PageView{Path: "/post/b", Referrer: "", UAHash: "h"})
	}
	// 轮询等待 flush goroutine 落库（不依赖定时器）
	deadline := time.Now().Add(2 * time.Second)
	for {
		n, _ := st.CountPageViews()
		if n >= pvBatchSize {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("batch threshold not flushed in time, count = %d", n)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
