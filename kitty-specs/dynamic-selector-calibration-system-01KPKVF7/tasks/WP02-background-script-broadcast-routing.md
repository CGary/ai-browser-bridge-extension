---
work_package_id: WP02
title: 'Background Script: Broadcast + Command Routing'
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-006
- FR-011
- FR-012
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-dynamic-selector-calibration-system-01KPKVF7
base_commit: 236b68632e699eb66b0c12305ff42b946fc9e1e2
created_at: '2026-04-19T22:22:59.752878+00:00'
subtasks:
- T010
- T011
- T012
- T013
- T014
- T015
shell_pid: "1520960"
agent: "codex"
history:
- date: '2026-04-19'
  agent: spec-kitty.tasks
  action: created
authoritative_surface: extension/
execution_mode: code_change
owned_files:
- extension/background.js
tags: []
---

# WP02 — Background Script: Broadcast + Command Routing

## Branch Strategy

- **Planning base**: `main`
- **Merge target**: `main`
- **Worktree**: allocated by `finalize-tasks` → `spec-kitty next --agent <name> --mission dynamic-selector-calibration-system-01KPKVF7`
- **No dependencies**: start immediately from `main`, parallel with WP01

To begin implementation:
```bash
spec-kitty agent action implement WP02 --agent <name>
```

## Objective

Extend `extension/background.js` to handle the five new IPC commands. Storage-managed commands (`calibrate`, `reset-selectors`) are handled directly in the background service worker — no content script needed. The remaining commands (`get-active-selectors`, `visual-picker-start`, `visual-picker-cancel`) are routed to the appropriate content script tab.

**This WP owns `extension/background.js` exclusively.** Do not touch `extension/content.js` or any Go files.

## Context

### Current State (background.js key section)

```javascript
port.onMessage.addListener(async (message) => {
  const tab = message.target
    ? findTargetedTab(message.target)
    : findFreeTab();

  if (!tab) {
    const error = message.target ? "target_not_found" : "no_free_tabs";
    port.postMessage({ status: "error", error });
    return;
  }

  const { tabId, entry } = tab;
  entry.state = "busy";

  try {
    const response = await chrome.tabs.sendMessage(tabId, {
      cmd: message.cmd,
      payload: message.payload,
    });
    port.postMessage(response);
  } catch (err) {
    port.postMessage({ status: "error", error: err.message });
  } finally {
    if (tabRegistry.has(tabId)) {
      tabRegistry.get(tabId).state = "free";
    }
  }
});
```

**Problem**: This handler routes EVERY command to a single tab. Calibration commands (`calibrate`, `reset-selectors`) need to: (a) write to `chrome.storage.local`, (b) broadcast to ALL tabs — not just one. They never need content script involvement.

### Interface Contract (read before implementing)

- `kitty-specs/dynamic-selector-calibration-system-01KPKVF7/contracts/ipc-commands.md` — all IPC request/response shapes
- `kitty-specs/dynamic-selector-calibration-system-01KPKVF7/contracts/internal-messages.md` — `UPDATE_SELECTORS`, `ACTIVATE_VISUAL_PICKER`, `DEACTIVATE_VISUAL_PICKER` message shapes
- `kitty-specs/dynamic-selector-calibration-system-01KPKVF7/data-model.md` — `CalibrationStore` storage key and schema

### Architecture Decisions

1. **`calibrate` and `reset-selectors` are background-only**: No tab is needed. Background reads/writes `chrome.storage.local` directly and then broadcasts `UPDATE_SELECTORS` to ALL registered tabs.

2. **`get-active-selectors` is delegated to one tab**: Content.js holds the merged `activeSelectors` state. Background routes to any free tab. Content.js returns the annotated map.

3. **Visual Picker commands are targeted**: They require DOM access, so they're routed to the specific tab identified by `message.target`. Background uses `ACTIVATE_VISUAL_PICKER` / `DEACTIVATE_VISUAL_PICKER` internal message types (not `cmd`).

4. **MV3 service worker can be killed**: `tabRegistry` is rebuilt on HANDSHAKE. `chrome.storage.local` is the only durable state. Do not cache calibrations in a background variable.

