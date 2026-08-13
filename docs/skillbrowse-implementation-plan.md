# skillbrowse — Implementation Plan

## Context

`docs/skillbrowse-project-brief.md` and `docs/superpowers/specs/2026-08-12-skillbrowse-design.md` define a fully-specified but unbuilt product: a read-only terminal app that discovers `SKILL.md` files across AI-agent tool directories, presents a searchable catalog, and renders skill content — plus a signed self-upgrade mechanism. The repository currently has no code, `go.mod`, or CI. This plan sequences the build from empty repo to a 1.0-ready codebase, following the design doc's package boundaries (§8.1), delivery outline (brief §"Delivery outline"), functional/non-functional requirements (§7, §10), and testing strategy (§14).

Local toolchain check: Go 1.26.1 is installed (matches the spec's baseline) and `gh` is available; `goreleaser` is not installed yet (needed only for Phase 6).

The plan is organized as 6 phases, each independently shippable/testable, mirroring the design doc's own ordering (catalog core → interactive experience → updater/release → validation) but broken into finer milestones so each is a reviewable unit of work.

## Phase 0 — Project scaffolding

- `go.mod` (module path TBD — ask user if not obvious from a GitHub remote), Go 1.26.
- Directory skeleton for all ten packages listed in design doc §8.1: `cmd/skillbrowse`, `internal/{config,sources,discovery,skill,catalog,ui,markdown,update,buildinfo}`.
- `.golangci.yml` (or equivalent) for static analysis, matching design doc §15 ("formatting, static analysis, tests, race tests, vulnerability checks").
- `Makefile` or `justfile` with `build`, `test`, `test-race`, `lint`, `vuln` targets — this becomes the canonical "commands" section to add back into `CLAUDE.md` once real commands exist.
- `internal/buildinfo`: version/commit/date/Go-version/OS/arch struct, populated via `-ldflags` at build time; wired to `skillbrowse version` (FR-12).
- `go.mod` module path: `github.com/dchancogne/skillbrowse` (also the compiled-in repo identity for the self-updater's GitHub Releases lookup).
- `cmd/skillbrowse/main.go` using **Cobra** for command routing: root command (default TUI launch) plus `upgrade` (`--check`, `--yes`), `version`, and `help`; root persistent flags `--config`, `--path` (repeatable), `--no-defaults`, `--no-color`.

## Phase 1 — Catalog core (discovery → parsing → normalization)

This is the design doc's own Phase 1 ("catalog core") and has zero UI dependency, so it's independently testable via unit/integration tests before any TUI exists.

1. **`internal/config`** — TOML loading (`version = 1`, `[[sources]]` with `path`/`label`/`agents`/`max_depth`/`enabled`), validation per design doc §5.2 rules (reject relative paths, `~` only as first component, `max_depth` 1–12 default 4), default path resolution (`$XDG_CONFIG_HOME/skillbrowse/config.toml` else `~/.config/skillbrowse/config.toml`). Malformed config → fatal error with file/field/correction guidance (exit code 2).
2. **`internal/sources`** — built-in registry table from design doc §5.1 (Agent Skills, Claude Code ×2 roots, Cursor, Codex ×2 roots, Hermes) as a small isolated descriptor list + fixture-based tests, merged with validated custom sources and `--path`/`--no-defaults` CLI overrides.
3. **`internal/discovery`** — bounded concurrent filesystem walker per source root: skip `.git`/`node_modules`/`vendor`, resolve root symlink but don't recursively follow directory symlinks (only accept a symlink whose immediate target contains `SKILL.md`), depth-bounded, canonical-path/inode dedup to prevent cycles, worker-pool concurrency, `context`-based cancellation for quit/rescan (NFR-07). Missing built-in roots silent; unreadable roots → source diagnostic.
4. **`internal/skill`** — YAML front-matter parser (`name`, `description`, whitespace-normalized), fallbacks (dir name; first paragraph truncated to 280 Unicode chars; `"No description provided"`), invalid front matter → diagnostic without hiding the skill, 2 MiB size cap (list with diagnostic, don't load content beyond cap).
5. **`internal/catalog`** — merge candidates by canonical path into the `Skill` record shape from design doc §5.5 (ID hash, Name, Description, CanonicalPath, SkillFilePath, ObservedPaths[], Agents[] union sorted+unique, SourceLabels[], ModifiedAt, Content, Diagnostics[]), deterministic default sort (case-insensitive name then canonical path), fuzzy search over concatenated searchable fields (name/description/agents/labels/paths) — evaluate a small fuzzy-match library vs hand-rolled (recommend `github.com/sahilm/fuzzy`, already common in Bubble Tea ecosystem apps).

**Exit criteria (maps to FR-01–05):** unit tests for each package per design doc §14.1; integration tests with temp fixture trees per §14.3 (all built-in source families, duplicate-via-symlink, mixed valid/invalid/oversized/unreadable). A CLI-less test harness (call the packages directly) proves the whole pipeline before any UI exists.

## Phase 2 — Interactive TUI

1. **`internal/markdown`** — Glamour v2 wrapper: sanitize untrusted control sequences before rendering, width-aware render cache (keyed by content hash + width) so resize doesn't re-render everything.
2. **`internal/ui`** — Bubble Tea v2 root model wired to catalog snapshots delivered via messages (never touches the filesystem directly, per design doc §8.1). Build in this order:
   - Wide layout (≥100 cols): list pane (Bubbles `list`) + detail pane (Bubbles `viewport` + Glamour).
   - Narrow layout (<100 cols): full-screen list ↔ full-screen detail, `Enter`/`Esc` transitions, state (filter/selection/scroll) preserved across the resize boundary (FR-06).
   - Search (Bubbles `textinput`, `/` to focus, fuzzy filter from `internal/catalog`, highlight matches where supported).
   - Detail view: metadata block + rendered/raw (`v`) toggle, scroll retention across toggle where possible (FR-08).
   - Rescan (`r`): re-run discovery pipeline, replace catalog, preserve stable selection (FR-09), footer shows scan status/warning count.
   - Help overlay (`?`) listing active bindings without discarding state (FR-10).
   - Minimum-size fallback message for unusably small terminals.
   - `NO_COLOR`/`--no-color` handling via Lip Gloss capability detection.
3. Wire `cmd/skillbrowse` default command to launch this model.

**Exit criteria:** golden terminal-view tests (wide/narrow/empty/warning/search/help/light/dark/no-color per §14.2) using a Bubble Tea test harness (`teatest` or equivalent); pseudo-terminal e2e tests for navigation/search/resize/detail/raw-toggle/rescan/help/quit (§14.4).

## Phase 3 — Self-updater

1. **`internal/update`** — GitHub `GET /repos/{owner}/{repo}/releases/latest` lookup (explicit API version header, bounded timeout, repo identity compiled in), strict OS/arch asset-name matching, no prerelease selection.
2. Verification chain (design doc §12.3, in this exact order): download archive+checksum-manifest+signature to bounded temp files → verify Ed25519 signature over manifest (embedded public key + key ID, with rotation support for dual-key releases) → verify SHA-256 digest against signed manifest → safe extraction (reject absolute paths, `..`, symlinks, multiple executable candidates, oversized entries) → confirm staged binary reports expected version → atomic same-directory rename preserving permissions.
3. Failure-path invariant: any failure before the final rename must leave the current executable byte-for-byte untouched — this needs an explicit test (§14.4 "rollback invariant test"), not just happy-path coverage.
4. Wire `skillbrowse upgrade [--check] [--yes]` and the TUI's `u` key (confirmation prompt unless `--yes` / non-interactive).

**Exit criteria:** local fake release server covering the 9 cases in design doc §14.3 (no-update, success, timeout, corrupt archive, bad checksum, bad signature, wrong asset, non-writable target, interrupted download).

## Phase 4 — Diagnostics, CLI polish, error taxonomy

- Implement the three error classes from design doc §9 consistently: fatal startup (exit 2 syntax/config, exit 1 environment), source diagnostics (footer warning count + diagnostics view), skill diagnostics (visible record + warning marker).
- `SKILLBROWSE_DEBUG=1` stderr diagnostic mode.
- Home-relative path display; never leak skill content or stack traces in default error output.
- `skillbrowse version` full field set (FR-12); exit-code contract from design doc §4.

## Phase 5 — Performance validation

- Synthetic fixture generator (committed) for 1,000 and 10,000-skill trees.
- Benchmarks recording Go version/OS/arch/filesystem/terminal-size, asserting NFR-01–04 (100ms frame, 500ms/1k-skill catalog readiness, 50ms interaction latency, <100MiB memory for 10k skills).
- Goroutine-leak check on quit/rescan cancellation (NFR-07), e.g. via `goleak`.

## Phase 6 — Release pipeline

- GoReleaser config: macOS/Linux × amd64/arm64 archives, SHA-256 checksums, checksum-manifest signing step, provenance/SBOM generation.
- GitHub Actions: format/lint/test/race/govulncheck/cross-build on PR; release workflow adds clean-machine launch smoke tests before publishing.
- Install script performing the same signature/checksum verification as the in-app updater (design doc §15).
- User docs: configuration, keys, diagnostics, privacy, upgrading, uninstalling (Definition of Done, design doc §16).
- Update root `CLAUDE.md` with real build/lint/test commands once they exist (currently placeholder per earlier session).

## Confirmed decisions

- **Module path**: `github.com/dchancogne/skillbrowse`.
- **CLI routing**: Cobra.
- **Fuzzy-match library**: recommend `github.com/sahilm/fuzzy` (common in Bubble Tea ecosystem apps) — open to alternatives if one turns out to fit poorly during Phase 1.

## Verification approach (applies across phases)

- Each phase ends with `go test ./... -race` green plus the specific fixture/golden/e2e suites named in its exit criteria.
- Phase 1 is verifiable headlessly (no terminal needed) — run it standalone before investing in TUI work, to catch discovery/parsing bugs cheaply.
- Phase 2 or later: manually run `go run ./cmd/skillbrowse` against a real `~/.claude/skills` (or a scripted fixture tree) to sanity-check the actual terminal experience, per the repo's guidance to test UI changes live before declaring done.
- `govulncheck ./...` and `go vet ./...` as a standing gate from Phase 0 onward.
