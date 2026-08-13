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
- [x] `internal/config`: TOML load/validate, XDG default path resolution
- [x] `internal/sources`: built-in registry (Agent Skills, Claude Code x2, Cursor, Codex x2, Hermes) + custom source merge
- [x] `internal/discovery`: bounded concurrent walker, symlink rules, cancellation
- [x] `internal/skill`: front-matter parser, fallbacks, 2 MiB cap, diagnostics
- [x] `internal/catalog`: merge/dedup, deterministic sort, fuzzy search (`github.com/sahilm/fuzzy`)
- [x] Unit tests per package + integration tests with fixture trees (FR-01–05)

## Phase 2 — Interactive TUI
- [x] `internal/markdown`: Glamour v2 wrapper, sanitization, width-aware cache
- [x] `internal/ui`: wide layout (list + detail panes)
- [x] `internal/ui`: narrow layout (list/detail screens, Enter/Esc)
- [x] `internal/ui`: search (`/`, fuzzy filter) — uses bubbles/list's own filter UI over a name/description/agent/label FilterValue (paths excluded — see internal/ui/item.go doc comment); no per-match highlighting yet
- [x] `internal/ui`: detail view + raw/rendered toggle (`v`)
- [x] `internal/ui`: rescan (`r`), help overlay (`?`), min-size fallback, `--no-color`/`NO_COLOR` (forced via `tea.WithColorProfile`)
- [x] Wire default command to launch TUI
- [ ] Golden terminal-view tests + pseudo-terminal e2e tests — not done; instead internal/ui/model_test.go unit-tests Update/View directly (navigation, search, narrow/wide, raw toggle, rescan-preserves-selection, help, too-small, quit). No real-terminal/pty verification was possible in this environment; manual `go run ./cmd/skillbrowse` smoke test still recommended before release.

## Phase 3 — Self-updater
- [x] `internal/update`: GitHub latest-release lookup, asset matching
- [x] Verification chain: signature → checksum → safe extraction → version check → atomic rename
- [x] Rollback invariant test (failure before rename leaves binary untouched)
- [x] Wire `upgrade --check/--yes` command and `u` TUI key (with an explicit y/n confirmation before install, per §12.1)
- [x] Fake release server test suite (9 cases from design doc §14.3, in internal/update/apply_test.go)
- Note: `update.DefaultVerifier()`'s trusted-key set is empty until Phase 6 generates and embeds the real Ed25519 signing key — self-update will correctly refuse to install (signature verification failure) until then. This is expected, not a bug.
- Also fixed in this phase: `cmd/skillbrowse/main.go` never printed RunE errors (root command sets `SilenceErrors`, and nothing else wrote them anywhere) — every CLI error was silently swallowed down to just an exit code. Now printed via `fmt.Fprintln(root.ErrOrStderr(), "Error:", err)`.
- Also bumped the Go toolchain (`go` directive) from 1.26.1 to 1.26.5: govulncheck flagged several stdlib CVEs (crypto/tls, net/http, crypto/x509, archive/tar, net/textproto) now reachable because Phase 3 added real HTTPS downloads and tar extraction.

## Phase 4 — Diagnostics, CLI polish, error taxonomy
- [x] Three-tier error handling (fatal/source/skill) with correct exit codes
- [x] `SKILLBROWSE_DEBUG=1` stderr diagnostics (`internal/debug`, used in cmd/skillbrowse, internal/update, internal/ui)
- [x] Home-relative path display, no content/stack-trace leaks
- [x] `skillbrowse version` full output (FR-12) — already correct from Phase 0, now covered by cmd/skillbrowse/root_test.go
- Bug fixed in this phase: exit code 2 ("invalid arguments or configuration") was only applied to our own `*usageError`s — Cobra's own pre-RunE errors (unknown command, unknown flag, wrong arg count) fell through to the default exit 1. Fixed with a `looksLikeUsageError` fallback matching Cobra's stable error-message prefixes, since Cobra exposes no typed error for these.
- Added: a "Source diagnostics" section in the help overlay (`?`), listing each source-level diagnostic (missing/unreadable configured root) with home-relative path and cause — design doc §9 pairs this with the footer's warning count, but no view previously showed the detail behind that count.
- `cmd/skillbrowse` had zero test coverage before this phase; added root_test.go (exit-code classification, version output, malformed-config/unknown-command/unknown-flag exit codes).

## Phase 5 — Performance validation
- [x] Synthetic fixture generator (1,000 / 10,000 skills) — `internal/benchfixture`
- [x] Benchmarks vs NFR-01–04 targets:
  - NFR-01 (100ms/p95 first frame): `BenchmarkModel_View_BeforeScan` — ~0.66ms/op
  - NFR-02 (500ms/p95, 1,000 skills): `BenchmarkPipeline_1000Skills` — ~73ms/op; `TestPipeline_1000SkillsRegressionGuard` is a loose (5s) CI smoke-test guard, not a strict p95 gate — shared CI hardware is noisier than "a 2022-era laptop with an SSD," so the benchmark itself is the real instrument for tracking this number
  - NFR-03 (50ms/p95 navigation/filter): `BenchmarkModel_Navigation` (~1.6ms/op), `BenchmarkModel_Filter` (~1.4ms/op)
  - NFR-04 (<100 MiB for 10,000 skills / <=50MiB Markdown): `TestCatalog_MemoryBudget` — measures retained heap (post-GC), not cumulative allocation; ~54 MiB retained for ~47 MiB of content
  - All comfortably within target on this dev machine (Intel i7-9750H); numbers will vary on other hardware, hence benchmarks over hardcoded thresholds
- [x] Goroutine-leak check on cancellation (NFR-07) — `internal/discovery/leak_test.go` via `go.uber.org/goleak`, covering both the cancelled-mid-scan and normal-completion paths (goroutines only actually originate in discovery's worker pool; the UI's cancellation wiring that calls into it was already exercised by Phase 2's rescan/quit tests)

## Phase 6 — Release pipeline
- [ ] GoReleaser config (macOS/Linux × amd64/arm64, checksums, signing, provenance/SBOM)
- [ ] GitHub Actions CI (lint/test/race/vuln/build) + release smoke tests
- [ ] Install script with in-app-equivalent verification
- [ ] User docs (config, keys, diagnostics, privacy, upgrading, uninstalling)
- [ ] Update root `CLAUDE.md` with real build/lint/test commands

## Review
_(fill in after implementation: what changed, deviations from plan, follow-ups)_
