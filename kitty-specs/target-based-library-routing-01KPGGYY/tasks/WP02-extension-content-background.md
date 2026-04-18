---
work_package_id: WP02
title: 'Chrome Extension: Content Script + Background Script'
dependencies: []
requirement_refs:
- FR-005
- FR-006
- FR-007
- FR-008
- FR-009
- FR-010
- FR-011
- FR-012
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-target-based-library-routing-01KPGGYY
base_commit: 90a033cd27b551753d1e354eb5cc52e44b3e31b1
created_at: '2026-04-18T18:44:26.961483+00:00'
subtasks:
- T006
- T007
- T008
- T009
- T010
- T011
shell_pid: "407621"
agent: "gemini"
history:
- date: '2026-04-18'
  event: created
authoritative_surface: extension/
execution_mode: code_change
owned_files:
- extension/content.js
- extension/background.js
tags: []
---

# WP02 — Chrome Extension: Content Script + Background Script

**Mission**: `target-based-library-routing-01KPGGYY`  
**Branch strategy**: Plan on `main`, merge to `main`. Work in the lane worktree allocated by spec-kitty.  
**No dependencies** — starts immediately, parallelizable with WP01.

## Objective

Make the Content Script detect the active NotebookLM library name from the DOM using MutationObserver (waiting for Angular to render `div.cover-title`) and include it in the HANDSHAKE message. Update the Background Script to store the library name per tab and use it for targeted routing. Never send HANDSHAKE with an empty target.

## Context

### Critical architectural constraints

**NotebookLM is an Angular SPA**. The Content Script executes synchronously before Angular renders any components. `div.cover-title` does NOT exist at script load time — it appears asynchronously after Angular's lifecycle. Sending HANDSHAKE immediately (current code) means `target` would be `undefined`. The fix requires a MutationObserver to wait.

**SPA navigation**: When the user navigates between libraries in the same tab (no page reload), Angular swaps the `div.cover-title` text in-place. The Content Script must detect this mutation and re-send HANDSHAKE with the new library name, keeping the tabRegistry entry fresh.

### Current `content.js` state (lines 16–19)

```js
chrome.runtime.sendMessage({
  type: "HANDSHAKE",
  service: "notebooklm",
});
console.log("[aibbe] Handshake sent for notebooklm");
```

This fires immediately — before Angular renders. It must be replaced.

### Current `background.js` state

**HANDSHAKE handler** (lines 61–77):
```js
chrome.runtime.onMessage.addListener((message, sender) => {
  if (message.type !== "HANDSHAKE") return;
  if (!sender.tab || typeof sender.tab.id !== "number") return;

  tabRegistry.set(sender.tab.id, {
    state: "free",
    service: message.service,
    lastSeen: Date.now(),
    // ← target not stored
  });
  console.log(`${LOG_PREFIX} Tab ${sender.tab.id} registered for ${message.service}`);
});
```

**`findFreeTab`** (lines 88–93):
```js
function findFreeTab() {
  for (const [tabId, entry] of tabRegistry) {
    if (entry.state === "free") return { tabId, entry };
  }
  return null;
}
```

**Routing** (lines 19–45):
```js
const freeTab = findFreeTab();
if (!freeTab) {
  port.postMessage({ status: "error", error: "no_free_tabs" });
  return;
}
```

### DOM selector

**Confirmed from `docs/chat-panel.html`**:
```html
<div class="cover-title mat-headline-medium"> Spec Kitty </div>
```

Selector: `div.cover-title`. Text content: `.trim()` required (Angular may include leading/trailing whitespace). The `mat-headline-medium` class is a Material typography utility — use only `div.cover-title` as the stable selector.

---

## Subtask T006 — Replace immediate HANDSHAKE with MutationObserver Phase 1

**Purpose**: Wait for Angular to render `div.cover-title` before sending HANDSHAKE. Never send with an empty or undefined target.

**File**: `extension/content.js`

**Remove** lines 16–19 (the immediate HANDSHAKE block):
```js
// DELETE THESE LINES:
chrome.runtime.sendMessage({
  type: "HANDSHAKE",
  service: "notebooklm",
});
console.log("[aibbe] Handshake sent for notebooklm");
```

