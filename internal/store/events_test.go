package store

import (
	"testing"

	"blog/internal/content"
)

func TestContentEventsPublishOncePerWriteHook(t *testing.T) {
	var got []content.Event
	events := content.NewEvents()
	events.Subscribe(func(event content.Event) { got = append(got, event) })
	s := &Store{}
	s.SetContentEvents(events)
	s.firePostWrite("update", "hello")
	s.firePageWrite("delete", "about")
	s.fireMomentWrite("create", "moment-1")
	if len(got) != 3 {
		t.Fatalf("event count = %d, want 3: %+v", len(got), got)
	}
	if got[0].Kind != content.PostChanged || got[1].Kind != content.PageChanged || got[2].Kind != content.MomentChanged {
		t.Fatalf("event kinds = %+v", got)
	}
}
