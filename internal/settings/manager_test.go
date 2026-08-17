package settings

import (
	"sync"
	"testing"
)

func TestManagerCurrentAndReplace(t *testing.T) {
	first := &Site{Title: "first"}
	m := NewManager(first)
	if got := m.Current(); got != first {
		t.Fatalf("Current() = %p, want %p", got, first)
	}
	second := &Site{Title: "second"}
	m.Replace(second)
	if got := m.Current(); got != second {
		t.Fatalf("Current() = %p, want %p", got, second)
	}
}

func TestManagerConcurrentSnapshots(t *testing.T) {
	m := NewManager(&Site{Title: "initial"})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.Replace(&Site{Title: "snapshot"})
			if got := m.Current(); got == nil || got.Title == "" {
				t.Errorf("invalid snapshot: %+v", got)
			}
		}(i)
	}
	wg.Wait()
}
