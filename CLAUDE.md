# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository status

This repository currently contains only planning documents — there is no source code yet. The product being planned is **`skillbrowse`**, a read-only, keyboard-first terminal application (macOS and Linux) for discovering and reading AI-agent skills (`SKILL.md` files) installed across tools like Claude Code, Cursor, Codex, Hermes, and generic `~/.agents` layouts.

The authoritative specs are:

- `docs/skillbrowse-project-brief.md` — product brief: problem, scope, UX principles, success measures.
- `docs/superpowers/specs/2026-08-12-skillbrowse-design.md` — full product requirements and technical design (UX detail, CLI surface, discovery model, package architecture, error handling, performance/security requirements, testing strategy, release process).

Read both before starting implementation — the design doc is the detailed source of truth and the brief is its companion summary. There is no `go.mod`, build tooling, or test suite yet, so there are no build/lint/test commands to run until that scaffolding is created.

## Planned architecture (from the design doc)

Once implementation begins, the design specifies a modular, in-memory Go application (Go 1.26) using the Charm v2 ecosystem — Bubble Tea v2 (app state/events), Bubbles v2 (list/input/viewport/spinner/help components), Lip Gloss v2 (responsive layout/styling), and Glamour v2 (Markdown rendering). No database; GoReleaser + GitHub Releases handle distribution.

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
```

Data flow: built-in registry + validated custom sources → discovery scanner → metadata parser → catalog normalizer → immutable catalog snapshot → Bubble Tea UI (rescan loops back to the scanner; explicit update requests go to the updater → GitHub Releases).

Key design constraints to preserve when implementing:

- **Read-only in v1**: never install, edit, delete, or upgrade skills themselves — only the `skillbrowse` binary self-upgrades.
- **No network on ordinary startup/browsing/rescan** — network access is explicit and user-initiated (`u` / `upgrade` command) only.
- **Resilience**: a malformed skill or unreadable source must never abort scanning of the rest; missing built-in roots are silent, unreadable existing roots produce a diagnostic.
- **Security for self-upgrade**: SHA-256 checksum manifest + Ed25519 signature verification with an embedded public key, safe archive extraction (reject `..`, absolute paths, symlinks, extra executables), atomic rename, no shell execution of downloaded content.
- **Untrusted content**: `SKILL.md` files are treated as untrusted text — never executed/templated, and terminal control sequences are sanitized before rendering.
- Config file: `$XDG_CONFIG_HOME/skillbrowse/config.toml` (or `~/.config/skillbrowse/config.toml`), `version = 1`, TOML `[[sources]]` entries with `path`, optional `label`/`agents`/`max_depth` (1–12, default 4)/`enabled`.
