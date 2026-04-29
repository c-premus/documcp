# Contributing

Thanks for your interest in DocuMCP. This file describes how the
project is developed, where to send changes, and what to expect when you
do.

## Project layout

- **GitHub** (`github.com/c-premus/documcp`) is a public mirror.
- The source of truth lives in a **private Forgejo** instance. Commits
  flow Forgejo → GitHub via `.forgejo/workflows/sync-github.yaml`, which
  rewrites history with `git filter-repo` to strip internal paths
  (`.devcontainer/`, internal CI configs, dev-only docs, agent
  configuration) before force-pushing to GitHub.

This is one-way mirroring. Two consequences for contributors:

1. **PRs are still welcome on GitHub.** Open them against `main`.
2. **PR commits get rewritten when they land.** Your authored commits
   will appear in the public history (with your name and email
   preserved), but their SHAs change on the next sync because the
   filter-repo rewrite is whole-history. If you fork later, fetch fresh
   to avoid working against stale SHAs.

## How a contribution moves through the system

1. You open a PR against `github.com/c-premus/documcp:main`.
2. The maintainer reviews on GitHub. Discussion happens in PR comments.
3. When merged, the maintainer cherry-picks (or rebases) the change to
   the private Forgejo `dev` branch. CI runs there; the change reaches
   the public mirror at the next sync (push to `main` or release tag).
4. The PR on GitHub gets closed with a reference to the public commit.

If you'd prefer to discuss a change before opening a PR, file a GitHub
issue first.

## Commit message format

Commit subjects drive the auto-generated `CHANGELOG.md`. Use
[Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>
```

Types that appear in the changelog:

- **feat:** new user-facing capability → `### Features`
- **fix:** bug fix → `### Fixes`
- **chore:** maintenance, dependency bumps, infra → `### Maintenance`
- **`!:` suffix or `BREAKING CHANGE:` body** → `### BREAKING CHANGES`

Types filtered out of the changelog (still welcome, just won't show in
release notes):

- `ci:`, `test:`, `refactor:`, `docs:`, `style:`, `perf:`

Promote anything user-visible (security fixes, API tweaks) to `fix:` or
`chore:` so it lands in the changelog.

Good subjects:

- `feat(search): add highlighted snippets to search_documents`
- `fix(epub): handle missing OPF rootfile attribute`
- `chore(deps): bump golang.org/x/image to v0.39.0`

## Pre-v1 versioning

Until v1.0.0, **breaking changes bump minor** (per the project's
`.forgejo/workflows/version-release.yaml`):

- `feat!:` or `BREAKING CHANGE:` while major is `0` → minor bump.
- `v1.0.0` is reserved for an explicit, manually-dispatched cut.

This matches Semver §4 ("v0.x.y is unstable; anything may change").

## Development setup

See [`README.md`](./README.md) for environment requirements (Go 1.26+,
PostgreSQL, Redis, optional Kiwix Serve / OIDC provider). The
`devcontainer.json` definition is private; if you contribute often, the
maintainer can share it on request.

Run the tests before submitting:

```bash
go test ./...                   # unit
go test -race ./...             # with race detector
go test -tags integration ./... # full suite (needs Docker)
```

Lint:

```bash
golangci-lint run
```

## License

By contributing, you agree your contributions will be licensed under the
project's LICENSE.

## Reporting security issues

Don't open a public issue. Email the maintainer directly (see commit
metadata) and include reproduction steps.