---

## Subtask T010 — Add broadcastToAllTabs(message) helper

**Purpose**: Utility function to send a message to every registered tab, ignoring errors for individual tabs (fire-and-forget semantics).

**Steps**:

1. Add the following function near the top of `background.js`, after the `tabRegistry` declaration:

   ```javascript
   async function broadcastToAllTabs(message) {
     const sends = [];
     for (const [tabId] of tabRegistry) {
       sends.push(
         chrome.tabs.sendMessage(tabId, message).catch(err => {
           console.warn(`${LOG_PREFIX} broadcastToAllTabs tab ${tabId} error:`, err.message);
         })
       );
     }
     await Promise.allSettled(sends);
   }
   ```

**Key rules**:
- `.catch()` on each individual send — a broken tab must not block the others
- `Promise.allSettled` (not `Promise.all`) — never rejects
- `LOG_PREFIX` is already defined at the top of the file as `"[aibbe]"`

**Files**: `extension/background.js`

**Validation**:
- [ ] Function iterates ALL entries in `tabRegistry`, not just free ones
- [ ] Individual tab errors are caught and logged, not rethrown
- [ ] Function always resolves (never rejects)

---

## Subtask T011 — Handle calibrate IPC command

**Purpose**: Accept new selector overrides, persist them, broadcast to all tabs, respond to CLI.

**Steps**:

1. In `port.onMessage.addListener`, intercept `calibrate` BEFORE the existing tab-routing logic. Add at the top of the handler:

   ```javascript
   if (message.cmd === "calibrate") {
     try {
       let updates;
       try {
         updates = typeof message.payload === "string"
           ? JSON.parse(message.payload)
           : (message.payload ?? {});
       } catch {
         port.postMessage({ status: "error", error: "invalid_payload" });
         return;
       }

       const stored = await chrome.storage.local.get("aibbe_calibrations");
       const existing = stored.aibbe_calibrations ?? {};
       const merged = { ...existing };
       const applied = [];

       const VALID_KEYS = new Set([
         "INPUT", "SUBMIT_BUTTON", "RESPONSE_CONTAINER", "RESPONSE_TEXT",
         "THINKING_MARKERS", "RESPONSE_READY_MARKERS", "CITATION_NOISE", "CODE_BLOCK"
       ]);

       for (const [key, value] of Object.entries(updates)) {
         if (VALID_KEYS.has(key)) {
           merged[key] = String(value);
           applied.push(key);
         } else {
           console.warn(`${LOG_PREFIX} calibrate: unknown key ignored: ${key}`);
         }
       }

       await chrome.storage.local.set({ aibbe_calibrations: merged });
       await broadcastToAllTabs({ type: "UPDATE_SELECTORS", calibrations: merged });

       port.postMessage({ status: "success", applied });
       console.log(`${LOG_PREFIX} calibrate: applied [${applied.join(", ")}]`);
     } catch (err) {
       port.postMessage({ status: "error", error: err.message });
     }
     return; // do not fall through to tab routing
   }
   ```

2. The `VALID_KEYS` set matches exactly the keys in `DEFAULT_SELECTORS` from `content.js`. Keep them in sync.

**Files**: `extension/background.js`

**Validation**:
- [ ] Partial calibration: only specified keys are updated; existing calibrations for other keys are preserved
- [ ] Unknown keys are ignored (with a warning log) and excluded from `applied[]`
- [ ] `aibbe_calibrations` in storage reflects the merged state (previous + new)
- [ ] `UPDATE_SELECTORS` is broadcast to all registered tabs with the full merged calibration object
- [ ] Response: `{status: "success", applied: ["KEY1", "KEY2"]}`
- [ ] Invalid JSON payload returns `{status: "error", error: "invalid_payload"}`

---

## Subtask T012 — Handle reset-selectors IPC command

**Purpose**: Clear all stored calibrations and revert all tabs to factory defaults.

**Steps**:

