# Migrations

PostgreSQL schema migrations for DocuMCP. Applied via `goose v3` in
`internal/database/migrate.go` against the same `*pgxpool.Pool` shared
by the rest of the application. Versions tracked in the
`goose_db_version` table; goose holds a session-level advisory lock for
the duration of an apply pass so concurrent invocations serialize.

## Why this file exists

Five migrations in this directory (000013, 000015, 000016, 000017,
000019) take `ACCESS EXCLUSIVE` on populated tables for the duration of
the apply window because they were written before this convention
existed. Two more (000008, 000012) used to do the same with index
creation; the 2026-05-02 retrofit moved them to `CREATE INDEX
CONCURRENTLY`. This document codifies the convention so the next
migration written here doesn't repeat the pattern.

## When `+goose NO TRANSACTION` is required

Goose wraps each migration in `BEGIN`/`COMMIT` by default. The
`-- +goose NO TRANSACTION` directive (placed at the top of the file,
above `-- +goose Up`) lifts that wrap and runs every statement in
autocommit. It's required for:

- `CREATE INDEX CONCURRENTLY` — Postgres rejects this inside a
  transaction.
- `DROP INDEX CONCURRENTLY` — same.
- `REINDEX CONCURRENTLY` — same.
- `ALTER TYPE … ADD VALUE` — must run outside a transaction.
- `VACUUM`, `VACUUM FULL`, `CLUSTER` — same.

Without the directive, goose emits the SQL inside `BEGIN`/`COMMIT` and
Postgres rejects the statement at parse time.

## Postgres lock spectrum

Lock conflicts between the migration's DDL and concurrent traffic
determine whether writes (or reads) block during the apply window.
Cheat sheet — relevant subset only:

| Operation | Lock | Blocks |
|-----------|------|--------|
| `SELECT` | ACCESS SHARE | nothing (compatible with anything except DDL writers) |
| `INSERT`/`UPDATE`/`DELETE` | ROW EXCLUSIVE | most DDL |
| `CREATE INDEX` (without CONCURRENTLY) | ACCESS EXCLUSIVE | **all reads and writes** |
| `CREATE INDEX CONCURRENTLY` | SHARE UPDATE EXCLUSIVE | nothing in DML |
| `DROP INDEX` | ACCESS EXCLUSIVE | all reads and writes |
| `DROP INDEX CONCURRENTLY` | SHARE UPDATE EXCLUSIVE | nothing in DML |
| `ALTER TABLE … ADD COLUMN` (NULLable, no DEFAULT) | ACCESS EXCLUSIVE | brief — metadata-only |
| `ALTER TABLE … ADD COLUMN … DEFAULT` (constant, PG 11+) | ACCESS EXCLUSIVE | brief — metadata-only |
| `ALTER TABLE … ADD COLUMN … GENERATED … STORED` | ACCESS EXCLUSIVE | **full table rewrite** |
| `ALTER TABLE … ALTER COLUMN … TYPE` | ACCESS EXCLUSIVE | **full table rewrite** |
| `ALTER TABLE … ADD CONSTRAINT … FOREIGN KEY` | SHARE ROW EXCLUSIVE | brief validation pass |
| `ALTER TABLE … ALTER COLUMN … DROP NOT NULL` | ACCESS EXCLUSIVE | brief — metadata-only |
| `ALTER TABLE … ALTER COLUMN … SET NOT NULL` | ACCESS EXCLUSIVE | full table scan to validate |
| `CREATE TABLE` | (no lock — new relation) | nothing |
| `DROP TABLE` | ACCESS EXCLUSIVE | all reads and writes on that table |

The two flavors of "full table rewrite" — STORED column add and
`ALTER COLUMN TYPE` — are the load-bearing cases. Both require Postgres
to materialize a new physical relation, holding `ACCESS EXCLUSIVE` for
the duration. Neither concurrent-mode flag changes that; the only fix
is to decompose the operation into multiple migrations.

## Convention for new migrations

- **CREATE INDEX on populated tables** must use `CREATE INDEX
  CONCURRENTLY IF NOT EXISTS` inside a `+goose NO TRANSACTION`
  migration. Same for `DROP INDEX CONCURRENTLY IF EXISTS` in the Down.
