package health

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
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
	restartTimes         map[string][]time.Time
	stopCh               chan struct{}
	stopOnce             sync.Once
	onContainerCountFunc func(int) // callback to update container count in gossip metadata
	onTickFunc           func()   // callback after each health tick (update resources in gossip, etc.)
	history              *History // health event history (may be nil)
}

// NewLoop creates a health check loop.
// onContainerCount is called each tick with the current running container count (may be nil).
// history records health events for the timeline API (may be nil).
func NewLoop(checker *Checker, c container.Provider, s *store.Store, interval time.Duration, onContainerCount func(int), onTick func(), history *History) *Loop {
	return &Loop{
		checker:              checker,
		container:            c,
		store:                s,
		interval:             interval,
		failures:             make(map[string]int),
		restartTimes:         make(map[string][]time.Time),
		stopCh:               make(chan struct{}),
		onContainerCountFunc: onContainerCount,
		onTickFunc:           onTick,
		history:              history,
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
	if l.onTickFunc != nil {
		l.onTickFunc()
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

		// Update failure counter BEFORE recording so the event has the correct count
		if result.Healthy {
			metrics.HealthCheckTotal.WithLabelValues("healthy").Inc()
			if l.failures[svcName] > 0 {
				slog.Info("health check recovered", "service", svcName)
			}
			l.failures[svcName] = 0
		} else {
			metrics.HealthCheckTotal.WithLabelValues("unhealthy").Inc()
			l.failures[svcName]++
		}

		// Record health event in timeline history
		if l.history != nil {
			l.history.Record(svcName, HealthEvent{
				Timestamp:           result.CheckedAt,
				Healthy:             result.Healthy,
				Message:             result.Message,
				DurationMs:          int32(result.Duration.Milliseconds()),
				CheckType:           string(cfg.Type),
				ConsecutiveFailures: int32(l.failures[svcName]),
			})
		}

		if result.Healthy {
			continue
		}
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

			// Crash-loop protection: max 3 restarts per 10 minutes
			now := time.Now()
			key := svcName
			l.restartTimes[key] = append(l.restartTimes[key], now)
			// Trim old entries outside the 10-minute window
			cutoff := now.Add(-10 * time.Minute)
			times := l.restartTimes[key]
			i := 0
			for i < len(times) && times[i].Before(cutoff) {
				i++
			}
			l.restartTimes[key] = times[i:]

			if len(l.restartTimes[key]) > 3 {
				slog.Error("crash loop detected, backing off",
					"service", svcName,
					"restarts_in_10m", len(l.restartTimes[key]))
				l.failures[svcName] = 0
				continue
			}

			slog.Warn("auto-restarting unhealthy container",
				"service", svcName,
				"container", container.ShortID(c.ID),
				"consecutive_failures", l.failures[svcName])

			// Stop and remove the failed container
			_ = l.container.Stop(ctx, c.ID, 10)
			_ = l.container.Remove(ctx, c.ID)

			// Rebuild container spec from stored service definition
			replicaIdx := 0
			if label, ok := c.Labels["hive.replica"]; ok {
				fmt.Sscanf(label, "%d", &replicaIdx)
			}

			// Build ports with replica offset
			ports := make(map[string]string)
			for hp, cp := range svcDef.Ports {
				if replicaIdx > 0 {
					if p, err := strconv.Atoi(hp); err == nil {
						hp = strconv.Itoa(p + replicaIdx)
					}
				}
				ports[hp] = cp
			}

			// Parse memory limit from service definition
			var memMB int64
			if svcDef.Resources.Memory != "" {
				if memBytes, parseErr := hivefile.ParseMemory(svcDef.Resources.Memory); parseErr == nil {
					memMB = memBytes / (1024 * 1024)
				}
			}

			// Build volume specs
			var volumes []container.VolumeSpec
			for _, v := range svcDef.Volumes {
				src := v.Linux
				if src == "" {
					src = v.Name
				}
				volumes = append(volumes, container.VolumeSpec{
					Source:   src,
					Target:   v.Target,
					ReadOnly: v.ReadOnly,
				})
			}

			// Look up the network name from store
			var networkName string
			if netBytes, netErr := l.store.Get("meta", "network:"+svcName); netErr == nil && netBytes != nil {
				networkName = string(netBytes)
			}

			// Build DNS aliases for Docker network resolution
			var networkAliases []string
			if networkName != "" {
				networkAliases = []string{
					svcName,                                        // "web" — resolves to any replica
					fmt.Sprintf("%s-%d", svcName, replicaIdx),      // "web-0" — specific replica
				}
			}

			spec := container.ContainerSpec{
				Name:           fmt.Sprintf("hive-%s-%d", svcName, replicaIdx),
				Image:          svcDef.Image,
				Env:            svcDef.Env,
				Ports:          ports,
				Volumes:        volumes,
				MemoryMB:       memMB,
				CPUs:           svcDef.Resources.CPUs,
				RestartPolicy:  svcDef.RestartPolicy,
				NetworkName:    networkName,
				NetworkAliases: networkAliases,
				Labels: map[string]string{
					"hive.managed": "true",
					"hive.service": svcName,
					"hive.replica": fmt.Sprintf("%d", replicaIdx),
				},
			}

			newID, createErr := l.container.CreateAndStart(ctx, spec)
			if createErr != nil {
				slog.Error("auto-restart failed", "service", svcName, "error", createErr)
			} else {
				slog.Info("container auto-restarted",
					"service", svcName,
					"old_id", container.ShortID(c.ID),
					"new_id", container.ShortID(newID))
			}
			l.failures[svcName] = 0
		}
	}

	// Clean up stale failure counts for services that no longer have running containers.
	// Prevents a redeployed service from inheriting a prior deployment's failure count.
	activeSvcs := make(map[string]bool)
	for _, c := range containers {
		if svc := c.Labels["hive.service"]; svc != "" {
			activeSvcs[svc] = true
		}
	}
	for svc := range l.failures {
		if !activeSvcs[svc] {
			delete(l.failures, svc)
		}
	}
	for svc := range l.restartTimes {
		if !activeSvcs[svc] {
			delete(l.restartTimes, svc)
		}
	}
}