1. Immediately after the `calibrate` handler, add:

   ```javascript
   if (message.cmd === "reset-selectors") {
     try {
       await chrome.storage.local.remove("aibbe_calibrations");
       await broadcastToAllTabs({ type: "UPDATE_SELECTORS", calibrations: {} });
       port.postMessage({ status: "success" });
       console.log(`${LOG_PREFIX} reset-selectors: all calibrations cleared`);
     } catch (err) {
       port.postMessage({ status: "error", error: err.message });
     }
     return;
   }
   ```

2. Broadcasting `{calibrations: {}}` after clearing ensures content.js resets to defaults even if the storage operation hasn't propagated yet. Content.js merges an empty object over defaults → full defaults restored.

**Files**: `extension/background.js`

**Validation**:
- [ ] `chrome.storage.local.remove("aibbe_calibrations")` is called
- [ ] `UPDATE_SELECTORS` is broadcast with `calibrations: {}`
- [ ] Response: `{status: "success"}`
- [ ] After reset, `get-active-selectors` returns all keys with `source: "default"`

---

## Subtask T013 — Handle get-active-selectors IPC command

**Purpose**: Return the full annotated selector map by delegating to one content script tab.

**Steps**:

1. After the `reset-selectors` handler, add:

   ```javascript
   if (message.cmd === "get-active-selectors") {
     const tab = findFreeTab();
     if (!tab) {
       port.postMessage({ status: "error", error: "no_free_tabs" });
       return;
     }

     const { tabId, entry } = tab;
     entry.state = "busy";

     try {
       const response = await chrome.tabs.sendMessage(tabId, {
         cmd: "get-active-selectors",
         payload: "",
       });
       port.postMessage(response);
     } catch (err) {
       port.postMessage({ status: "error", error: err.message });
     } finally {
       if (tabRegistry.has(tabId)) {
         tabRegistry.get(tabId).state = "free";
       }
     }
     return;
   }
   ```

2. This uses the same one-to-one routing pattern as the existing `generate` command — content.js handles `get-active-selectors` and returns the annotated map.

**Files**: `extension/background.js`

**Validation**:
- [ ] Routes to `findFreeTab()` (not a targeted tab — any registered tab works)
- [ ] Tab state is restored to `"free"` in the `finally` block
- [ ] Response passes through from content.js unchanged (contains the `selectors` object)

---

## Subtask T014 — Handle visual-picker-start IPC command

**Purpose**: Activate the Visual Picker on the specific tab the operator is watching.

**Steps**:

1. After the `get-active-selectors` handler, add:

   ```javascript
   if (message.cmd === "visual-picker-start") {
     let payload;
     try {
       payload = typeof message.payload === "string"
         ? JSON.parse(message.payload)
         : (message.payload ?? {});
     } catch {
       port.postMessage({ status: "error", error: "invalid_payload" });
       return;
     }

     const key = payload.key || "UNKNOWN";
     const tab = message.target
       ? findTargetedTab(message.target)
       : findFreeTab();

     if (!tab) {
       const error = message.target ? "target_not_found" : "no_free_tabs";
       port.postMessage({ status: "error", error });
       return;
     }

     const { tabId, entry } = tab;
     entry.state = "busy";

     try {
       const response = await chrome.tabs.sendMessage(tabId, {
         type: "ACTIVATE_VISUAL_PICKER",
         key,
       });
       port.postMessage(response);
     } catch (err) {
       port.postMessage({ status: "error", error: err.message });
     } finally {
       if (tabRegistry.has(tabId)) {
         tabRegistry.get(tabId).state = "free";
       }
     }
     return;
   }
   ```

2. Note: uses `type: "ACTIVATE_VISUAL_PICKER"` (internal message type) not `cmd`. Content.js (WP01) handles this type in its `chrome.runtime.onMessage` listener.

**Files**: `extension/background.js`

**Validation**:
- [ ] Uses `ACTIVATE_VISUAL_PICKER` internal message type (not `cmd: "visual-picker-start"`)
- [ ] `key` from payload is forwarded to content.js
- [ ] Response from content.js (`{status: "success", key, selector}` or `{status: "cancelled", key}`) is passed through to the native port
- [ ] Error: `{status: "error", error: "target_not_found"}` if no matching tab

---

## Subtask T015 — Handle visual-picker-cancel IPC command

