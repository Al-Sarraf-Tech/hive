package health

import (
	"sync"
	"time"
)

// HealthEvent records the result of a single health check.
type HealthEvent struct {
	Timestamp           time.Time
	Healthy             bool
	Message             string
	DurationMs          int32
	CheckType           string
	ConsecutiveFailures int32
}

// serviceHistory is a ring buffer of health events for one service.
type serviceHistory struct {
	entries []HealthEvent
	head    int
	count   int
	cap     int
}

// History stores health check history for all services.
// Thread-safe via read-write mutex.
type History struct {
	mu       sync.RWMutex
	services map[string]*serviceHistory
	eventCap int
}

// NewHistory creates a History that retains up to eventCapPerService events per service.
func NewHistory(eventCapPerService int) *History {
	if eventCapPerService <= 0 {
		eventCapPerService = 100
	}
	return &History{
		services: make(map[string]*serviceHistory),
		eventCap: eventCapPerService,
	}
}

// Record appends a health event for the named service.
func (h *History) Record(serviceName string, event HealthEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	sh, ok := h.services[serviceName]
	if !ok {
		sh = &serviceHistory{
			entries: make([]HealthEvent, h.eventCap),
			cap:     h.eventCap,
		}
		h.services[serviceName] = sh
	}

	sh.entries[sh.head] = event
	sh.head = (sh.head + 1) % sh.cap
	if sh.count < sh.cap {
		sh.count++
	}
}

// Get returns up to limit health events for the named service, oldest first.
// If limit <= 0, returns all stored events.
func (h *History) Get(serviceName string, limit int) []HealthEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	sh, ok := h.services[serviceName]
	if !ok || sh.count == 0 {
		return nil
	}

	n := sh.count
	if limit > 0 && limit < n {
		n = limit
	}

	result := make([]HealthEvent, n)
	// Start index: oldest entry in the ring buffer, offset to return only the last n entries
	start := (sh.head - sh.count + sh.cap) % sh.cap
	// If we're returning fewer than all, skip older entries
	skip := sh.count - n
	start = (start + skip) % sh.cap

	for i := 0; i < n; i++ {
		result[i] = sh.entries[(start+i)%sh.cap]
	}
	return result
}

// CurrentState returns the current health status for a service based on the most recent event.
func (h *History) CurrentState(serviceName string) (healthy bool, consecutiveFailures int) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	sh, ok := h.services[serviceName]
	if !ok || sh.count == 0 {
		return false, 0
	}

	// Most recent entry is at head-1
	latest := sh.entries[(sh.head-1+sh.cap)%sh.cap]
	return latest.Healthy, int(latest.ConsecutiveFailures)
}
