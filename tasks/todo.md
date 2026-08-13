# skillbrowse — Implementation Plan

Full plan with rationale: `docs/skillbrowse-implementation-plan.md`.
Source specs: `docs/skillbrowse-project-brief.md`, `docs/superpowers/specs/2026-08-12-skillbrowse-design.md`.

Confirmed decisions: module path `github.com/dchancogne/skillbrowse`, Cobra for CLI routing.

## Phase 0 — Project scaffolding
- [x] `go.mod` (module `github.com/dchancogne/skillbrowse`, Go 1.26)
- [x] Package skeleton: `cmd/skillbrowse`, `internal/{config,sources,discovery,skill,catalog,ui,markdown,update,buildinfo}`
- [x] `.golangci.yml` static analysis config
- [x] `Makefile` with `build`, `test`, `test-race`, `lint`, `vuln` targets
- [x] `internal/buildinfo` (version/commit/date/Go/OS/arch via `-ldflags`)
- [x] `cmd/skillbrowse/main.go` with Cobra: root (TUI), `upgrade`, `version`, `help`; flags `--config`, `--path`, `--no-defaults`, `--no-color`

## Phase 1 — Catalog core
- [ ] `internal/config`: TOML load/validate, XDG default path resolution
- [ ] `internal/sources`: built-in registry (Agent Skills, Claude Code x2, Cursor, Codex x2, Hermes) + custom source merge
- [ ] `internal/discovery`: bounded concurrent walker, symlink rules, cancellation
- [ ] `internal/skill`: front-matter parser, fallbacks, 2 MiB cap, diagnostics
- [ ] `internal/catalog`: merge/dedup, deterministic sort, fuzzy search (`github.com/sahilm/fuzzy`)
- [ ] Unit tests per package + integration tests with fixture trees (FR-01–05)

## Phase 2 — Interactive TUI
- [ ] `internal/markdown`: Glamour v2 wrapper, sanitization, width-aware cache
- [ ] `internal/ui`: wide layout (list + detail panes)
- [ ] `internal/ui`: narrow layout (list/detail screens, Enter/Esc)
- [ ] `internal/ui`: search (`/`, fuzzy filter, highlighting)
- [ ] `internal/ui`: detail view + raw/rendered toggle (`v`)
- [ ] `internal/ui`: rescan (`r`), help overlay (`?`), min-size fallback, `--no-color`/`NO_COLOR`
- [ ] Wire default command to launch TUI
- [ ] Golden terminal-view tests + pseudo-terminal e2e tests

## Phase 3 — Self-updater
- [ ] `internal/update`: GitHub latest-release lookup, asset matching
- [ ] Verification chain: signature → checksum → safe extraction → version check → atomic rename
- [ ] Rollback invariant test (failure before rename leaves binary untouched)
- [ ] Wire `upgrade --check/--yes` command and `u` TUI key
- [ ] Fake release server test suite (9 cases from design doc §14.3)

## Phase 4 — Diagnostics, CLI polish, error taxonomy
- [ ] Three-tier error handling (fatal/source/skill) with correct exit codes
- [ ] `SKILLBROWSE_DEBUG=1` stderr diagnostics
- [ ] Home-relative path display, no content/stack-trace leaks
- [ ] `skillbrowse version` full output (FR-12)

## Phase 5 — Performance validation
- [ ] Synthetic fixture generator (1,000 / 10,000 skills)
- [ ] Benchmarks vs NFR-01–04 targets
- [ ] Goroutine-leak check on cancellation (NFR-07)

## Phase 6 — Release pipeline
- [ ] GoReleaser config (macOS/Linux × amd64/arm64, checksums, signing, provenance/SBOM)
- [ ] GitHub Actions CI (lint/test/race/vuln/build) + release smoke tests
- [ ] Install script with in-app-equivalent verification
- [ ] User docs (config, keys, diagnostics, privacy, upgrading, uninstalling)
- [ ] Update root `CLAUDE.md` with real build/lint/test commands

## Review
_(fill in after implementation: what changed, deviations from plan, follow-ups)_