**Add** the following function and its invocation at the top-level (place after the `SELECTORS` constant block, before the utility functions):

```js
const TITLE_SELECTOR = "div.cover-title";
const HANDSHAKE_TIMEOUT_MS = 5000;

function sendHandshake(target) {
  chrome.runtime.sendMessage({
    type: "HANDSHAKE",
    service: "notebooklm",
    target,
  });
  console.log(`[aibbe] Handshake sent: target=${target}`);
}

function watchLibraryTitle(titleElement) {
  // Phase 2 — watches for SPA navigation changes after Phase 1 fires.
  // Implemented in T007.
}

function waitForLibraryTitle() {
  const existing = document.querySelector(TITLE_SELECTOR);
  const existingText = existing?.textContent?.trim();
  if (existingText) {
    sendHandshake(existingText);
    watchLibraryTitle(existing);
    return;
  }

  const timeoutId = setTimeout(() => {
    observer.disconnect();
    console.warn("[aibbe] Timed out waiting for library title — HANDSHAKE not sent");
  }, HANDSHAKE_TIMEOUT_MS);

  const observer = new MutationObserver(() => {
    const el = document.querySelector(TITLE_SELECTOR);
    const text = el?.textContent?.trim();
    if (!text) return;

    clearTimeout(timeoutId);
    observer.disconnect();
    sendHandshake(text);
    watchLibraryTitle(el);
  });

  observer.observe(document.body, { childList: true, subtree: true });
}

waitForLibraryTitle();
```

**Key invariants**:
- `sendHandshake` is only called when `text` is a non-empty string
- The observer disconnects itself after firing (no memory leak)
- The 5-second timeout disconnects the observer and logs a warning

**Validation**:
- [ ] The immediate 3-line HANDSHAKE block is removed
- [ ] `waitForLibraryTitle()` is called at module level
- [ ] Extension reloads without JS errors in the background page console
- [ ] Opening a NotebookLM tab shows the log `[aibbe] Handshake sent: target=<library name>` after a brief delay

---

## Subtask T007 — MutationObserver Phase 2: SPA navigation re-detection

**Purpose**: When the user navigates between libraries within the same tab (Angular route change without page reload), detect the `div.cover-title` text change and re-send HANDSHAKE with the new library name.

**File**: `extension/content.js`

**Fill in `watchLibraryTitle`** (skeleton added in T006):

```js
function watchLibraryTitle(titleElement) {
  const navObserver = new MutationObserver(() => {
    const text = titleElement.textContent?.trim();
    if (!text) return;
    sendHandshake(text);
  });

  navObserver.observe(titleElement, {
    childList: true,
    characterData: true,
    subtree: true,
  });
}
```

**How this works**:
- After Phase 1 fires and sends the initial HANDSHAKE, `watchLibraryTitle` is called with the resolved `div.cover-title` element.
- The `navObserver` watches that element's subtree for content changes.
- When Angular swaps the library name (SPA navigation), the observer fires, reads the new text, and re-sends HANDSHAKE.
- The Background Script's HANDSHAKE handler updates `tabRegistry` with the new `target`, making the tab immediately routable to the new library name.

**Edge case — element replaced entirely**: If Angular replaces the `div.cover-title` element itself (rather than mutating its text), Phase 2 observer will miss it. In that case, the `document.body` observer from Phase 1 would catch the new element appearing. To handle this cleanly, if Phase 2 detects an empty text after a mutation, re-invoke `waitForLibraryTitle()` (with a guard to prevent duplicate registration):

```js
function watchLibraryTitle(titleElement) {
  const navObserver = new MutationObserver(() => {
    const text = titleElement.textContent?.trim();
    if (text) {
      sendHandshake(text);
    } else {
      // Element replaced — re-run Phase 1
      navObserver.disconnect();
      waitForLibraryTitle();
    }
  });

  navObserver.observe(titleElement, {
    childList: true,
    characterData: true,
    subtree: true,
  });
}
```

**Validation**:
- [ ] After initial HANDSHAKE, navigating to a different library (without reload) logs `[aibbe] Handshake sent: target=<new library name>`
- [ ] Background page tabRegistry shows updated `target` after navigation
- [ ] No duplicate/infinite observer loops observed in console

---

