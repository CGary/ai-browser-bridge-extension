# Research: Target-based Library Routing

## DOM Selector — Library Title

**Decision**: Use `div.cover-title` as the primary selector for the NotebookLM library title.

**Rationale**: Confirmed from live DOM inspection (`docs/chat-panel.html`). The element is:
```html
<div class="cover-title mat-headline-medium"> Spec Kitty </div>
```
The `cover-title` class is the stable semantic class. `mat-headline-medium` is a Material typography utility class that may change with theming updates, so `div.cover-title` alone is the robust selector.

**Alternative considered**: Using the tab `<title>` element. Rejected because NotebookLM updates the `<title>` asynchronously and with a different format that includes the app name ("Spec Kitty - NotebookLM"), requiring brittle string parsing.

---

## MutationObserver for Angular SPA Title Detection

**Decision**: Use `MutationObserver` on `document.body` with `childList: true, subtree: true` to detect when `div.cover-title` appears and when its text content changes.

**Rationale**: NotebookLM is an Angular SPA. The Content Script executes synchronously before Angular finishes rendering. The `div.cover-title` element does not exist at script load time — it is inserted by Angular's component lifecycle. `MutationObserver` is the standard browser API for detecting async DOM changes without polling.

**Two-phase observation strategy**:
1. **Phase 1 — Appearance**: Observe `document.body` until `div.cover-title` exists and has non-empty `textContent`. Fire HANDSHAKE once. Switch to Phase 2.
2. **Phase 2 — Navigation**: Observe `div.cover-title` directly for `characterData` and `childList` changes. Re-fire HANDSHAKE when content changes (SPA library navigation).

**Alternative considered**: `requestAnimationFrame` loop. Rejected — polling wastes CPU and has no deterministic stop condition for SPA navigation.

**Alternative considered**: `DOMContentLoaded` / `window.onload`. Rejected — these fire before Angular finishes rendering components.

**Timeout**: 5 seconds maximum wait (per NFR-001). After timeout, log a warning and do not send HANDSHAKE — the tab remains unregistered.

---

## IPC `Request` Struct Extension

**Decision**: Add `Target string` with `json:"target,omitempty"` to the `Request` struct in `internal/ipc/ipc.go`.

**Rationale**: `omitempty` ensures backward compatibility — existing tests and CLI invocations that do not set `-target` produce JSON without the `target` key, matching the current wire format exactly. Daemon forwards JSON bytes without inspection (verified in `daemon/main.go` line ~179), so no daemon changes are needed for routing.

**Alternative considered**: Passing target as a prefix in `Payload` (e.g., `SIAT::question`). Rejected explicitly in the ideas document — fragile, requires escape logic, wrong layer.

---

## Background Script Routing Logic

**Decision**: Add `findTargetedTab(target)` alongside the existing `findFreeTab()`. The main message handler branches on `message.target`:
- If `message.target` is non-empty → `findTargetedTab(message.target)`
- If `message.target` is empty/absent → `findFreeTab()` (existing behavior, zero regression)

**Rationale**: Preserves existing behavior for `-target`-less invocations. Minimal surface area change.

**Error for missing target**: Return `{ status: "error", error: "target_not_found" }` when `findTargetedTab` returns null. This mirrors the existing `no_free_tabs` error pattern in `background.js`.

---

## Daemon Log Enhancement

**Decision**: Optional — update the daemon's `[INFO] received` log line to include `target` when present.

**Rationale**: The daemon does not need to parse the JSON for routing. The log enhancement is purely observability. It is a non-breaking, one-line change that aids debugging. Marked as optional — it does not affect correctness.

---

## Lane Independence Analysis

| Lane | Files | External Dependencies |
|------|-------|-----------------------|
| A — Go Backend | `internal/ipc/ipc.go`, `cmd/cli/main.go` | None outside Go stdlib |
| B — Extension | `extension/content.js`, `extension/background.js` | NotebookLM DOM (confirmed selector) |

The two lanes share only the JSON message contract (`target` field). Because both sides of the contract are defined in the spec (and confirmed here), no coordination is needed during implementation — Lane A and Lane B can proceed fully in parallel.

**Integration point**: When both lanes are merged, the end-to-end flow works automatically. The Go side serializes `target` into JSON; the JS side reads it from `message.target`. No glue code required.
