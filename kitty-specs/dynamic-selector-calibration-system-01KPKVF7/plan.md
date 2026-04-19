# Implementation Plan: Dynamic Selector Calibration System

**Branch**: `main` | **Date**: 2026-04-19 | **Spec**: [spec.md](./spec.md)
**Input**: `kitty-specs/dynamic-selector-calibration-system-01KPKVF7/spec.md`

## Summary

Replace the static `SELECTORS` constant in the Chrome extension with a runtime-configurable selector map backed by `chrome.storage.local`. New IPC commands (`calibrate`, `reset-selectors`, `get-active-selectors`) let the operator update selectors from the CLI with immediate, no-reload propagation to all registered tabs. A separate Visual Picker mode activates an in-page highlight overlay that turns a click into a CSS selector string, returned to the CLI.

No changes to the Go daemon or CLI are required — the daemon already forwards any `cmd`+`payload` to Native Messaging transparently.

## Technical Context

**Language/Version**: Go 1.22 (daemon/CLI, no changes) · Vanilla JavaScript ES2020 (Chrome Extension MV3)
**Primary Dependencies**: `chrome.storage.local`, `chrome.runtime.onMessage`, `chrome.tabs.sendMessage` (all built-in MV3 APIs)
**Storage**: `chrome.storage.local` — stores calibration overrides under key `aibbe_calibrations`
**Testing**: Go table-driven tests (`go test ./...`); extension JS tested via manual extension reload + CLI commands
**Target Platform**: Linux (daemon) · Chrome / Chromium 120+ (extension MV3)
**Project Type**: Single repo, two distinct runtimes — Go daemon (unchanged) + Chrome extension (modified)
**Performance Goals**: Calibration propagation to all registered tabs ≤ 5 seconds · CLI round-trip ≤ 5 seconds
**Constraints**: No external network calls · No changes to manifest.json or extension ID · Storage footprint ≤ 50 KB

## Charter Check

*Charter not present — section skipped. Governance: software-dev-default, DIR-001, DIR-002.*

## Project Structure

### Documentation (this feature)

```
kitty-specs/dynamic-selector-calibration-system-01KPKVF7/
├── plan.md              ← this file
├── research.md          ← Phase 0 output
├── data-model.md        ← Phase 1 output
├── quickstart.md        ← Phase 1 output
├── contracts/
│   ├── ipc-commands.md  ← IPC message formats (CLI ↔ Daemon ↔ Extension)
│   └── internal-messages.md  ← Background ↔ Content Script message formats
└── tasks.md             ← Phase 2 output (/spec-kitty.tasks — NOT created here)
```

### Source Code (repository root)

```
extension/
├── content.js           ← PRIMARY CHANGE: dynamic selectors, calibration handlers, visual picker
└── background.js        ← PRIMARY CHANGE: broadcast capability, new command routing

# No changes:
daemon/
cmd/cli/
internal/
```

**Structure Decision**: Chrome extension only (two files). No Go changes. No new files in the extension — all logic added to existing `content.js` and `background.js`.

## Lane Architecture

This feature is split into two independently executable lanes. Both target the same two files but touch different sections. A shared **Interface Contract** (see `contracts/`) must be merged before implementing either lane.

### Lane A — Calibration Core

**Files**: `extension/content.js` (storage + command handlers) · `extension/background.js` (broadcast)

**Scope**:
1. Replace `const SELECTORS` with `let activeSelectors`
2. Add `loadSelectors()`: reads `aibbe_calibrations` from `chrome.storage.local`, merges with code defaults
3. Call `loadSelectors()` on script init and on `UPDATE_SELECTORS` message receipt
4. Handle `calibrate` command: persist overrides, trigger `UPDATE_SELECTORS` broadcast, respond `{status: "success"}`
5. Handle `reset-selectors` command: clear `aibbe_calibrations`, trigger broadcast, respond `{status: "success"}`
6. Handle `get-active-selectors` command: return full map with `source` annotation per key
7. In `background.js`: broadcast `UPDATE_SELECTORS` to all registered tabs (not just one free tab) for calibration commands

**Prerequisite**: Interface Contract committed.

### Lane B — Visual Picker

**Files**: `extension/content.js` (overlay UI) · `extension/background.js` (route to target tab)

**Scope**:
1. Handle `visual-picker-start` command: activate highlight mode on the targeted tab
2. Highlight mode: inject overlay CSS, attach `mouseover`/`mouseout`/`click` event listeners
3. On hover: compute optimal CSS selector for the element under cursor, show tooltip
4. On click: capture selector, remove overlay, respond `{status: "success", key, selector}` to CLI
5. Handle `visual-picker-cancel` command: tear down overlay without selecting, respond `{status: "cancelled"}`
6. In `background.js`: route `visual-picker-start`/`visual-picker-cancel` to the tab matching `req.Target`

**Prerequisite**: Interface Contract committed. Does NOT require Lane A to be merged — uses only the agreed selector key names.

### Shared Interface Contract (must land before either Lane)

Defined in `contracts/`. Establishes:
- `aibbe_calibrations` storage key format
- `UPDATE_SELECTORS` internal message schema
- All IPC command request/response shapes
- Visual Picker command shapes

## Complexity Tracking

*No Charter Check violations to justify.*
