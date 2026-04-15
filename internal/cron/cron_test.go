package cron

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		expr    string
		wantErr bool
		errMsg  string
	}{
		{"every minute", "* * * * *", false, ""},
		{"top of hour", "0 * * * *", false, ""},
		{"specific time", "30 14 * * *", false, ""},
		{"range", "0-5 * * * *", false, ""},
		{"step", "*/15 * * * *", false, ""},
		{"list", "1,15,30 * * * *", false, ""},
		{"range with step", "1-30/2 * * * *", false, ""},
		{"complex", "0,30 9-17 * 1-6 1-5", false, ""},
		{"dow 7 sunday", "0 0 * * 7", false, ""},
		{"dow 0 sunday", "0 0 * * 0", false, ""},

		// Error cases.
		{"too few fields", "* * *", true, "expected 5 fields, got 3"},
		{"too many fields", "* * * * * *", true, "expected 5 fields, got 6"},
		{"empty", "", true, "expected 5 fields, got 0"},
		{"minute out of range", "60 * * * *", true, "out of range"},
		{"hour out of range", "0 24 * * *", true, "out of range"},
		{"dom out of range", "0 0 32 * *", true, "out of range"},
		{"dom zero", "0 0 0 * *", true, "out of range"},
		{"month out of range", "0 0 * 13 *", true, "out of range"},
		{"month zero", "0 0 * 0 *", true, "out of range"},
		{"dow out of range", "0 0 * * 8", true, "out of range"},
		{"invalid range", "5-2 * * * *", true, "invalid range"},
		{"bad value", "abc * * * *", true, "invalid value"},
		{"bad step", "*/0 * * * *", true, "step must be positive"},
		{"negative step", "*/-1 * * * *", true, "step must be positive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, err := Parse(tt.expr)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) = nil error, want error containing %q", tt.expr, tt.errMsg)
				}
				if tt.errMsg != "" && !containsSubstring(err.Error(), tt.errMsg) {
					t.Errorf("Parse(%q) error = %q, want it to contain %q", tt.expr, err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.expr, err)
			}
			if s == nil {
				t.Fatalf("Parse(%q) returned nil schedule", tt.expr)
			}
		})
	}
}

