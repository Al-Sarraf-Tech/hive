package logs

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jalsarraf0/hive/daemon/internal/container"
)

const defaultBufferCapacity = 10000

// cancelEntry pairs a cancel func with a generation counter so deferred cleanup
// in a goroutine only removes its own entry, not a newer one started by poll().
type cancelEntry struct {
	cancel context.CancelFunc
	gen    uint64
}

// Collector watches managed containers and streams their logs into a ring buffer.
type Collector struct {
	provider container.Provider
	buffer   *RingBuffer
	nodeName string

	mu      sync.Mutex
	active  map[string]cancelEntry // containerID -> cancel + generation
	nextGen uint64
}

// NewCollector creates a log collector that tails all managed containers.
func NewCollector(provider container.Provider, nodeName string) *Collector {
	return &Collector{
		provider: provider,
		buffer:   NewRingBuffer(defaultBufferCapacity),
		nodeName: nodeName,
		active:   make(map[string]cancelEntry),
	}
}

// Buffer returns the ring buffer for reading.
func (c *Collector) Buffer() *RingBuffer {
	return c.buffer
}

// Start begins watching for containers and tailing their logs.
// Blocks until ctx is cancelled.
func (c *Collector) Start(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	slog.Info("log collector started")

	// Run once immediately, then on ticker
	c.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			c.cancelAll()
			return
		case <-ticker.C:
			c.poll(ctx)
		}
	}
}

func (c *Collector) poll(ctx context.Context) {
	containers, err := c.provider.ListContainers(ctx, map[string]string{
		"hive.managed": "true",
	})
	if err != nil {
		slog.Error("log collector: failed to list containers", "error", err)
		return
	}

	// Track which containers are still running
	alive := make(map[string]bool, len(containers))
	for _, ctr := range containers {
		if ctr.Status != "running" {
			continue
		}
		alive[ctr.ID] = true
		c.startTail(ctx, ctr)
	}

	// Cancel goroutines for containers that are gone
	c.mu.Lock()
	for id, entry := range c.active {
		if !alive[id] {
			entry.cancel()
			delete(c.active, id)
		}
	}
	c.mu.Unlock()
}

func (c *Collector) startTail(ctx context.Context, ctr container.ContainerInfo) {
	tailCtx, cancel := context.WithCancel(ctx)

	c.mu.Lock()
	if _, exists := c.active[ctr.ID]; exists {
		c.mu.Unlock()
		cancel()
		return
	}
	c.nextGen++
	gen := c.nextGen
	c.active[ctr.ID] = cancelEntry{cancel: cancel, gen: gen}
	c.mu.Unlock()

	svcName := ctr.Labels["hive.service"]

	go func() {
		defer func() {
			c.mu.Lock()
			// Only delete our own entry — if poll() already replaced it with a
			// newer generation, deleting would orphan the new goroutine.
			if entry, ok := c.active[ctr.ID]; ok && entry.gen == gen {
				delete(c.active, ctr.ID)
			}
			c.mu.Unlock()
		}()

		reader, err := c.provider.Logs(tailCtx, ctr.ID, container.LogOpts{
			Follow:    true,
			TailLines: 100,
		})
		if err != nil {
			slog.Debug("log collector: failed to attach logs", "container", ctr.ID, "error", err)
			return
		}
		defer reader.Close()

		err = container.StreamDockerLogs(reader, func(line string, stream string) error {
			if tailCtx.Err() != nil {
				return tailCtx.Err()
			}
			c.buffer.Push(Entry{
				ServiceName: svcName,
				ContainerID: ctr.ID,
				NodeName:    c.nodeName,
				Line:        line,
				Stream:      stream,
				Timestamp:   time.Now(),
			})
			return nil
		})
		if err != nil && ctx.Err() == nil {
			slog.Debug("log collector: stream ended", "container", ctr.ID, "error", err)
		}
	}()
}

func (c *Collector) cancelAll() {
	c.mu.Lock()
	for id, entry := range c.active {
		entry.cancel()
		delete(c.active, id)
	}
	c.mu.Unlock()
}
