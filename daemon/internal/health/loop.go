package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/jalsarraf0/hive/daemon/internal/container"
	"github.com/jalsarraf0/hive/daemon/internal/hivefile"
	"github.com/jalsarraf0/hive/daemon/internal/metrics"
	"github.com/jalsarraf0/hive/daemon/internal/store"
)

// Loop runs periodic health checks on all local services and auto-restarts failed containers.
type Loop struct {
	checker              *Checker
	container            container.Provider
	store                *store.Store
	interval             time.Duration
	failures             map[string]int
	stopCh               chan struct{}
	stopOnce             sync.Once
	onContainerCountFunc func(int) // callback to update container count in gossip metadata
}

// NewLoop creates a health check loop.
// onContainerCount is called each tick with the current running container count (may be nil).
func NewLoop(checker *Checker, c container.Provider, s *store.Store, interval time.Duration, onContainerCount func(int)) *Loop {
	return &Loop{
		checker:              checker,
		container:            c,
		store:                s,
		interval:             interval,
		failures:             make(map[string]int),
		stopCh:               make(chan struct{}),
		onContainerCountFunc: onContainerCount,
	}
}

// Start begins the health check loop. Blocks until Stop is called or ctx is cancelled.
func (l *Loop) Start(ctx context.Context) {
	ticker := time.NewTicker(l.interval)
	defer ticker.Stop()

	slog.Info("health check loop started", "interval", l.interval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-l.stopCh:
			return
		case <-ticker.C:
			l.runChecks(ctx)
		}
	}
}

// Stop signals the loop to stop. Safe to call once.
func (l *Loop) Stop() {
	l.stopOnce.Do(func() {
		close(l.stopCh)
	})
}

func (l *Loop) runChecks(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	// List all managed containers
	containers, err := l.container.ListContainers(ctx, map[string]string{
		"hive.managed": "true",
	})
	if err != nil {
		slog.Error("health loop: failed to list containers", "error", err)
		return
	}

	// Update container count in gossip metadata for spread scoring
	runningCount := 0
	for _, c := range containers {
		if c.Status == "running" {
			runningCount++
		}
	}
	metrics.ContainerCount.Set(float64(runningCount))
	if l.onContainerCountFunc != nil {
		l.onContainerCountFunc(runningCount)
	}

	for _, c := range containers {
		if c.Status != "running" {
			continue
		}

		svcName := c.Labels["hive.service"]
		if svcName == "" {
			continue
		}

		// Load service definition to get health check config
		svcData, err := l.store.Get("services", svcName)
		if err != nil || svcData == nil {
			continue
		}

		var svcDef hivefile.ServiceDef
		if err := json.Unmarshal(svcData, &svcDef); err != nil {
			continue
		}

		if svcDef.Health.Type == "" || svcDef.Health.Port == 0 {
			continue // no health check configured
		}

		// Run the check
		checkTimeout := 5 * time.Second
		if svcDef.Health.Timeout != "" {
			if d, err := time.ParseDuration(svcDef.Health.Timeout); err == nil && d > 0 {
				checkTimeout = d
			}
		}

		cfg := Config{
			Type:    CheckType(svcDef.Health.Type),
			Host:    "127.0.0.1", // local container
			Port:    svcDef.Health.Port,
			Path:    svcDef.Health.Path,
			Timeout: checkTimeout,
		}

		result := l.checker.Check(ctx, cfg)

		if result.Healthy {
			metrics.HealthCheckTotal.WithLabelValues("healthy").Inc()
			if l.failures[svcName] > 0 {
				slog.Info("health check recovered", "service", svcName)
			}
			l.failures[svcName] = 0
			continue
		}

		metrics.HealthCheckTotal.WithLabelValues("unhealthy").Inc()
		l.failures[svcName]++
		slog.Warn("health check failed",
			"service", svcName,
			"consecutive", l.failures[svcName],
			"message", result.Message,
		)

		// Auto-restart after exceeding retries
		retries := svcDef.Health.Retries
		if retries <= 0 {
			retries = 3
		}
		if l.failures[svcName] >= retries {
			if svcDef.RestartPolicy == "no" {
				slog.Warn("container unhealthy but restart_policy=no, skipping restart",
					"service", svcName,
				)
				continue
			}

			// Don't destroy the container if we can't recreate it.
			// Phase 1: log the persistent failure for operator attention.
			// Phase 2 will reconstruct the ContainerSpec from store and auto-restart.
			slog.Error("container persistently unhealthy, manual intervention required",
				"service", svcName,
				"container", c.ID,
				"consecutive_failures", l.failures[svcName],
			)
			l.failures[svcName] = 0
		}
	}
}
