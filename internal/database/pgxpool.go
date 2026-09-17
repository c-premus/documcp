package database

import (
	"context"
	"fmt"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPgxPool creates the primary database connection pool for all database access.
func NewPgxPool(ctx context.Context, dsn string, maxConns, minConns int32, maxConnLifetime, maxConnIdleTime time.Duration) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing pgxpool config: %w", err)
	}
	cfg.MaxConns = maxConns
	cfg.MinConns = minConns
	cfg.MaxConnLifetime = maxConnLifetime
	cfg.MaxConnIdleTime = maxConnIdleTime
	// otelpgx v0.12.0 made both of the options we used to pass here the
	// default, per the OpenTelemetry database span conventions: the span name
	// is the low-cardinality operation name ("SELECT") with no "query "
	// prefix, and the full statement stays in the db.query.text attribute.
	// The explicit options are now deprecated no-ops. Emitted span names are
	// unchanged; to go back, the opt-ins are WithFullSQLInSpanName() and
	// WithQuerySpanNamePrefix().
	cfg.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating pgxpool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging pgxpool: %w", err)
	}
	return pool, nil
}

// NewBarePgxPool creates a small uninstrumented pool used for trace-free
// checks — readiness probes and the River leader gauge. No otelpgx tracer
// means queries don't emit ping/pool.acquire spans on every scrape. Sized
// for low-frequency probes (MaxConns=2, MinConns=1); not a general-purpose
// pool — do not route application queries through it.
func NewBarePgxPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing bare pgxpool config: %w", err)
	}
	cfg.MaxConns = 2
	cfg.MinConns = 1
	cfg.MaxConnIdleTime = 10 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating bare pgxpool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging bare pgxpool: %w", err)
	}
	return pool, nil
}
