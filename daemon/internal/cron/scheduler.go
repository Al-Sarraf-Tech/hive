package cron

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Job is a scheduled task.
type Job struct {
	Name     string   // unique identifier
	Schedule *Schedule
	Service  string   // service to exec in
	Command  []string // command to run
	LastRun  time.Time
	NextRun  time.Time
}

// ExecFn is called when a job fires. Receives service name and command.
type ExecFn func(ctx context.Context, service string, command []string) error

// Scheduler runs cron jobs on their schedules.
type Scheduler struct {
	mu     sync.RWMutex
	jobs   map[string]*Job
	execFn ExecFn
}

// NewScheduler creates a cron scheduler that calls execFn when jobs fire.
func NewScheduler(execFn ExecFn) *Scheduler {
	return &Scheduler{
		jobs:   make(map[string]*Job),
		execFn: execFn,
	}
}

// Add registers a cron job. Returns error if the expression is invalid.
func (s *Scheduler) Add(name, cronExpr, service string, command []string) error {
	sched, err := Parse(cronExpr)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.jobs[name] = &Job{
		Name:     name,
		Schedule: sched,
		Service:  service,
		Command:  command,
		NextRun:  sched.Next(now),
	}
	slog.Info("cron job registered", "name", name, "schedule", cronExpr, "service", service, "next_run", s.jobs[name].NextRun.Format(time.RFC3339))
	return nil
}

// Remove unregisters a cron job.
func (s *Scheduler) Remove(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs[name]; ok {
		delete(s.jobs, name)
		return true
	}
	return false
}

// List returns a snapshot of all registered jobs.
func (s *Scheduler) List() []Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		jobs = append(jobs, *j)
	}
	return jobs
}

// Start runs the scheduler loop until ctx is cancelled.
// Checks every 30 seconds for jobs that need to fire.
func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Immediate check on startup
	s.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	now := time.Now()
	s.mu.Lock()
	var toRun []*Job
	for _, j := range s.jobs {
		if !j.NextRun.IsZero() && !now.Before(j.NextRun) {
			toRun = append(toRun, j)
		}
	}
	s.mu.Unlock()

	for _, j := range toRun {
		slog.Info("cron job firing", "name", j.Name, "service", j.Service, "command", j.Command)
		if err := s.execFn(ctx, j.Service, j.Command); err != nil {
			slog.Error("cron job failed", "name", j.Name, "error", err)
		}
		s.mu.Lock()
		j.LastRun = now
		j.NextRun = j.Schedule.Next(now)
		slog.Debug("cron job rescheduled", "name", j.Name, "next_run", j.NextRun.Format(time.RFC3339))
		s.mu.Unlock()
	}
}
