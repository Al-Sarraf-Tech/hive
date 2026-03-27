package logs

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jalsarraf0/hive/daemon/internal/container"
)

const defaultBufferCapacity = 10000

// Collector watches managed containers and streams their logs into a ring buffer.
type Collector struct {
	provider container.Provider
	buffer   *RingBuffer
	nodeName string

	mu     sync.Mutex
	active map[string]context.CancelFunc // containerID -> cancel
}

// NewCollector creates a log collector that tails all managed containers.
func NewCollector(provider container.Provider, nodeName string) *Collector {
	return &Collector{
		provider: provider,
		buffer:   NewRingBuffer(defaultBufferCapacity),
		nodeName: nodeName,
		active:   make(map[string]context.CancelFunc),
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
	for id, cancel := range c.active {
		if !alive[id] {
			cancel()
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
	c.active[ctr.ID] = cancel
	c.mu.Unlock()

	svcName := ctr.Labels["hive.service"]

	go func() {
		defer func() {
			c.mu.Lock()
			delete(c.active, ctr.ID)
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
	for id, cancel := range c.active {
		cancel()
		delete(c.active, id)
	}
	c.mu.Unlock()
}
