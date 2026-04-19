# Tasks: Dynamic Selector Calibration System

**Branch**: `main` → merge target: `main`
**Mission**: `dynamic-selector-calibration-system-01KPKVF7`

---

## Interface Contract (agreed — foundation for parallel execution)

Both WPs share this contract, fully specified in `contracts/`.

| Boundary | Contract |
|----------|----------|
| **Storage key** | `aibbe_calibrations` in `chrome.storage.local` — flat JSON `{key: cssSelector}` |
| **UPDATE_SELECTORS** | Background → all tabs: `{type:"UPDATE_SELECTORS", calibrations:{...}}` |
| **ACTIVATE_VISUAL_PICKER** | Background → target tab: `{type:"ACTIVATE_VISUAL_PICKER", key}` → responds `{status,key,selector}` or `{status:"cancelled",key}` |
| **DEACTIVATE_VISUAL_PICKER** | Background → target tab: `{type:"DEACTIVATE_VISUAL_PICKER"}` → responds `{status:"success"}` |
| **IPC responses** | All defined in `contracts/ipc-commands.md` |

**WP01 consumes**: `UPDATE_SELECTORS`, `ACTIVATE_VISUAL_PICKER`, `DEACTIVATE_VISUAL_PICKER` from background
**WP02 produces**: those same messages; handles `calibrate`/`reset-selectors` directly (no tab needed); routes `get-active-selectors`/`visual-picker-*` to content via the agreed message types

---

## Subtask Index

| ID | Description | WP | Parallel |
|----|-------------|----|---------|
| T001 | Rename `SELECTORS` → `DEFAULT_SELECTORS`; add `let activeSelectors` | WP01 | [P] | [D] |
| T002 | Implement async `loadSelectors()` with chrome.storage.local | WP01 | [D] |
| T003 | Update all SELECTORS.X refs → activeSelectors.X; call loadSelectors() at init | WP01 | [D] |
| T004 | Register UPDATE_SELECTORS internal message listener | WP01 | [D] |
| T005 | Handle `get-active-selectors` message — annotated map response | WP01 | [D] |
| T006 | Implement `generateSelector(element)` — CSS selector generation | WP01 | [D] |
| T007 | Visual Picker: overlay injection + hover detection + tooltip | WP01 | [D] |
| T008 | Visual Picker: click-capture + Escape handler + teardown | WP01 | [D] |
| T009 | Handle ACTIVATE_VISUAL_PICKER + DEACTIVATE_VISUAL_PICKER messages | WP01 | [D] |
| T010 | Add `broadcastToAllTabs(message)` helper in background.js | WP02 | [D] |
| T011 | Handle `calibrate` IPC — write storage + broadcast + respond | WP02 | [D] |
| T012 | Handle `reset-selectors` IPC — clear storage + broadcast + respond | WP02 | [D] |
| T013 | Handle `get-active-selectors` IPC — route to tab, relay response | WP02 | [D] |
| T014 | Handle `visual-picker-start` IPC — route ACTIVATE_VISUAL_PICKER to target tab | WP02 | [P] |
| T015 | Handle `visual-picker-cancel` IPC — route DEACTIVATE_VISUAL_PICKER to target tab | WP02 | [P] |

All `[P]` across WP01/WP02 = safe to execute in parallel (different files, no shared state).

---

## Work Packages

### WP01 — Content Script: Dynamic Selector System + Visual Picker

**Priority**: High | **Estimated size**: ~480 lines | **Prompt**: `tasks/WP01-content-script-dynamic-selectors-visual-picker.md`
**Owned**: `extension/content.js` | **Dependencies**: none | **Lane**: A (independent, no blocking deps)

- [x] T001 Rename `SELECTORS` → `DEFAULT_SELECTORS`; add `let activeSelectors` (WP01)
- [x] T002 Implement async `loadSelectors()` with chrome.storage.local (WP01)
- [x] T003 Update all SELECTORS.X refs → activeSelectors.X; call loadSelectors() at init (WP01)
- [x] T004 Register UPDATE_SELECTORS internal message listener (WP01)
- [x] T005 Handle `get-active-selectors` message — annotated map response (WP01)
- [x] T006 Implement `generateSelector(element)` — CSS selector generation (WP01)
- [x] T007 Visual Picker: overlay injection + hover detection + tooltip (WP01)
- [x] T008 Visual Picker: click-capture + Escape handler + teardown (WP01)
- [x] T009 Handle ACTIVATE_VISUAL_PICKER + DEACTIVATE_VISUAL_PICKER messages (WP01)

**Implementation sketch**:
1. Phase 1 — Dynamic Selector Infrastructure (T001–T005): Replace static constant, add storage layer, update all references, handle calibration reads
2. Phase 2 — Visual Picker (T006–T009): selector generation, DOM overlay, event capture, session lifecycle

**Risks**: Content script is `"use strict"` — all new async init must be safe. Angular-generated class names must be filtered in generateSelector.

---

### WP02 — Background Script: Broadcast + Command Routing

**Priority**: High | **Estimated size**: ~330 lines | **Prompt**: `tasks/WP02-background-script-broadcast-routing.md`
**Owned**: `extension/background.js` | **Dependencies**: none | **Lane**: B (independent, no blocking deps)

- [x] T010 Add `broadcastToAllTabs(message)` helper in background.js (WP02)
- [x] T011 Handle `calibrate` IPC — write storage + broadcast + respond (WP02)
- [x] T012 Handle `reset-selectors` IPC — clear storage + broadcast + respond (WP02)
- [x] T013 Handle `get-active-selectors` IPC — route to tab, relay response (WP02)
- [ ] T014 Handle `visual-picker-start` IPC — route ACTIVATE_VISUAL_PICKER to target tab (WP02)
- [ ] T015 Handle `visual-picker-cancel` IPC — route DEACTIVATE_VISUAL_PICKER to target tab (WP02)

**Implementation sketch**:
1. Add broadcast helper (T010)
2. Storage-handled commands (T011–T012) — background does these directly, no tab needed
3. Tab-delegated commands (T013–T015) — background routes to content.js via chrome.tabs.sendMessage

**Risks**: MV3 service worker can be terminated between events — `tabRegistry` is rebuilt on HANDSHAKE; `chrome.storage.local` is the only reliable persistent state.

---

## Execution Order

```
[contracts/ipc-commands.md + contracts/internal-messages.md already committed]

WP01 (extension/content.js) ──────────────────┐
                                               ├── merge when both complete
WP02 (extension/background.js) ───────────────┘
```

No sequential dependencies. Both WPs can start from `main` in separate worktrees.
