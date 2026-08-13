# skillbrowse — Project Brief

**Date:** August 12, 2026  
**Status:** Approved for implementation planning  
**Product:** `skillbrowse`  
**Platforms:** macOS and Linux

## Executive summary

`skillbrowse` is a fast, keyboard-first terminal application for finding and reading AI-agent skills installed on a developer's machine. It scans well-known skill locations and user-configured paths, presents a searchable catalog, identifies the agents and source locations associated with each skill, and renders the selected `SKILL.md` without leaving the terminal.

The first release is deliberately read-only. It will not install, edit, remove, or update skills. It will include a secure self-upgrade mechanism for the `skillbrowse` binary itself.

## Problem

AI coding tools store reusable skills in different directories and installation layouts. A developer using several agents can accumulate overlapping, duplicated, or forgotten skills across `~/.agents`, `~/.claude`, `~/.cursor`, `~/.codex`, `~/.hermes`, and plugin caches. Today, inspecting that inventory usually means remembering each layout, searching the filesystem manually, and opening individual Markdown files.

The result is poor visibility: developers cannot quickly answer which skills are installed, where they came from, which agent locations expose them, or what instructions they contain.

## Product promise

> Run one command, see every locally installed skill, and read its instructions immediately.

## Primary user

A developer or AI power user who works with two or more terminal-based coding agents and maintains skills in both standard and custom directories.

## User outcomes

- Understand the local skill inventory in seconds.
- Find a skill by name, description, agent, or path.
- See duplicate exposure across agents without losing source-path information.
- Read rendered or raw `SKILL.md` content in the terminal.
- Diagnose malformed or unreadable skill installations.
- Upgrade `skillbrowse` safely without reinstalling it manually.

## Version 1 scope

### Included

- Default interactive TUI launched by `skillbrowse`.
- Concurrent scanning of well-known and custom skill sources.
- Built-in support for generic Agents, Claude Code, Cursor, Codex, and Hermes layouts.
- Extensible TOML configuration for additional paths and agent labels.
- Normalized metadata: name, description, agents, source paths, modification time, and warnings.
- Fuzzy search across name, description, agent, and path.
- Responsive split-pane layout with a narrow-terminal fallback.
- Rendered Markdown detail view plus a raw-source toggle.
- Manual rescan, help overlay, and diagnostics.
- Explicit, signed, atomic application self-upgrades from GitHub Releases.
- Prebuilt macOS and Linux binaries for `amd64` and `arm64`.

### Excluded

- Installing, editing, deleting, or upgrading skills.
- A remote skill registry or marketplace.
- Background indexing or a persistent database.
- Automatic network access at startup.
- Usage telemetry.
- Windows support.

## Experience principles

1. **Local first:** Browsing never requires the network and never sends skill content elsewhere.
2. **Useful under imperfection:** One malformed skill or inaccessible directory must not prevent browsing the rest.
3. **Keyboard native:** Core tasks require no mouse and follow familiar terminal conventions.
4. **Transparent:** Every catalog entry shows how it was found and which paths contributed to it.
5. **Read-only by design:** Viewing a skill cannot change it.

## Proposed experience

On launch, the TUI begins scanning and shows results as they become available. Wide terminals show a catalog on the left and the selected skill's metadata and rendered `SKILL.md` on the right. Narrow terminals show a full-screen list; `Enter` opens the detail reader and `Esc` returns.

Primary keys are `↑/↓` or `j/k` to navigate, `/` to search, `Enter` to focus or open details, `Esc` to return, `r` to rescan, `v` to toggle rendered/raw content, `u` to check for an application update, `?` for help, and `q` to quit.

## Technical direction

Use a modular, in-memory Go architecture:

- **Bubble Tea v2** for application state and terminal events.
- **Bubbles v2** for list, text input, viewport, spinner, and help components.
- **Lip Gloss v2** for responsive layout and styling.
- **Glamour v2** for Markdown rendering.
- Focused internal packages for configuration, source registration, discovery, parsing, catalog normalization, the TUI, and upgrades.
- GoReleaser and GitHub Releases for multi-platform delivery.

No database is needed for the expected local dataset. Package boundaries preserve the option to add sources or a non-interactive output mode later without coupling them to the TUI.

## Success measures

The first release succeeds when:

- At least 95% of valid `SKILL.md` fixtures across supported layouts are discovered and attributed correctly.
- A 1,000-skill local fixture becomes usable within 500 ms at the 95th percentile on a 2022-era laptop with an SSD.
- Navigation and filtering update within 50 ms at the 95th percentile.
- A malformed skill produces an entry or diagnostic without terminating the scan.
- A failed or interrupted upgrade leaves the current executable usable.
- All supported OS/architecture release artifacts pass installation and launch smoke tests.
- No network request occurs during ordinary browsing or rescanning.

Because v1 has no telemetry, product feedback will come from opt-in user reports, issue templates, and repeatable local benchmarks rather than hidden collection.

## Delivery outline

1. Build the catalog core: configuration, source adapters, discovery, parsing, normalization, and diagnostics.
2. Build the interactive experience: list, search, responsive details, Markdown rendering, rescan, and help.
3. Add the secure updater and release pipeline.
4. Validate performance, terminal compatibility, signed artifacts, and cross-platform installation.

## Principal risks and mitigations

| Risk | Mitigation |
|---|---|
| Agent directory conventions change | Keep source layouts in a small registry with fixture-based tests and allow custom sources. |
| Duplicate or symlinked installations confuse attribution | Canonicalize targets, merge identical paths, and preserve every observed source and agent label. |
| Large or malformed Markdown harms responsiveness | Cap readable file size, parse asynchronously, cache rendered content per width, and expose warnings. |
| Self-update damages the installation | Verify signed checksums, stage beside the executable, and replace atomically only after all checks pass. |
| Color or layout is unreadable in some terminals | Detect capabilities, support no-color mode, and fall back to a single-pane layout. |

## Decision summary

- The product name and executable are `skillbrowse`.
- Version 1 targets macOS and Linux.
- Discovery combines built-in and custom paths.
- The primary experience is arrow navigation plus fuzzy search.
- Only the application, not installed skills, is upgradeable in v1.
- The chosen implementation is a modular in-memory Go application using the Charm v2 ecosystem.