## Subtask T008 — Store `target` in tabRegistry on HANDSHAKE

**Purpose**: Persist the library name in the tab registry so the routing function can filter by it.

**File**: `extension/background.js`

**Find the HANDSHAKE handler** (lines 61–77) and add `target`:

```js
// Before
tabRegistry.set(sender.tab.id, {
  state: "free",
  service: message.service,
  lastSeen: Date.now(),
});
console.log(`${LOG_PREFIX} Tab ${sender.tab.id} registered for ${message.service}`);

// After
tabRegistry.set(sender.tab.id, {
  state: "free",
  service: message.service,
  lastSeen: Date.now(),
  target: message.target,   // ← ADD
});
console.log(`${LOG_PREFIX} Tab ${sender.tab.id} registered for ${message.service} target=${message.target}`);
```

**Notes**:
- `message.target` is guaranteed non-empty by the Content Script invariant (T006). The Background Script stores it as-is.
- If a HANDSHAKE arrives with `message.target` undefined (e.g., from a non-updated content script during testing), it stores `undefined` — this is acceptable; `findTargetedTab` won't match undefined against any user-supplied string.
- Re-sent HANDSHAKE (SPA navigation, T007) updates the registry entry — the `set` call overwrites the previous entry, resetting `state` to `"free"` and updating `target` and `lastSeen`.

**Validation**:
- [ ] After a NotebookLM tab registers, `tabRegistry.get(<tabId>).target` shows the library name
- [ ] Re-sent HANDSHAKE updates `target` without leaving stale state
- [ ] `state` is reset to `"free"` on every HANDSHAKE (already the case — just verify)

---

## Subtask T009 — Add `findTargetedTab(target)` function

**Purpose**: Locate the first free tab whose registered library name matches `target`.

**File**: `extension/background.js`

**Add after `findFreeTab`** (line 93):

```js
function findTargetedTab(target) {
  for (const [tabId, entry] of tabRegistry) {
    if (entry.state === "free" && entry.target === target) {
      return { tabId, entry };
    }
  }
  return null;
}
```

**Matching semantics**:
- Exact string equality — `"SIAT" !== "siat"`
- `entry.target` must be a non-empty string (guaranteed by T008 when HANDSHAKE arrives from updated content script)
- Returns `null` when no matching free tab exists (target not open, all matching tabs busy)

**Multiple tabs with same target**: Returns the first match in insertion order (Map preserves insertion order in JS). This matches FR-012.

**Validation**:
- [ ] Function exists and is accessible in the background page
- [ ] Returns correct tab when one free matching tab exists
- [ ] Returns `null` when target doesn't match any tab
- [ ] Returns `null` when matching tab is `"busy"`

---

## Subtask T010 — Update routing to branch on `message.target`

**Purpose**: Use `findTargetedTab` when the incoming request specifies a target; fall back to `findFreeTab` when it does not.

**File**: `extension/background.js`

**Find the routing code** in `port.onMessage.addListener` (around line 22–44):

```js
// Before
const freeTab = findFreeTab();
if (!freeTab) {
  port.postMessage({ status: "error", error: "no_free_tabs" });
  return;
}
const { tabId, entry } = freeTab;
```

**After** (replace the block above):
```js
const tab = message.target
  ? findTargetedTab(message.target)
  : findFreeTab();

if (!tab) {
  const error = message.target ? "target_not_found" : "no_free_tabs";
  port.postMessage({ status: "error", error });
  return;
}

const { tabId, entry } = tab;
```

**Logic table**:

| `message.target` | Tab found | Action |
|------------------|-----------|--------|
| empty/absent | yes | Route via `findFreeTab` |
| empty/absent | no | `{ error: "no_free_tabs" }` |
| non-empty | yes | Route via `findTargetedTab` |
| non-empty | no | `{ error: "target_not_found" }` |

**Backward compatibility**: When `-target` is omitted by the CLI, `message.target` is `undefined` (the JSON key is absent). `undefined` is falsy in JS — the ternary falls through to `findFreeTab()`. Existing behavior preserved identically.

**Validation**:
- [ ] Untargeted request routes to first free tab (existing behavior)
- [ ] Targeted request routes to correct tab
- [ ] `no_free_tabs` error preserved for untargeted requests with no free tabs
- [ ] `target_not_found` error returned for targeted requests with no match

