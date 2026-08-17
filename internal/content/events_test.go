package content

import (
	"sync"
	"testing"
)

func TestEventsPublishOnceAndUnsubscribe(t *testing.T) {
	events := NewEvents()
	var mu sync.Mutex
	var got []Event
	sub := events.Subscribe(func(event Event) {
		mu.Lock()
		got = append(got, event)
		mu.Unlock()
	})
	events.Publish(Event{Kind: PostChanged, Op: "update", Slug: "hello"})
	if len(got) != 1 || got[0].Kind != PostChanged || got[0].Slug != "hello" {
		t.Fatalf("events = %+v", got)
	}
	sub.Close()
	sub.Close()
	events.Publish(Event{Kind: Restored})
	if len(got) != 1 {
		t.Fatalf("closed subscription received event: %+v", got)
	}
}
