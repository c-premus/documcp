#!/usr/bin/env bash
# check-migrations.sh — lint goose migrations for unsafe DDL patterns.
#
# Enforces the convention documented in migrations/README.md:
#
#   1. CREATE INDEX (without CONCURRENTLY) is only allowed on a table that
#      is also CREATEd in the same migration file (the table is empty at
#      creation time, so the lock is microseconds).
#
#   2. DROP INDEX (without CONCURRENTLY) is forbidden — existing indexes
#      must be dropped concurrently.
#
#   3. Files containing CREATE INDEX CONCURRENTLY, DROP INDEX CONCURRENTLY,
#      or ALTER COLUMN ... TYPE must declare `-- +goose NO TRANSACTION`
#      somewhere in the file (typically the first non-blank line).
#
# Exit codes:
#   0 — all migrations pass the lint
#   1 — at least one violation found (printed with file:line + remediation)
#   2 — script invoked incorrectly or migrations directory missing
#
# Usage:
#   bash scripts/check-migrations.sh             # default: lint migrations/
#   bash scripts/check-migrations.sh path/to/dir # lint a custom dir

set -euo pipefail

MIGRATIONS_DIR="${1:-migrations}"

if [[ ! -d "$MIGRATIONS_DIR" ]]; then
    echo "check-migrations.sh: directory not found: $MIGRATIONS_DIR" >&2
    exit 2
fi

violations=0

# Strip SQL comments (lines starting with --) before pattern matching so
# "-- WARNING: ... CREATE INDEX ..." narrative comments don't trigger
# false positives. We keep the original line numbers for reporting via
# grep -n on the unfiltered file.

# Helper: report a violation. Args: file, line, pattern_name, hint
report() {
    local file=$1 line=$2 name=$3 hint=$4
    echo "$file:$line: $name" >&2
    echo "    $hint" >&2
    violations=$((violations + 1))
}

# Helper: returns 0 if file declares `-- +goose NO TRANSACTION`.
has_no_transaction_directive() {
    grep -qE '^[[:space:]]*--[[:space:]]*\+goose[[:space:]]+NO[[:space:]]+TRANSACTION' "$1"
}

# Helper: returns 0 if line is inside a SQL comment (starts with -- after
# optional whitespace). Used to skip narrative violations of the lint.
is_comment_line() {
    [[ "$1" =~ ^[[:space:]]*-- ]]
}

