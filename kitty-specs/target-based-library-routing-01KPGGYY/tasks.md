# Tasks: Target-based Library Routing

**Mission**: `target-based-library-routing-01KPGGYY`  
**Branch**: `main → main`  
**Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)

---

## Subtask Index

| ID | Description | WP | Parallel |
|----|-------------|-----|---------|
| T001 | Add `Target string` field to `ipc.Request` struct | WP01 | [P] |
| T002 | Update ipc package tests for `Target` field serialization | WP01 | [P] |
| T003 | Add `-target` string flag to CLI and pass to `Request` | WP01 | [P] |
| T004 | Update CLI integration tests for `-target` flag | WP01 | [P] |
| T005 | Update daemon log line to include `target` when present | WP01 | [P] |
| T006 | Replace immediate HANDSHAKE with MutationObserver Phase 1 in `content.js` | WP02 | [P] |
| T007 | Add MutationObserver Phase 2 for SPA navigation re-detection in `content.js` | WP02 | [P] |
| T008 | Store `target` in tabRegistry on HANDSHAKE in `background.js` | WP02 | [P] |
| T009 | Add `findTargetedTab(target)` function to `background.js` | WP02 | [P] |
| T010 | Update routing in `background.js` to branch on `message.target` | WP02 | [P] |
| T011 | Add `target_not_found` error response path in `background.js` | WP02 | [P] |

---

## Work Packages

### WP01 — Go Backend: IPC + CLI + Daemon

**Priority**: High  
**Estimated prompt size**: ~320 lines  
**Dependencies**: none  
**Parallelizable with**: WP02 (no file overlap)  
**Prompt file**: [tasks/WP01-go-backend-ipc-cli.md](tasks/WP01-go-backend-ipc-cli.md)

**Goal**: Extend the Go IPC contract and CLI to support a `-target` flag that carries the library name through the daemon transparently.

**Included subtasks**:

- [ ] T001 Add `Target string` field to `ipc.Request` struct (WP01)
- [ ] T002 Update ipc package tests for `Target` field serialization (WP01)
- [ ] T003 Add `-target` string flag to CLI and pass to `Request` (WP01)
- [ ] T004 Update CLI integration tests for `-target` flag (WP01)
- [ ] T005 Update daemon log line to include `target` when present (WP01)

**Implementation sketch**:
1. Add `Target string \`json:"target,omitempty"\`` to `ipc.Request`
2. Update existing table-driven tests in `internal/ipc/` to cover Target serialization/omission
3. Add `flag.String("target", "", "...")` in `cmd/cli/main.go`, include in `ipc.Request{}`
4. Update `cmd/cli/` test suite to exercise empty and non-empty `-target`
5. Update daemon INFO log format to include `target=%s` when `target` is non-empty

**Success criteria**: `go test ./...` and `go vet ./...` pass with no regressions.

---

### WP02 — Chrome Extension: Content Script + Background Script

**Priority**: High  
**Estimated prompt size**: ~380 lines  
**Dependencies**: none  
**Parallelizable with**: WP01 (no file overlap)  
**Prompt file**: [tasks/WP02-extension-content-background.md](tasks/WP02-extension-content-background.md)

**Goal**: Make the Content Script detect the active NotebookLM library from the DOM using MutationObserver and include it in the HANDSHAKE; update the Background Script to store and match targets in tab routing.

**Included subtasks**:

- [ ] T006 Replace immediate HANDSHAKE with MutationObserver Phase 1 in `content.js` (WP02)
- [ ] T007 Add MutationObserver Phase 2 for SPA navigation re-detection in `content.js` (WP02)
- [ ] T008 Store `target` in tabRegistry on HANDSHAKE in `background.js` (WP02)
- [ ] T009 Add `findTargetedTab(target)` function to `background.js` (WP02)
- [ ] T010 Update routing in `background.js` to branch on `message.target` (WP02)
- [ ] T011 Add `target_not_found` error response path in `background.js` (WP02)

**Implementation sketch**:
1. Remove the 3-line immediate HANDSHAKE from `content.js` (lines 16–19)
2. Add `waitForLibraryTitle()` using MutationObserver on `document.body`: fire HANDSHAKE once when `div.cover-title` has non-empty `textContent`, with 5 s timeout
3. After firing, switch observer to watch `div.cover-title` directly for `characterData`/`childList` mutations (Phase 2 — SPA navigation)
4. In `background.js` HANDSHAKE handler: add `target: message.target` to the stored registry entry
5. Add `findTargetedTab(target)` that returns first free tab with matching `target`
6. In `port.onMessage` handler: branch on `message.target`; use `findTargetedTab` or `findFreeTab` accordingly; emit `target_not_found` error when targeted tab not found

**Success criteria**: Manual verification — two NotebookLM tabs with different library names both register correctly; targeted CLI query routes to the correct tab.
