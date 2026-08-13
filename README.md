# skillbrowse

A read-only, keyboard-first terminal application (macOS and Linux) for discovering and reading AI-agent skills (`SKILL.md` files) installed across tools like Claude Code, Cursor, Codex, Hermes, and generic `~/.agents` layouts.

> Run one command, see every locally installed skill, and read its instructions immediately.

## Status

Implementation is in progress, following the phased plan in [`docs/skillbrowse-implementation-plan.md`](docs/skillbrowse-implementation-plan.md).

- Done: Phase 0 (scaffolding), Phase 1 (catalog core — `internal/config`, `internal/sources`, `internal/discovery`, `internal/skill`, `internal/catalog`)
- Not yet built: TUI, self-updater, diagnostics/CLI polish, performance validation, release pipeline

Track detailed progress in [`tasks/todo.md`](tasks/todo.md).

## Documentation

- [`docs/skillbrowse-project-brief.md`](docs/skillbrowse-project-brief.md) — product brief: problem, scope, UX principles, success measures.
- [`docs/superpowers/specs/2026-08-12-skillbrowse-design.md`](docs/superpowers/specs/2026-08-12-skillbrowse-design.md) — full product requirements and technical design.
- [`docs/skillbrowse-implementation-plan.md`](docs/skillbrowse-implementation-plan.md) — phased build plan.

## Key design constraints

- **Read-only in v1** — never installs, edits, deletes, or upgrades skills themselves; only the `skillbrowse` binary self-upgrades.
- **No network on ordinary startup/browsing/rescan** — network access is explicit and user-initiated (`u` / `upgrade` command) only.
- **Resilience** — a malformed skill or unreadable source never aborts scanning of the rest.
- **Untrusted content** — `SKILL.md` files are treated as untrusted text: never executed or templated, and terminal control sequences are sanitized before rendering.

## Architecture

Built with Go 1.26 and the Charm v2 ecosystem (Bubble Tea, Bubbles, Lip Gloss, Glamour) for the TUI, and Cobra for CLI routing. No database — everything is scanned and held in memory. Distribution via GoReleaser + GitHub Releases.

```text
cmd/skillbrowse       command routing, flags, dependency wiring
internal/config       TOML loading, validation, path expansion
internal/sources      built-in registry and source descriptors
internal/discovery    bounded filesystem scanning and cancellation
internal/skill        parser, normalized model, diagnostics
internal/catalog      merging, sorting, filtering inputs
internal/ui           Bubble Tea models, views, key maps, responsive layout
internal/markdown     sanitized rendering and width-aware cache
internal/update       release lookup, verification, staging, replacement
internal/buildinfo    version and build metadata
```

## Development

```sh
make build       # go build -o bin/skillbrowse ./cmd/skillbrowse
make test        # go test ./...
make test-race   # go test -race ./...
make lint        # golangci-lint run ./...
make vuln        # govulncheck ./...
```

Run a single test: `go test ./internal/<package>/... -run TestName`.

See [`CLAUDE.md`](CLAUDE.md) for full contributor/agent guidance.