shopt -s nullglob
for file in "$MIGRATIONS_DIR"/*.sql; do
    # Skip files that explicitly opt out via `-- lint-disable-file`. Used
    # by historical migrations whose lock semantics are documented and
    # whose contents are frozen per migrations/README.md "Historical
    # record". The directive must include a reason after the colon so the
    # exemption is self-documenting.
    if grep -qE '^[[:space:]]*--[[:space:]]*lint-disable-file:' "$file"; then
        continue
    fi
    declare -a created_tables=()
    # Collect tables created in this file (lowercased identifier without
    # quotes). We use this to allow inline CREATE INDEX on these tables.
    while IFS= read -r tname; do
        # Lowercase via tr for portability across bash versions.
        created_tables+=("$(echo "$tname" | tr '[:upper:]' '[:lower:]')")
    done < <(
        grep -iE '^[[:space:]]*CREATE[[:space:]]+TABLE([[:space:]]+IF[[:space:]]+NOT[[:space:]]+EXISTS)?[[:space:]]+("?)([a-zA-Z_][a-zA-Z0-9_]*)' "$file" \
            | sed -E 's/^[[:space:]]*CREATE[[:space:]]+TABLE([[:space:]]+IF[[:space:]]+NOT[[:space:]]+EXISTS)?[[:space:]]+"?([a-zA-Z_][a-zA-Z0-9_]*).*/\2/I' \
            || true
    )

    declared_no_txn=0
    if has_no_transaction_directive "$file"; then
        declared_no_txn=1
    fi

    # Pass 1: CREATE INDEX without CONCURRENTLY on a table not created in this file.
    while IFS=: read -r line content; do
        # Skip comment-only lines.
        if is_comment_line "$content"; then
            continue
        fi
        # Skip lines that ARE concurrent.
        if echo "$content" | grep -qiE 'CREATE[[:space:]]+INDEX[[:space:]]+CONCURRENTLY'; then
            continue
        fi
        # Extract target table — pattern: CREATE INDEX [IF NOT EXISTS] [name] ON [schema.]table
        target=$(echo "$content" | sed -nE 's/.*CREATE[[:space:]]+INDEX([[:space:]]+IF[[:space:]]+NOT[[:space:]]+EXISTS)?([[:space:]]+[a-zA-Z_][a-zA-Z0-9_]*)?[[:space:]]+ON([[:space:]]+ONLY)?[[:space:]]+("?)([a-zA-Z_][a-zA-Z0-9_]*\.)?([a-zA-Z_][a-zA-Z0-9_]*).*/\6/Ip' \
            | tr '[:upper:]' '[:lower:]')
        if [[ -z "$target" ]]; then
            # Could not parse — flag for visibility rather than silently allow.
            report "$file" "$line" \
                "CREATE INDEX without CONCURRENTLY (could not parse target table)" \
                "Use 'CREATE INDEX CONCURRENTLY IF NOT EXISTS ... ON <table>' inside a '+goose NO TRANSACTION' migration. See migrations/README.md."
            continue
        fi
        # If the target was created in this file, allow.
        allowed=0
        for t in "${created_tables[@]:-}"; do
            if [[ "$t" == "$target" ]]; then
                allowed=1
                break
            fi
        done
        if [[ $allowed -eq 0 ]]; then
            report "$file" "$line" \
                "CREATE INDEX without CONCURRENTLY on existing table '$target'" \
                "Use 'CREATE INDEX CONCURRENTLY IF NOT EXISTS' inside a '+goose NO TRANSACTION' migration. See migrations/README.md."
        fi
    done < <(grep -niE '^[^-]*CREATE[[:space:]]+INDEX[[:space:]]+(IF[[:space:]]+NOT[[:space:]]+EXISTS[[:space:]]+)?[a-zA-Z_]' "$file" || true)

    # Pass 2: DROP INDEX without CONCURRENTLY.
    while IFS=: read -r line content; do
        if is_comment_line "$content"; then
            continue
        fi
        if echo "$content" | grep -qiE 'DROP[[:space:]]+INDEX[[:space:]]+CONCURRENTLY'; then
            continue
        fi
        report "$file" "$line" \
            "DROP INDEX without CONCURRENTLY" \
            "Use 'DROP INDEX CONCURRENTLY IF EXISTS' inside a '+goose NO TRANSACTION' migration. See migrations/README.md."
    done < <(grep -niE '^[^-]*DROP[[:space:]]+INDEX[[:space:]]+' "$file" || true)

    # Pass 3: CONCURRENTLY operations require NO TRANSACTION.
    if [[ $declared_no_txn -eq 0 ]]; then
        while IFS=: read -r line content; do
            if is_comment_line "$content"; then
                continue
            fi
            report "$file" "$line" \
                "CONCURRENTLY operation in a transactional migration" \
                "Add '-- +goose NO TRANSACTION' at the top of the file (above '-- +goose Up'). See migrations/README.md."
        done < <(grep -niE '(CREATE[[:space:]]+INDEX|DROP[[:space:]]+INDEX|REINDEX)[[:space:]]+CONCURRENTLY' "$file" || true)
    fi

    # Pass 4: ALTER COLUMN ... TYPE requires NO TRANSACTION (defensive — the
    # cast is a table rewrite either way, but flagging it prompts the
    # author to consider the safe multi-step pattern in README.md).
    if [[ $declared_no_txn -eq 0 ]]; then
        while IFS=: read -r line content; do
            if is_comment_line "$content"; then
                continue
            fi
            report "$file" "$line" \
                "ALTER COLUMN ... TYPE in a transactional migration" \
                "ALTER COLUMN TYPE rewrites the table under ACCESS EXCLUSIVE. Decompose into the safe multi-step pattern in migrations/README.md (add new column + dual-write + backfill + swap), or add '-- +goose NO TRANSACTION' if the rewrite is intentional."
        done < <(grep -niE 'ALTER[[:space:]]+COLUMN[[:space:]]+[a-zA-Z_][a-zA-Z0-9_]*[[:space:]]+(SET[[:space:]]+DATA[[:space:]]+)?TYPE' "$file" || true)
    fi

    unset created_tables
done

if [[ $violations -gt 0 ]]; then
    echo "" >&2
    echo "check-migrations.sh: $violations violation(s) found. See migrations/README.md for the safe-migration convention." >&2
    exit 1
fi

echo "check-migrations.sh: $MIGRATIONS_DIR clean ($(ls "$MIGRATIONS_DIR"/*.sql 2>/dev/null | wc -l) files)."
exit 0
