# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository status

The product is **`skillbrowse`**, a read-only, keyboard-first terminal application (macOS and Linux) for discovering and reading AI-agent skills (`SKILL.md` files) installed across tools like Claude Code, Cursor, Codex, Hermes, and generic `~/.agents` layouts. Implementation is in progress, following `docs/skillbrowse-implementation-plan.md`'s phased sequence. Done: Phase 0 (scaffolding), Phase 1 (catalog core), Phase 2 (the Bubble Tea TUI), Phase 3 (the self-updater), and Phase 4 (error-taxonomy/exit-code correctness, `SKILLBROWSE_DEBUG=1` diagnostics via `internal/debug`, and a source-diagnostics view in the TUI help overlay). Not yet built: performance validation (Phase 5) and the release pipeline (Phase 6). The TUI has not been verified against a real terminal/pty in this environment — only unit-tested and smoke-tested non-interactively — so a manual `go run ./cmd/skillbrowse` check is recommended before treating it as done. Self-updating won't actually succeed yet: `update.DefaultVerifier()` has no embedded trusted key until Phase 6 sets up real release signing, so `upgrade` will correctly fail signature verification for now.

The authoritative specs are:

- `docs/skillbrowse-project-brief.md` — product brief: problem, scope, UX principles, success measures.
- `docs/superpowers/specs/2026-08-12-skillbrowse-design.md` — full product requirements and technical design (UX detail, CLI surface, discovery model, package architecture, error handling, performance/security requirements, testing strategy, release process).
- `docs/skillbrowse-implementation-plan.md` — the phased build plan derived from the two docs above; check `tasks/todo.md` for current progress against it.

Read the specs before extending implementation — the design doc is the detailed source of truth and the brief is its companion summary.

## Commands

```
make build       # go build -o bin/skillbrowse ./cmd/skillbrowse
make test         # go test ./...
make test-race    # go test -race ./...
make lint         # golangci-lint run ./...
make vuln         # govulncheck ./... (go install golang.org/x/vuln/cmd/govulncheck@latest if missing)
```

Run a single test: `go test ./internal/<package>/... -run TestName`.

## Planned architecture (from the design doc)

The design specifies a modular, in-memory Go application (Go 1.26) using the Charm v2 ecosystem — Bubble Tea v2 (app state/events), Bubbles v2 (list/input/viewport/spinner/help components), Lip Gloss v2 (responsive layout/styling), and Glamour v2 (Markdown rendering). No database; GoReleaser + GitHub Releases handle distribution. CLI routing uses Cobra (`cmd/skillbrowse`: root command launches the TUI, plus `upgrade [--check] [--yes]`, `version`, `help`).

Package boundaries are intentionally decoupled — filesystem discovery has no dependency on terminal components, the updater has no dependency on the catalog, and the UI receives catalog snapshots/diagnostics via messages rather than touching the filesystem directly:

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
internal/debug        opt-in SKILLBROWSE_DEBUG=1 stderr diagnostic log
```

Data flow: built-in registry + validated custom sources → discovery scanner → metadata parser → catalog normalizer → immutable catalog snapshot → Bubble Tea UI (rescan loops back to the scanner; explicit update requests go to the updater → GitHub Releases).

Key design constraints to preserve when implementing:

- **Read-only in v1**: never install, edit, delete, or upgrade skills themselves — only the `skillbrowse` binary self-upgrades.
- **No network on ordinary startup/browsing/rescan** — network access is explicit and user-initiated (`u` / `upgrade` command) only.
- **Resilience**: a malformed skill or unreadable source must never abort scanning of the rest; missing built-in roots are silent, unreadable existing roots produce a diagnostic.
- **Security for self-upgrade**: SHA-256 checksum manifest + Ed25519 signature verification with an embedded public key, safe archive extraction (reject `..`, absolute paths, symlinks, extra executables), atomic rename, no shell execution of downloaded content.
- **Untrusted content**: `SKILL.md` files are treated as untrusted text — never executed/templated, and terminal control sequences are sanitized before rendering.
- Config file: `$XDG_CONFIG_HOME/skillbrowse/config.toml` (or `~/.config/skillbrowse/config.toml`), `version = 1`, TOML `[[sources]]` entries with `path`, optional `label`/`agents`/`max_depth` (1–12, default 4)/`enabled`.
