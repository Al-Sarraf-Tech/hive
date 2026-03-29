// Package autoscale provides horizontal autoscaling for Hive services.
// When a service configures [autoscale], the autoscaler evaluates CPU metrics
// from the health loop and triggers scale up/down via the ScaleService RPC.
package autoscale

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jalsarraf0/hive/daemon/internal/hivefile"
)

// ScaleFunc is called when the autoscaler decides to change replica count.
type ScaleFunc func(ctx context.Context, service string, replicas int) error

// Autoscaler evaluates CPU metrics and triggers scaling decisions.
type Autoscaler struct {
	scaleFn   ScaleFunc
	mu        sync.Mutex
	lastScale map[string]time.Time // last scale event per service
}

// New creates an autoscaler with the given scale function.
func New(scaleFn ScaleFunc) *Autoscaler {
	return &Autoscaler{
		scaleFn:   scaleFn,
		lastScale: make(map[string]time.Time),
	}
}

// Evaluate checks if a service should be scaled up or down based on CPU metrics.
// currentReplicas is how many replicas are currently running.
// avgCPU is the average CPU percentage across all replicas (0-100).
func (a *Autoscaler) Evaluate(ctx context.Context, service string, config hivefile.AutoscaleDef, currentReplicas int, avgCPU float64) {
	if config.Max <= 0 || config.CPUTarget <= 0 {
		return // autoscale not configured
	}
	if config.Min <= 0 {
		config.Min = 1
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Parse cooldowns
	cooldownUp := 60 * time.Second
	cooldownDown := 300 * time.Second
	if d, err := time.ParseDuration(config.CooldownUp); err == nil && d > 0 {
		cooldownUp = d
	}
	if d, err := time.ParseDuration(config.CooldownDown); err == nil && d > 0 {
		cooldownDown = d
	}

	now := time.Now()
	lastScale := a.lastScale[service]

	// Scale up: CPU above target and below max replicas
	if avgCPU > config.CPUTarget && currentReplicas < config.Max {
		if now.Sub(lastScale) < cooldownUp {
			return // still in cooldown
		}
		newReplicas := currentReplicas + 1
		slog.Info("autoscale: scaling up", "service", service, "from", currentReplicas, "to", newReplicas, "cpu", avgCPU, "target", config.CPUTarget)
		if err := a.scaleFn(ctx, service, newReplicas); err != nil {
			slog.Error("autoscale: scale up failed", "service", service, "error", err)
			return
		}
		a.lastScale[service] = now
	}

	// Scale down: CPU below 50% of target and above min replicas
	if avgCPU < config.CPUTarget*0.5 && currentReplicas > config.Min {
		if now.Sub(lastScale) < cooldownDown {
			return
		}
		newReplicas := currentReplicas - 1
		slog.Info("autoscale: scaling down", "service", service, "from", currentReplicas, "to", newReplicas, "cpu", avgCPU, "target", config.CPUTarget)
		if err := a.scaleFn(ctx, service, newReplicas); err != nil {
			slog.Error("autoscale: scale down failed", "service", service, "error", err)
			return
		}
		a.lastScale[service] = now
	}
}
