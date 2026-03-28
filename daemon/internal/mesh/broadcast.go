package mesh

import "sync"

// EventBroadcaster distributes mesh events to multiple subscribers.
// Each subscriber gets an independent buffered channel. Events are
// delivered non-blocking — slow subscribers drop events.
type EventBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[uint64]chan MeshEvent
	nextID      uint64
}

func NewEventBroadcaster() *EventBroadcaster {
	return &EventBroadcaster{subscribers: make(map[uint64]chan MeshEvent)}
}

// Subscribe creates a new subscription channel with the given buffer size.
// Returns a unique ID for unsubscription and a read-only channel.
func (b *EventBroadcaster) Subscribe(bufSize int) (uint64, <-chan MeshEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	ch := make(chan MeshEvent, bufSize)
	b.subscribers[b.nextID] = ch
	return b.nextID, ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *EventBroadcaster) Unsubscribe(id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.subscribers[id]; ok {
		close(ch)
		delete(b.subscribers, id)
	}
}

// Broadcast sends an event to all subscribers. Non-blocking — if a
// subscriber's channel is full, the event is dropped for that subscriber.
func (b *EventBroadcaster) Broadcast(event MeshEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// subscriber too slow, drop
		}
	}
}
