package cron

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParse_Basic(t *testing.T) {
	tests := []struct {
		expr    string
		wantErr bool
	}{
		{"* * * * *", false},
		{"0 0 * * *", false},
		{"*/5 * * * *", false},
		{"0 9-17 * * 1-5", false},
		{"30 4 1,15 * *", false},
		{"0 0 1 1 *", false},
		{"bad", true},
		{"* * *", true},
		{"60 * * * *", true},  // minute out of range
		{"* 25 * * *", true},  // hour out of range
		{"* * * * 8", true},   // weekday out of range
		{"* * 0 * *", true},   // day 0 out of range
		{"*/0 * * * *", true}, // step 0
	}

	for _, tt := range tests {
		_, err := Parse(tt.expr)
		if (err != nil) != tt.wantErr {
			t.Errorf("Parse(%q): err=%v, wantErr=%v", tt.expr, err, tt.wantErr)
		}
	}
}

func TestSchedule_Matches(t *testing.T) {
	s, err := Parse("30 14 * * 1") // 14:30 on Mondays
	if err != nil {
		t.Fatal(err)
	}

	// Monday 14:30
	mon := time.Date(2026, 3, 30, 14, 30, 0, 0, time.UTC) // 2026-03-30 is a Monday
	if !s.Matches(mon) {
		t.Error("should match Monday 14:30")
	}

	// Monday 14:31
	if s.Matches(mon.Add(time.Minute)) {
		t.Error("should not match 14:31")
	}

	// Tuesday 14:30
	tue := mon.Add(24 * time.Hour)
	if s.Matches(tue) {
		t.Error("should not match Tuesday")
	}
}

func TestSchedule_EveryFiveMinutes(t *testing.T) {
	s, err := Parse("*/5 * * * *")
	if err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 60; i++ {
		tt := base.Add(time.Duration(i) * time.Minute)
		expected := i%5 == 0
		if s.Matches(tt) != expected {
			t.Errorf("minute %d: got %v, want %v", i, s.Matches(tt), expected)
		}
	}
}

func TestSchedule_Next(t *testing.T) {
	s, err := Parse("0 12 * * *") // noon every day
	if err != nil {
		t.Fatal(err)
	}

	from := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC) // 10:00
	next := s.Next(from)
	expected := time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("next=%v, want %v", next, expected)
	}

	// After noon, should jump to next day
	from2 := time.Date(2026, 3, 27, 13, 0, 0, 0, time.UTC)
	next2 := s.Next(from2)
	expected2 := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)
	if !next2.Equal(expected2) {
		t.Errorf("next=%v, want %v", next2, expected2)
	}
}

func TestSchedule_Next_MonthSkip(t *testing.T) {
	s, err := Parse("0 0 1 6 *") // midnight on June 1st
	if err != nil {
		t.Fatal(err)
	}

	from := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	next := s.Next(from)
	if next.Month() != time.June || next.Day() != 1 {
		t.Errorf("expected June 1, got %v", next)
	}
}

func TestScheduler_AddRemoveList(t *testing.T) {
	sched := NewScheduler(func(ctx context.Context, svc string, cmd []string) error {
		return nil
	})

	err := sched.Add("backup", "0 2 * * *", "db", []string{"pg_dump"})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	jobs := sched.List()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Name != "backup" {
		t.Errorf("job name: %s", jobs[0].Name)
	}
	if jobs[0].Service != "db" {
		t.Errorf("job service: %s", jobs[0].Service)
	}

	if !sched.Remove("backup") {
		t.Error("remove should return true")
	}
	if sched.Remove("backup") {
		t.Error("second remove should return false")
	}
	if len(sched.List()) != 0 {
		t.Error("expected 0 jobs after remove")
	}
}

func TestScheduler_InvalidExpr(t *testing.T) {
	sched := NewScheduler(nil)
	err := sched.Add("bad", "not a cron", "svc", nil)
	if err == nil {
		t.Error("expected error for invalid expression")
	}
	if !strings.Contains(err.Error(), "5 fields") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestScheduler_Fires(t *testing.T) {
	var mu sync.Mutex
	var fired []string

	sched := NewScheduler(func(ctx context.Context, svc string, cmd []string) error {
		mu.Lock()
		fired = append(fired, svc)
		mu.Unlock()
		return nil
	})

	// Add a job that matches every minute
	_ = sched.Add("always", "* * * * *", "test-svc", []string{"echo", "hi"})

	// Force NextRun to the past so tick() fires it
	sched.mu.Lock()
	sched.jobs["always"].NextRun = time.Now().Add(-time.Minute)
	sched.mu.Unlock()

	// Manually trigger tick
	ctx := context.Background()
	sched.tick(ctx)

	mu.Lock()
	defer mu.Unlock()
	if len(fired) != 1 || fired[0] != "test-svc" {
		t.Errorf("expected [test-svc], got %v", fired)
	}
}