**Purpose**: Cancel an active Visual Picker session on the target tab.

**Steps**:

1. After the `visual-picker-start` handler, add:

   ```javascript
   if (message.cmd === "visual-picker-cancel") {
     const tab = message.target
       ? findTargetedTab(message.target)
       : findFreeTab();

     if (!tab) {
       // If no tab found, the picker isn't active anyway — succeed silently
       port.postMessage({ status: "success" });
       return;
     }

     const { tabId, entry } = tab;
     entry.state = "busy";

     try {
       const response = await chrome.tabs.sendMessage(tabId, {
         type: "DEACTIVATE_VISUAL_PICKER",
       });
       port.postMessage(response);
     } catch (err) {
       // If the tab is unresponsive, the picker is effectively cancelled
       port.postMessage({ status: "success" });
     } finally {
       if (tabRegistry.has(tabId)) {
         tabRegistry.get(tabId).state = "free";
       }
     }
     return;
   }
   ```

2. This command is idempotent — succeeds even if no picker was active. This matches the `contracts/ipc-commands.md` spec for `visual-picker-cancel`.

**Files**: `extension/background.js`

**Validation**:
- [ ] Uses `DEACTIVATE_VISUAL_PICKER` internal message type
- [ ] Succeeds (`{status: "success"}`) even when no tab is found
- [ ] Succeeds even when content.js tab is unresponsive (catch block)

---

## Final Handler Order

After all subtasks are complete, the handler in `port.onMessage.addListener` must dispatch in this order:

1. `calibrate` → storage + broadcast (T011)
2. `reset-selectors` → storage clear + broadcast (T012)
3. `get-active-selectors` → route to free tab (T013)
4. `visual-picker-start` → route to target tab via ACTIVATE_VISUAL_PICKER (T014)
5. `visual-picker-cancel` → route to target tab via DEACTIVATE_VISUAL_PICKER (T015)
6. *(everything else)* → existing tab routing logic (generate, etc.)

---

## Definition of Done

- [ ] `extension/background.js` loads without errors in the extension
- [ ] `calibrate` persists to storage and broadcasts to all registered tabs
- [ ] `reset-selectors` clears storage and broadcasts
- [ ] `get-active-selectors` returns the annotated map from content.js
- [ ] `visual-picker-start` activates the picker on the target tab
- [ ] `visual-picker-cancel` cancels any active picker session
- [ ] Existing `generate` command still works (not broken by the new handlers)
- [ ] All new handlers use `return;` to prevent fall-through to the existing routing logic

## Risks

| Risk | Mitigation |
|------|-----------|
| `tabRegistry` is empty at calibration time | Commands still write/clear storage. When a tab registers (HANDSHAKE), it calls `loadSelectors()` which reads the updated storage. Broadcast is a best-effort enhancement. |
| MV3 service worker terminated between calibrate and the broadcast | The storage write happens before the broadcast. Next tab init will read updated storage via `loadSelectors()`. |
| `get-active-selectors` called when all tabs are busy | Returns `{status: "error", error: "no_free_tabs"}`. Operator can retry — tabs clear to "free" after each command. |
| `visual-picker-start` timeout (operator doesn't click) | The CLI blocks waiting for a response. This is an operator-side timeout issue; no mitigation needed in the extension (the spec does not define a picker timeout). |

## Reviewer Guidance

1. Confirm that `calibrate` and `reset-selectors` do NOT use `findFreeTab()` or `findTargetedTab()` — they must not route to a tab at all.
2. Verify broadcast uses `broadcastToAllTabs` (all tabs) not `findFreeTab` (one tab).
3. Check that all new handlers end with `return;` — critical to prevent fall-through to existing routing.
4. Test: calibrate with 1 key, verify other 7 keys unchanged in `get-active-selectors` response.
5. Test: `reset-selectors` after calibrating, verify `get-active-selectors` shows all as `"default"`.

## Activity Log

- 2026-04-19T22:29:24Z – unknown – shell_pid=1520960 – Moved to for_review
- 2026-04-19T22:29:29Z – codex – shell_pid=1520960 – Started review via action command