func TestNext(t *testing.T) {
	t.Parallel()

	loc := time.UTC

	tests := []struct {
		name string
		expr string
		from time.Time
		want time.Time
	}{
		{
			name: "every minute advances one minute",
			expr: "* * * * *",
			from: time.Date(2025, 1, 1, 0, 0, 0, 0, loc),
			want: time.Date(2025, 1, 1, 0, 1, 0, 0, loc),
		},
		{
			name: "top of hour from middle of hour",
			expr: "0 * * * *",
			from: time.Date(2025, 1, 1, 12, 30, 0, 0, loc),
			want: time.Date(2025, 1, 1, 13, 0, 0, 0, loc),
		},
		{
			name: "specific time 14:30",
			expr: "30 14 * * *",
			from: time.Date(2025, 1, 1, 0, 0, 0, 0, loc),
			want: time.Date(2025, 1, 1, 14, 30, 0, 0, loc),
		},
		{
			name: "specific time already past today",
			expr: "30 14 * * *",
			from: time.Date(2025, 1, 1, 15, 0, 0, 0, loc),
			want: time.Date(2025, 1, 2, 14, 30, 0, 0, loc),
		},
		{
			name: "every 15 minutes",
			expr: "*/15 * * * *",
			from: time.Date(2025, 1, 1, 0, 10, 0, 0, loc),
			want: time.Date(2025, 1, 1, 0, 15, 0, 0, loc),
		},
		{
			name: "every 15 minutes from 14",
			expr: "*/15 * * * *",
			from: time.Date(2025, 1, 1, 0, 14, 0, 0, loc),
			want: time.Date(2025, 1, 1, 0, 15, 0, 0, loc),
		},
		{
			name: "list of minutes",
			expr: "1,15,30 * * * *",
			from: time.Date(2025, 1, 1, 0, 2, 0, 0, loc),
			want: time.Date(2025, 1, 1, 0, 15, 0, 0, loc),
		},
		{
			name: "range with step",
			expr: "1-30/10 * * * *",
			from: time.Date(2025, 1, 1, 0, 0, 0, 0, loc),
			want: time.Date(2025, 1, 1, 0, 1, 0, 0, loc),
		},
		{
			name: "range with step skips",
			expr: "1-30/10 * * * *",
			from: time.Date(2025, 1, 1, 0, 2, 0, 0, loc),
			want: time.Date(2025, 1, 1, 0, 11, 0, 0, loc),
		},
		{
			name: "weekday filter Monday",
			expr: "0 9 * * 1",
			from: time.Date(2025, 1, 1, 0, 0, 0, 0, loc), // Wednesday
			want: time.Date(2025, 1, 6, 9, 0, 0, 0, loc), // Next Monday
		},
		{
			name: "dow 0 matches Sunday",
			expr: "0 0 * * 0",
			from: time.Date(2025, 1, 1, 0, 0, 0, 0, loc), // Wednesday
			want: time.Date(2025, 1, 5, 0, 0, 0, 0, loc), // Next Sunday
		},
		{
			name: "dow 7 matches Sunday same as 0",
			expr: "0 0 * * 7",
			from: time.Date(2025, 1, 1, 0, 0, 0, 0, loc), // Wednesday
			want: time.Date(2025, 1, 5, 0, 0, 0, 0, loc), // Next Sunday
		},
		{
			name: "end of month rollover",
			expr: "0 0 31 * *",
			from: time.Date(2025, 1, 31, 1, 0, 0, 0, loc),
			want: time.Date(2025, 3, 31, 0, 0, 0, 0, loc), // Feb has no 31, skips to March
		},
		{
			name: "february 28 non-leap",
			expr: "0 0 29 2 *",
			from: time.Date(2025, 1, 1, 0, 0, 0, 0, loc),
			want: time.Date(2028, 2, 29, 0, 0, 0, 0, loc), // Next leap year
		},
		{
			name: "leap year feb 29",
			expr: "0 0 29 2 *",
			from: time.Date(2028, 1, 1, 0, 0, 0, 0, loc),
			want: time.Date(2028, 2, 29, 0, 0, 0, 0, loc),
		},
		{
			name: "specific month",
			expr: "0 0 1 6 *",
			from: time.Date(2025, 7, 1, 0, 0, 0, 0, loc),
			want: time.Date(2026, 6, 1, 0, 0, 0, 0, loc),
		},
		{
			name: "from mid-second truncates to minute",
			expr: "* * * * *",
			from: time.Date(2025, 1, 1, 0, 0, 30, 0, loc),
			want: time.Date(2025, 1, 1, 0, 1, 0, 0, loc),
		},
		{
			name: "weekday range Mon-Fri",
			expr: "0 9 * * 1-5",
			from: time.Date(2025, 1, 4, 10, 0, 0, 0, loc), // Saturday
			want: time.Date(2025, 1, 6, 9, 0, 0, 0, loc),  // Monday
		},
		{
			name: "month list",
			expr: "0 0 1 1,7 *",
			from: time.Date(2025, 2, 1, 0, 0, 0, 0, loc),
			want: time.Date(2025, 7, 1, 0, 0, 0, 0, loc),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, err := Parse(tt.expr)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.expr, err)
			}

			got := s.Next(tt.from)
			if !got.Equal(tt.want) {
				t.Errorf("Next(%v) = %v, want %v", tt.from, got, tt.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	t.Parallel()

	expr := "*/5 9-17 * * 1-5"
	s, err := Parse(expr)
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", expr, err)
	}
	if s.String() != expr {
		t.Errorf("String() = %q, want %q", s.String(), expr)
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
