# Implementation Plan: Target-based Library Routing

**Branch**: `main` | **Date**: 2026-04-18 | **Spec**: [spec.md](spec.md)  
**Mission**: `target-based-library-routing-01KPGGYY` (`01KPGGYYTN0BT3K5ZCZSTG1WEX`)  
**Merge target**: `main`

## Summary

Add a `-target` flag to the CLI so users can route queries to a specific NotebookLM library when multiple tabs are open. The Go backend adds one field to the IPC struct and one CLI flag. The Chrome Extension reads the library title from the DOM using MutationObserver, registers it in the tab registry, and routes requests by target match. The two implementation lanes are fully independent and can execute in parallel.

## Technical Context

**Language/Version**: Go 1.21+ (backend), JavaScript ES2020 (Chrome Extension MV3)  
**Primary Dependencies**: Go stdlib only; Chrome Extension APIs (`chrome.runtime`, `chrome.tabs`)  
**Storage**: In-memory only (tabRegistry Map in Background Script; no `chrome.storage.*`)  
**Testing**: Go — table-driven tests (`go test ./...`); Extension — manual browser test (no automated harness)  
**Target Platform**: Linux daemon + Chromium MV3 Extension  
**Performance Goals**: HANDSHAKE sent within 5 s of DOM readiness (NFR-001)  
**Constraints**: Max 1 MB IPC message; Daemon logic unchanged; No disk persistence

## Charter Check

*Charter absent — skipped.*

## Lane Architecture

This feature decomposes into **two fully independent lanes** with a single shared JSON contract (`target` field) defined upfront.

```
Contract boundary: { "target": "<library name>" } in IPC JSON
        │
        ├── Lane A: Go Backend ──────────────────────────────────────────
        │   internal/ipc/ipc.go   Add Target field to Request struct
        │   cmd/cli/main.go        Add -target flag
        │   (daemon/main.go)       Optional: log target field
        │
        └── Lane B: Chrome Extension ────────────────────────────────────
            extension/content.js    MutationObserver + HANDSHAKE with target
            extension/background.js tabRegistry target field + targeted routing
```

**Why they're independent**: Lane A serializes `target` into JSON. Lane B reads `message.target` from JSON. They share only the field name — defined in the spec and contracts. Neither lane requires the other's code to compile, test, or validate.

**Integration**: When both lanes are merged, end-to-end routing works automatically — no glue code.

## Project Structure

### Documentation

```
kitty-specs/target-based-library-routing-01KPGGYY/
├── plan.md           ← This file
├── spec.md
├── research.md       ← Phase 0 output
├── data-model.md     ← Phase 1 output
├── contracts/
│   ├── ipc-request.md
│   └── handshake.md
└── tasks/            ← Created by /spec-kitty.tasks
```

### Source (files touched per lane)

```
Lane A — Go Backend
├── internal/ipc/ipc.go          Request.Target field (1 line)
├── cmd/cli/main.go              -target flag + marshal (3 lines)
└── daemon/main.go               log line (optional, 1 line)

Lane B — Extension
├── extension/content.js         MutationObserver HANDSHAKE (±30 lines)
└── extension/background.js      tabRegistry + routing (±20 lines)
```

## Phase 0: Research

*Complete.* See [research.md](research.md). No unknowns remain.

Key findings:
- DOM selector confirmed: `div.cover-title` (from `docs/chat-panel.html`)
- MutationObserver two-phase strategy: appearance (Phase 1) + mutation watch (Phase 2)
- Daemon requires zero routing changes — forwards JSON bytes transparently
- `omitempty` on `Target` field preserves full backward compatibility

## Phase 1: Design & Contracts

*Complete.* See [data-model.md](data-model.md) and [contracts/](contracts/).

### Lane A Design

**`internal/ipc/ipc.go`** — Add one field:
```go
type Request struct {
    Cmd     string `json:"cmd"`
    Target  string `json:"target,omitempty"`  // ← ADD
    Payload string `json:"payload"`
}
```

**`cmd/cli/main.go`** — Add flag and pass to Request:
```go
target := flag.String("target", "", "target library name (optional)")
// ...
ipc.Request{Cmd: *cmd, Target: *target, Payload: *payload}
```

**`daemon/main.go`** (optional) — Update log format to include target.

### Lane B Design

**`extension/content.js`** — Replace immediate HANDSHAKE with observer-based approach:

```js
// Phase 1: wait for div.cover-title to appear with content
// Phase 2: watch for content changes (SPA navigation)
// Invariant: HANDSHAKE only sent when target is non-empty string
// Timeout: 5 s, then log warning and stop
```

**`extension/background.js`** — Two changes:

1. Store `target` in registry on HANDSHAKE:
```js
tabRegistry.set(sender.tab.id, {
  state: "free",
  service: message.service,
  lastSeen: Date.now(),
  target: message.target,   // ← ADD
});
```

2. Add `findTargetedTab` and branch routing:
```js
function findTargetedTab(target) {
  for (const [tabId, entry] of tabRegistry) {
    if (entry.state === "free" && entry.target === target) return { tabId, entry };
  }
  return null;
}

// In port.onMessage handler:
const tab = message.target
  ? findTargetedTab(message.target)
  : findFreeTab();

if (!tab) {
  const error = message.target ? "target_not_found" : "no_free_tabs";
  port.postMessage({ status: "error", error });
  return;
}
```

## Complexity Tracking

No charter violations.

## Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| `div.cover-title` selector changes with NotebookLM UI update | Medium | Selector is in `SELECTORS` constant — single change point |
| HANDSHAKE sent before title renders | High | MutationObserver Phase 1 guards this — never sends with empty target |
| Target stale after SPA navigation | High | MutationObserver Phase 2 re-sends HANDSHAKE on mutation |
| Tab busy when target matches | Low | Documented behavior: returns `target_not_found` |

## Merge Strategy

Both lanes produce isolated diffs with no file overlap. Either can merge first. Recommended order: **Lane A first** (smaller, lower risk), then **Lane B** — but strictly optional since there are no conflicts.

---

**Branch**: `main → main`  
**Next step**: `/spec-kitty.tasks`