---

## Subtask T011 — Add `target_not_found` error response

**Purpose**: Verify the error path from T010 produces the correct structured error and is returned to the CLI.

**File**: `extension/background.js`

This subtask is mostly a verification and documentation task — the error code itself is set in T010. The work here is to confirm the error propagates correctly end-to-end.

**Verify error propagation**:

The `port.postMessage({ status: "error", error: "target_not_found" })` call in T010 flows through the Native Messaging port back to the daemon, which writes it to the Unix socket, which the CLI reads and prints to stdout before exiting 1.

**Trace the path**:
1. `background.js`: `port.postMessage({ status: "error", error: "target_not_found" })`
2. Native Messaging: 4-byte LE prefix + JSON → daemon stdin
3. `daemon/main.go`: reads response from extension, writes to Unix socket
4. `cmd/cli/main.go`: reads from socket, writes to `os.Stdout`, exits

Expected CLI output:
```
{"status":"error","error":"target_not_found"}
```

**Manual test**:
1. Have at least one NotebookLM tab open with library "SIAT"
2. Run: `go run cmd/cli/main.go -cmd "generate" -target "DoesNotExist" -payload "test"`
3. Verify stdout: `{"status":"error","error":"target_not_found"}`
4. Verify CLI exit code: `echo $?` → `1`

**Validation**:
- [ ] `target_not_found` error string matches exactly (case-sensitive, snake_case)
- [ ] CLI stdout contains the JSON error object
- [ ] CLI exits with code 1
- [ ] Existing `no_free_tabs` path is untouched

---

## Branch Strategy

Planning base: `main`. Merge target: `main`.

Worktree for this WP is allocated by `spec-kitty next`. Do not manually create branches. When your work is complete, run:

```bash
spec-kitty agent action implement WP02 --agent <name>
```

## Definition of Done

- [ ] Immediate HANDSHAKE removed from `content.js` (lines 16–19)
- [ ] `waitForLibraryTitle()` implemented with 5-second timeout and Phase 1 MutationObserver
- [ ] `watchLibraryTitle()` implemented for Phase 2 SPA navigation re-detection
- [ ] HANDSHAKE message includes `target` field (non-empty string guaranteed)
- [ ] `tabRegistry` entries include `target` field
- [ ] `findTargetedTab(target)` implemented with exact-match semantics
- [ ] Routing branches on `message.target` presence
- [ ] `target_not_found` error returned when targeted tab not found
- [ ] Manual test: two tabs with different libraries, both register and route correctly
- [ ] Manual test: SPA navigation updates registry and new library routes correctly
- [ ] Backward compat: untargeted CLI invocation still routes to first free tab

## Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Angular replaces `div.cover-title` element on navigation (not just mutates text) | Medium | Phase 2 fallback re-invokes `waitForLibraryTitle()` on empty text detection |
| `div.cover-title` selector changes in future NotebookLM update | Medium | Selector isolated in `TITLE_SELECTOR` constant — single change point |
| Memory leak from unconnected MutationObserver | Low | Phase 1 observer disconnects itself after firing; Phase 2 has no explicit disconnect but is scoped to the tab lifecycle |
| Extension not reloaded after code change during manual test | Low | After loading updated extension, reload all NotebookLM tabs |

## Reviewer Guidance

- Confirm Phase 1 observer is disconnected after firing (no lingering `document.body` observer)
- Confirm Phase 2 observer targets the resolved element, not `document.body`
- Confirm `sendHandshake` is never called with empty or undefined `target`
- Confirm the routing ternary handles `undefined` (absent JSON key) correctly as falsy
- Confirm `findTargetedTab` uses strict equality (`===`), not loose (`==`)
- Manual test: open DevTools on background page, check `tabRegistry` entries after loading two NotebookLM tabs

## Activity Log

- 2026-04-18T18:44:27Z – claude – shell_pid=405401 – Assigned agent via action command
- 2026-04-18T18:45:14Z – claude – shell_pid=405401 – Ready for review: MutationObserver HANDSHAKE with target, findTargetedTab, target_not_found error
- 2026-04-18T18:45:42Z – gemini – shell_pid=407621 – Started review via action command
