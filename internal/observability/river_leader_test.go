package observability

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

// fakeRow lets us program the scan target for stubQuerier below without
// pulling in a pgxmock dependency.
type fakeRow struct {
	count int
	err   error
}

func (r *fakeRow) Scan(dst ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dst) == 1 {
		if ptr, ok := dst[0].(*int); ok {
			*ptr = r.count
		}
	}
	return nil
}

type stubQuerier struct {
	row *fakeRow
}

func (s *stubQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return s.row
}

func TestRiverLeaderActive(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		row  *fakeRow
		want float64
	}{
		{"non-expired leader present", &fakeRow{count: 1}, 1},
		{"no leader row", &fakeRow{count: 0}, 0},
		{"query error degrades to 0", &fakeRow{err: errors.New("boom")}, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := riverLeaderActive(context.Background(), &stubQuerier{row: tc.row})
			if got != tc.want {
				t.Errorf("riverLeaderActive() = %v, want %v", got, tc.want)
			}
		})
	}
}
