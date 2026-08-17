package content

import (
	"sync"
	"time"
)

// EventKind identifies the source of a content-derived cache invalidation.
type EventKind string

const (
	PostChanged    EventKind = "post.changed"
	PageChanged    EventKind = "page.changed"
	MomentChanged  EventKind = "moment.changed"
	ContentChanged EventKind = "content.changed"
	Restored       EventKind = "content.restored"
)

// Event is emitted once after a successful content write or full restore.
type Event struct {
	Kind EventKind
	Op   string
	Slug string
	At   time.Time
}

// Subscriber receives content events synchronously in publication order.
type Subscriber func(Event)

// Subscription removes one subscriber. Close is idempotent.
type Subscription interface{ Close() }

// Events is the shared content change bus consumed by search, OG, sitemap and
// Stellar cache owners.
type Events interface {
	Publish(Event)
	Subscribe(Subscriber) Subscription
}

type bus struct {
	mu     sync.RWMutex
	nextID uint64
	subs   map[uint64]Subscriber
}

// NewEvents creates an in-process content event bus.
func NewEvents() Events { return &bus{subs: make(map[uint64]Subscriber)} }

func (b *bus) Publish(event Event) {
	if event.At.IsZero() {
		event.At = time.Now()
	}
	b.mu.RLock()
	subs := make([]Subscriber, 0, len(b.subs))
	for _, sub := range b.subs {
		subs = append(subs, sub)
	}
	b.mu.RUnlock()
	for _, sub := range subs {
		sub(event)
	}
}

func (b *bus) Subscribe(sub Subscriber) Subscription {
	if sub == nil {
		return noopSubscription{}
	}
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	b.subs[id] = sub
	b.mu.Unlock()
	return &subscription{bus: b, id: id}
}

type subscription struct {
	once sync.Once
	bus  *bus
	id   uint64
}

func (s *subscription) Close() {
	if s == nil || s.bus == nil {
		return
	}
	s.once.Do(func() {
		s.bus.mu.Lock()
		delete(s.bus.subs, s.id)
		s.bus.mu.Unlock()
	})
}

type noopSubscription struct{}

func (noopSubscription) Close() {}