- **CREATE INDEX on a table created in the same migration** can stay
  inline (the table is empty at the moment of creation, so the lock is
  microseconds). 000002, 000003, 000005, 000006, 000007, 000010 follow
  this shape.
- **ALTER COLUMN TYPE on populated tables** is forbidden in a single
  migration. Use the safe multi-step pattern below.
- **STORED column adds on populated tables** are forbidden in a single
  migration. Use the safe multi-step pattern below.
- **Foreign key constraints**: `ADD CONSTRAINT … FOREIGN KEY` is
  acceptable inline because the validation pass is brief on small
  tables. For very large tables, use `ADD CONSTRAINT … NOT VALID` +
  separate `VALIDATE CONSTRAINT` so the validation can be online.

## Safe multi-step pattern for STORED column adds

When a populated table needs a new generated column (or a column-type
change), decompose into a sequence of migrations:

1. **Migration N**: `ALTER TABLE … ADD COLUMN new_col … NULL` —
   metadata-only, fast under ACCESS EXCLUSIVE.
2. **Migration N+1**: `CREATE OR REPLACE FUNCTION` + `CREATE TRIGGER`
   that maintains `new_col` on every INSERT/UPDATE so any rows written
   from this point forward carry the right value.
3. **Code release**: deploy the trigger but keep readers on `old_col`.
4. **Backfill**: a separate worker (e.g. a River one-shot job, or a
   migration with `+goose NO TRANSACTION` doing batched updates of
   500-1000 rows at a time) populates `new_col` for pre-trigger rows.
5. **Verify**: a SELECT counts rows where `new_col IS NULL` and the
   trigger predicate doesn't justify it. Block the swap until zero.
6. **Migration N+2**: `ALTER TABLE … ALTER COLUMN new_col SET NOT
   NULL` (online via `NOT VALID` + `VALIDATE`), then drop the trigger,
   then `ALTER TABLE … DROP COLUMN old_col` (metadata-only).
7. **Code release**: switch readers to `new_col`.

For a STORED generated column specifically, replace step 2 with
maintaining the value in a `BEFORE INSERT OR UPDATE` trigger. Once the
backfill is done and the swap has happened, the trigger can be
replaced with `ALTER TABLE … ADD COLUMN final_col … GENERATED ALWAYS
AS (…) STORED` only if the table is small enough that one more
ACCESS EXCLUSIVE rewrite is acceptable. Otherwise keep the trigger.

The pattern is not free — it's typically 3-4 migrations and a backfill
worker — but it avoids the multi-minute write-downtime window that an
in-place STORED rewrite forces.

## Lint gate

`scripts/check-migrations.sh` enforces this convention in CI. It
exits non-zero on:

- `CREATE INDEX` (without CONCURRENTLY) on a table that is not also
  being created in the same migration.
- `DROP INDEX` (without CONCURRENTLY) on existing indexes.
- `CREATE INDEX CONCURRENTLY`, `DROP INDEX CONCURRENTLY`, or
  `ALTER COLUMN … TYPE` in a file that lacks `-- +goose NO TRANSACTION`
  at the top.

The script is wired into the Forgejo `lint` job and the GitHub Actions
mirror.

## Historical record

The five existing rewrite migrations (000013, 000015, 000016, 000017,
000019) carry inline `WARNING: TABLE-REWRITE MIGRATION` comments at
the top of their `-- +goose Up` sections. They are not retroactively
restructured because:

1. Already deployed to production at `deploy-20260429-002909` (v0.27.0).
   Goose tracks applied versions; modifying the SQL of an applied
   migration only affects future fresh deploys.
2. The PHP→Go schema-compat path (`projectbrief.md` "same PostgreSQL
   schema (zero-downtime migration possible)") is the load-bearing
   case where these would re-apply on a populated tenant. The inline
   warnings tell operators what to expect; the safer pattern above
   tells future contributors what to write next time.
3. Decomposing each into the multi-step pattern would be 4-7 new
   migrations per finding — too invasive for migrations that are
   already past the point where they could matter on the existing
   prod database.

If a future tenant onboarding (PHP→Go install, fork, or hosted-service
shape) makes the lock window genuinely operationally painful, the right
move is to write a migration that pre-applies the new shape under the
multi-step pattern *before* running the historical migrations, not to
retroactively rewrite the historical ones.
