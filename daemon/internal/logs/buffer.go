// Package logs provides a ring buffer for aggregating container log lines.
package logs

import (
	"sync"
	"time"
)

// Entry is a single log line from a container.
type Entry struct {
	ID          int       `json:"id"` // monotonic sequence number
	ServiceName string    `json:"service_name"`
	ContainerID string    `json:"container_id"`
	NodeName    string    `json:"node_name"`
	Line        string    `json:"line"`
	Stream      string    `json:"stream"` // "stdout" or "stderr"
	Timestamp   time.Time `json:"timestamp"`
}

// RingBuffer is a fixed-capacity, thread-safe ring buffer of log entries.
type RingBuffer struct {
	mu      sync.RWMutex
	entries []Entry
	head    int
	count   int
	cap     int
	nextID  int // monotonic counter for entry IDs
}

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 100
	}
	return &RingBuffer{
		entries: make([]Entry, capacity),
		cap:     capacity,
	}
}

// Push adds an entry to the ring buffer, evicting the oldest if full.
func (rb *RingBuffer) Push(e Entry) {
	rb.mu.Lock()
	rb.nextID++
	e.ID = rb.nextID
	rb.entries[rb.head] = e
	rb.head = (rb.head + 1) % rb.cap
	if rb.count < rb.cap {
		rb.count++
	}
	rb.mu.Unlock()
}

// Last returns the most recent n entries (oldest first).
func (rb *RingBuffer) Last(n int) []Entry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n > rb.count {
		n = rb.count
	}
	if n == 0 {
		return nil
	}

	result := make([]Entry, n)
	start := (rb.head - n + rb.cap) % rb.cap
	for i := 0; i < n; i++ {
		result[i] = rb.entries[(start+i)%rb.cap]
	}
	return result
}

// ForService returns the most recent n entries for a specific service (oldest first).
func (rb *RingBuffer) ForService(name string, n int) []Entry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	// Walk backwards from head, collecting matching entries
	var matches []Entry
	for i := 0; i < rb.count && len(matches) < n; i++ {
		idx := (rb.head - 1 - i + rb.cap) % rb.cap
		if rb.entries[idx].ServiceName == name {
			matches = append(matches, rb.entries[idx])
		}
	}

	// Reverse to oldest-first order
	for i, j := 0, len(matches)-1; i < j; i, j = i+1, j-1 {
		matches[i], matches[j] = matches[j], matches[i]
	}
	return matches
}

// Since returns all entries with ID > lastID, optionally filtered by service name.
// Results are returned in oldest-first order.
func (rb *RingBuffer) Since(lastID int, service string) []Entry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	var result []Entry
	for i := 0; i < rb.count; i++ {
		idx := (rb.head - rb.count + i + rb.cap) % rb.cap
		e := rb.entries[idx]
		if e.ID <= lastID {
			continue
		}
		if service != "" && e.ServiceName != service {
			continue
		}
		result = append(result, e)
	}
	return result
}
