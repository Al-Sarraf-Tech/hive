// Package cron implements a lightweight cron scheduler for Hive.
// Supports standard 5-field cron expressions (minute hour day month weekday).
package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schedule represents a parsed cron expression.
type Schedule struct {
	Minute  fieldMatcher // 0-59
	Hour    fieldMatcher // 0-23
	Day     fieldMatcher // 1-31
	Month   fieldMatcher // 1-12
	Weekday fieldMatcher // 0-6 (Sunday=0)
}

// Parse parses a standard 5-field cron expression.
// Supports: *, ranges (1-5), steps (*/5, 1-10/2), lists (1,3,5), and literals.
func Parse(expr string) (*Schedule, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron expression must have 5 fields, got %d: %q", len(fields), expr)
	}

	minute, err := parseField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("minute field: %w", err)
	}
	hour, err := parseField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("hour field: %w", err)
	}
	day, err := parseField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("day field: %w", err)
	}
	month, err := parseField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("month field: %w", err)
	}
	weekday, err := parseField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("weekday field: %w", err)
	}

	return &Schedule{
		Minute:  minute,
		Hour:    hour,
		Day:     day,
		Month:   month,
		Weekday: weekday,
	}, nil
}

// Matches returns true if the given time matches this schedule.
func (s *Schedule) Matches(t time.Time) bool {
	return s.Minute.matches(t.Minute()) &&
		s.Hour.matches(t.Hour()) &&
		s.Day.matches(t.Day()) &&
		s.Month.matches(int(t.Month())) &&
		s.Weekday.matches(int(t.Weekday()))
}

// Next returns the next time after 'from' that matches this schedule.
// Searches up to 366 days ahead; returns zero time if no match found.
func (s *Schedule) Next(from time.Time) time.Time {
	// Start from the next minute
	t := from.Truncate(time.Minute).Add(time.Minute)
	limit := t.Add(366 * 24 * time.Hour)

	for t.Before(limit) {
		if !s.Month.matches(int(t.Month())) {
			// Skip to first day of next month
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			continue
		}
		if !s.Day.matches(t.Day()) || !s.Weekday.matches(int(t.Weekday())) {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
			continue
		}
		if !s.Hour.matches(t.Hour()) {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
			continue
		}
		if !s.Minute.matches(t.Minute()) {
			t = t.Add(time.Minute)
			continue
		}
		return t
	}
	return time.Time{}
}

// fieldMatcher matches integers in a cron field.
type fieldMatcher struct {
	all    bool
	values map[int]bool
}

func (f fieldMatcher) matches(val int) bool {
	if f.all {
		return true
	}
	return f.values[val]
}

func parseField(s string, min, max int) (fieldMatcher, error) {
	if s == "*" {
		return fieldMatcher{all: true}, nil
	}

	values := make(map[int]bool)

	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)

		// Handle step: */N or range/N
		step := 1
		if idx := strings.Index(part, "/"); idx >= 0 {
			var err error
			step, err = strconv.Atoi(part[idx+1:])
			if err != nil || step <= 0 {
				return fieldMatcher{}, fmt.Errorf("invalid step in %q", s)
			}
			part = part[:idx]
		}

		if part == "*" {
			// */N
			for i := min; i <= max; i += step {
				values[i] = true
			}
			continue
		}

		// Handle range: a-b
		if idx := strings.Index(part, "-"); idx >= 0 {
			lo, err := strconv.Atoi(part[:idx])
			if err != nil {
				return fieldMatcher{}, fmt.Errorf("invalid range start in %q", s)
			}
			hi, err := strconv.Atoi(part[idx+1:])
			if err != nil {
				return fieldMatcher{}, fmt.Errorf("invalid range end in %q", s)
			}
			if lo < min || hi > max || lo > hi {
				return fieldMatcher{}, fmt.Errorf("range %d-%d out of bounds (%d-%d)", lo, hi, min, max)
			}
			for i := lo; i <= hi; i += step {
				values[i] = true
			}
			continue
		}

		// Single value
		val, err := strconv.Atoi(part)
		if err != nil {
			return fieldMatcher{}, fmt.Errorf("invalid value %q", part)
		}
		if val < min || val > max {
			return fieldMatcher{}, fmt.Errorf("value %d out of range (%d-%d)", val, min, max)
		}
		values[val] = true
	}

	if len(values) == 0 {
		return fieldMatcher{}, fmt.Errorf("empty field: %q", s)
	}
	return fieldMatcher{values: values}, nil
}
