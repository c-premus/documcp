// Package cron provides a minimal parser for standard 5-field cron expressions.
//
// Supported format: minute hour dom month dow
//
// Each field supports: specific values, ranges (1-5), steps (*/5, 1-30/2),
// lists (1,15,30), and wildcard (*). Day-of-week uses 0-7 where both 0 and 7
// represent Sunday.
package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schedule represents a parsed cron expression. It implements
// Next(time.Time) time.Time, satisfying River's PeriodicSchedule interface.
type Schedule struct {
	minutes  bitset
	hours    bitset
	doms     bitset
	months   bitset
	dows     bitset
	original string
}

// bitset is a compact representation of which values in a field are set.
// Supports values 0-63 using a single uint64.
type bitset uint64

func (b *bitset) set(v int) { *b |= 1 << uint(v) } //nolint:gosec // values are bounds-checked before use
func (b *bitset) has(v int) bool {
	return *b&(1<<uint(v)) != 0 //nolint:gosec // values are bounds-checked before use
}

// Parse parses a standard 5-field cron expression (minute hour dom month dow).
// It returns a Schedule or a descriptive error.
func Parse(expr string) (*Schedule, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("parsing cron: expected 5 fields, got %d", len(fields))
	}

	minutes, err := parseField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("parsing cron minute field %q: %w", fields[0], err)
	}

	hours, err := parseField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("parsing cron hour field %q: %w", fields[1], err)
	}

	doms, err := parseField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("parsing cron dom field %q: %w", fields[2], err)
	}

	months, err := parseField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("parsing cron month field %q: %w", fields[3], err)
	}

	dows, err := parseField(fields[4], 0, 7)
	if err != nil {
		return nil, fmt.Errorf("parsing cron dow field %q: %w", fields[4], err)
	}

	// Normalize day-of-week: 7 (Sunday) maps to 0.
	if dows.has(7) {
		dows.set(0)
	}

	return &Schedule{
		minutes:  minutes,
		hours:    hours,
		doms:     doms,
		months:   months,
		dows:     dows,
		original: expr,
	}, nil
}

// Next returns the next time after t that matches the cron schedule.
func (s *Schedule) Next(t time.Time) time.Time {
	// Start from the next minute boundary.
	t = t.Truncate(time.Minute).Add(time.Minute)

	// Search up to 4 years ahead to cover leap year cycles.
	limit := t.Add(4 * 366 * 24 * time.Hour)

	for t.Before(limit) {
		if !s.months.has(int(t.Month())) {
			// Advance to first day of next month.
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			continue
		}

		if !s.doms.has(t.Day()) || !s.dows.has(int(t.Weekday())) {
			// Advance to next day.
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
			continue
		}

		if !s.hours.has(t.Hour()) {
			// Advance to next hour.
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
			continue
		}

		if !s.minutes.has(t.Minute()) {
			// Advance one minute.
			t = t.Add(time.Minute)
			continue
		}

		return t
	}

	// Should be unreachable for valid schedules.
	return time.Time{}
}

// String returns the original cron expression.
func (s *Schedule) String() string {
	return s.original
}

// parseField parses a single cron field into a bitset of allowed values.
func parseField(field string, lo, hi int) (bitset, error) {
	var b bitset

	for part := range strings.SplitSeq(field, ",") {
		if err := parsePart(part, lo, hi, &b); err != nil {
			return 0, err
		}
	}

	return b, nil
}

// parsePart parses a single element of a comma-separated cron field.
// It handles *, ranges (1-5), steps (*/5, 1-5/2), and single values.
func parsePart(part string, lower, upper int, b *bitset) error {
	// Split on "/" for step values.
	rangeStr, stepStr, hasStep := strings.Cut(part, "/")

	var lo, hi int

	switch {
	case rangeStr == "*":
		lo, hi = lower, upper
	case strings.ContainsRune(rangeStr, '-'):
		parts := strings.SplitN(rangeStr, "-", 2)
		var err error
		lo, err = strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("invalid value %q: %w", parts[0], err)
		}
		hi, err = strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid value %q: %w", parts[1], err)
		}
		if lo > hi {
			return fmt.Errorf("invalid range: %d > %d", lo, hi)
		}
	default:
		v, err := strconv.Atoi(rangeStr)
		if err != nil {
			return fmt.Errorf("invalid value %q: %w", rangeStr, err)
		}
		if !hasStep {
			// Single value, no step.
			if v < lower || v > upper {
				return fmt.Errorf("value %d out of range [%d, %d]", v, lower, upper)
			}
			b.set(v)
			return nil
		}
		lo, hi = v, upper
	}

	// Validate range bounds.
	if lo < lower || hi > upper {
		return fmt.Errorf("range %d-%d out of bounds [%d, %d]", lo, hi, lower, upper)
	}

	step := 1
	if hasStep {
		var err error
		step, err = strconv.Atoi(stepStr)
		if err != nil {
			return fmt.Errorf("invalid step %q: %w", stepStr, err)
		}
		if step <= 0 {
			return fmt.Errorf("step must be positive, got %d", step)
		}
	}

	for i := lo; i <= hi; i += step {
		b.set(i)
	}

	return nil
}
