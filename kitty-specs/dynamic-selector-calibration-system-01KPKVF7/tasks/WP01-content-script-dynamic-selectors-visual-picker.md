---
work_package_id: WP01
title: 'Content Script: Dynamic Selector System + Visual Picker'
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-005
- FR-007
- FR-008
- FR-009
- FR-010
- FR-011
- FR-012
planning_base_branch: main
merge_target_branch: main
branch_strategy: Branch from main. Merge back into main when WP01 and WP02 are both complete.
subtasks:
- T001
- T002
- T003
- T004
- T005
- T006
- T007
- T008
- T009
history:
- date: '2026-04-19'
  agent: spec-kitty.tasks
  action: created
authoritative_surface: extension/
execution_mode: code_change
owned_files:
- extension/content.js
tags: []
---

# WP01 — Content Script: Dynamic Selector System + Visual Picker

## Branch Strategy

- **Planning base**: `main`
- **Merge target**: `main`
- **Worktree**: allocated by `finalize-tasks` → `spec-kitty next --agent <name> --mission dynamic-selector-calibration-system-01KPKVF7`
- **No dependencies**: start immediately from `main`

To begin implementation:
```bash
spec-kitty agent action implement WP01 --agent <name>
```

## Objective

Transform `extension/content.js` from a file with a static `SELECTORS` constant into a runtime-configurable system. Selectors are loaded from `chrome.storage.local` on init and re-loaded on demand. A Visual Picker mode lets an operator click page elements to generate CSS selectors.

**This WP owns `extension/content.js` exclusively.** Do not touch `extension/background.js` or any Go files.

## Context

### Current State (content.js)

```javascript
const SELECTORS = {
  INPUT: 'body > labs-tailwind-root > ... > textarea',
  SUBMIT_BUTTON: 'body > ... > button',
  RESPONSE_CONTAINER: 'div.chat-panel-content > ...',
  RESPONSE_TEXT: 'div.message-text-content, ...',
  THINKING_MARKERS: '.thinking-message, thinking-animation',
  RESPONSE_READY_MARKERS: '.message-actions, .xap-copy-to-clipboard, [aria-label*="copy"]',
  CITATION_NOISE: 'button.citation-marker, ...',
  CODE_BLOCK: 'code, pre',
};
```

`SELECTORS` is referenced in:
- `extractCleanTextFromNode` → `SELECTORS.CITATION_NOISE`
- `inspectLatestResponse` → `SELECTORS.RESPONSE_CONTAINER`, `SELECTORS.RESPONSE_TEXT`, `SELECTORS.THINKING_MARKERS`, `SELECTORS.RESPONSE_READY_MARKERS`, `SELECTORS.CODE_BLOCK`
- `injectAndSubmit` → `SELECTORS.INPUT`, `SELECTORS.SUBMIT_BUTTON`
- `waitForAIResponse` → `SELECTORS.SUBMIT_BUTTON`

### Interface Contract (read these files before implementing)

- `kitty-specs/dynamic-selector-calibration-system-01KPKVF7/contracts/internal-messages.md` — message types this WP must handle
- `kitty-specs/dynamic-selector-calibration-system-01KPKVF7/contracts/ipc-commands.md` — shapes of `get-active-selectors` response
- `kitty-specs/dynamic-selector-calibration-system-01KPKVF7/data-model.md` — `CalibrationStore` and `ActiveSelectorMap` data structures

### Key Invariant

`DEFAULT_SELECTORS` is immutable. `activeSelectors` is the mutable runtime map. All code in the file reads from `activeSelectors`, never from `DEFAULT_SELECTORS` directly after init.

---

## Subtask T001 — Rename SELECTORS → DEFAULT_SELECTORS; declare activeSelectors

**Purpose**: Establish the two-variable pattern: one immutable default, one mutable runtime map.

**Steps**:

1. Rename the existing `const SELECTORS = { ... }` to `const DEFAULT_SELECTORS = { ... }`. Keep all values identical — this is a rename only.

2. Immediately after `DEFAULT_SELECTORS`, declare:
   ```javascript
   let activeSelectors = { ...DEFAULT_SELECTORS };
   ```

3. Do NOT update any `SELECTORS.X` references yet — that is T003's job.

**Files**: `extension/content.js`

**Validation**:
- [ ] `DEFAULT_SELECTORS` is `const`, `activeSelectors` is `let`
- [ ] `DEFAULT_SELECTORS` values are identical to the original `SELECTORS`
- [ ] `activeSelectors` is initialized as a shallow copy of `DEFAULT_SELECTORS`

---

## Subtask T002 — Implement async loadSelectors()

**Purpose**: Create the function that reads calibrations from persistent storage and merges them over the defaults.

**Steps**:

1. Add the following function after the `activeSelectors` declaration:

   ```javascript
   async function loadSelectors() {
     try {
       const stored = await chrome.storage.local.get("aibbe_calibrations");
       const calibrations = stored.aibbe_calibrations ?? {};
       activeSelectors = { ...DEFAULT_SELECTORS };
       for (const [key, value] of Object.entries(calibrations)) {
         if (key in DEFAULT_SELECTORS) {
           activeSelectors[key] = String(value);
         }
       }
       console.log("[aibbe] Selectors loaded — calibrations:", Object.keys(calibrations));
     } catch (err) {
       console.warn("[aibbe] loadSelectors failed, using defaults:", err.message);
       activeSelectors = { ...DEFAULT_SELECTORS };
     }
   }
   ```

**Key rules**:
- Always start from a fresh copy of `DEFAULT_SELECTORS` (line: `activeSelectors = { ...DEFAULT_SELECTORS }`)
- Only copy keys that exist in `DEFAULT_SELECTORS` (guard: `if (key in DEFAULT_SELECTORS)`)
- Cast to `String(value)` to prevent non-string values from entering the map
- Never throw — always fall back to defaults on any error

**Files**: `extension/content.js`

**Validation**:
- [ ] `loadSelectors()` is `async`
- [ ] Always resets to `DEFAULT_SELECTORS` before applying calibrations
- [ ] Unknown keys in storage are silently ignored
- [ ] Errors are caught and logged; function never throws

---

## Subtask T003 — Update all SELECTORS.X references → activeSelectors.X; call loadSelectors() at init

**Purpose**: Wire up the dynamic map so all existing logic reads from `activeSelectors`.

**Steps**:

1. Do a global find-and-replace in `content.js`: `SELECTORS.` → `activeSelectors.`
   - Verify: `extractCleanTextFromNode`, `inspectLatestResponse`, `injectAndSubmit`, `waitForAIResponse` all use `activeSelectors.*` now.

2. At the bottom of the file (after the `waitForLibraryTitle` call and before the message listener), call `loadSelectors()`:

   ```javascript
   // Initialize dynamic selectors from storage before handling any commands
   loadSelectors().catch(err => console.warn("[aibbe] Initial selector load failed:", err.message));
   ```

   Note: this is fire-and-forget at init. For strict correctness, place this call before `waitForLibraryTitle()` so selectors are ready when the first command arrives. Since `waitForLibraryTitle` runs a MutationObserver (async), the selector load will have completed by the time any command is processed.

3. The existing `chrome.runtime.onMessage.addListener` already has a handler for `message.cmd === "generate"`. Ensure this handler still works correctly after the rename — it calls `injectAndSubmit` and `waitForAIResponse` which now use `activeSelectors`.

**Files**: `extension/content.js`

**Validation**:
- [ ] Zero remaining `SELECTORS.` references (run: `grep -n "SELECTORS\." extension/content.js | grep -v "DEFAULT_SELECTORS\|activeSelectors"`)
- [ ] `loadSelectors()` is called at script init
- [ ] `generate` command still works (selectors are loaded before the extension processes commands)

---

## Subtask T004 — Register UPDATE_SELECTORS internal message listener

**Purpose**: Allow `background.js` to tell this content script to reload its selectors after a calibration.

**Steps**:

1. In the existing `chrome.runtime.onMessage.addListener` callback, add a handler for the `UPDATE_SELECTORS` type. Place it BEFORE the existing `message.cmd === "generate"` check:

   ```javascript
   chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
     if (message.type === "UPDATE_SELECTORS") {
       loadSelectors()
         .then(() => console.log("[aibbe] Selectors reloaded from UPDATE_SELECTORS"))
         .catch(err => console.warn("[aibbe] Selector reload failed:", err.message));
       return; // fire-and-forget — no async response needed
     }

     if (message.cmd === "generate") {
       // ... existing code unchanged ...
     }
   });
   ```

   **Important**: Return `undefined` (not `true`) for `UPDATE_SELECTORS`. Returning `true` would signal that `sendResponse` will be called asynchronously — which it won't be here.

**Files**: `extension/content.js`

**Validation**:
- [ ] `UPDATE_SELECTORS` handler calls `loadSelectors()` and does NOT call `sendResponse`
- [ ] Handler returns `undefined` (no explicit return or `return;`)
- [ ] Existing `generate` handler is unchanged and still returns `true` (async response)

---

## Subtask T005 — Handle get-active-selectors message

**Purpose**: Allow the operator to inspect the current selector map via CLI. Background.js routes this command here as `{cmd: "get-active-selectors"}`.

**Steps**:

1. In the message listener, add a handler for `message.cmd === "get-active-selectors"`:

   ```javascript
   if (message.cmd === "get-active-selectors") {
     (async () => {
       try {
         await loadSelectors(); // ensure state is current
         const stored = await chrome.storage.local.get("aibbe_calibrations");
         const calibrations = stored.aibbe_calibrations ?? {};
         const result = {};
         for (const key of Object.keys(DEFAULT_SELECTORS)) {
           result[key] = {
             value: activeSelectors[key],
             source: key in calibrations ? "calibration" : "default",
           };
         }
         sendResponse({ status: "success", selectors: result });
       } catch (err) {
         sendResponse({ status: "error", error: err.message });
       }
     })();
     return true; // async response
   }
   ```

2. The response shape matches `contracts/ipc-commands.md` — each key has `{value, source}`.

**Files**: `extension/content.js`

**Validation**:
- [ ] Response includes ALL keys from `DEFAULT_SELECTORS`, not just calibrated ones
- [ ] `source` is `"calibration"` for overridden keys, `"default"` for others
- [ ] Returns `true` to signal async response

---

## Subtask T006 — Implement generateSelector(element)

**Purpose**: Given a DOM element, produce the most stable CSS selector that uniquely identifies it.

**Steps**:

1. Add the following function (place it before the Visual Picker logic):

   ```javascript
   function generateSelector(element) {
     // 1. Stable id (not generated by Angular/framework)
     if (element.id && !/^ng-|^cdk-|^mat-|^auto-|\d{5,}/.test(element.id)) {
       return `#${CSS.escape(element.id)}`;
     }

     // 2. Semantic stable attributes
     for (const attr of ["data-testid", "data-test-id", "aria-label", "name", "type"]) {
       const val = element.getAttribute?.(attr);
       if (val && val.length < 60) {
         const sel = `${element.tagName.toLowerCase()}[${attr}="${CSS.escape(val)}"]`;
         if (document.querySelectorAll(sel).length === 1) return sel;
       }
     }

     // 3. Stable CSS classes (filter Angular/Material generated ones)
     const stableClasses = [...element.classList].filter(
       c => c.length > 2 && !/^ng-|^cdk-|^mat-|^_ng|^ng\d|tns-c/.test(c)
     );
     if (stableClasses.length > 0) {
       const sel = "." + stableClasses.slice(0, 3).map(CSS.escape).join(".");
       if (document.querySelectorAll(sel).length === 1) return sel;
       // Try with tag
       const tagSel = `${element.tagName.toLowerCase()}.${stableClasses.slice(0, 2).map(CSS.escape).join(".")}`;
       if (document.querySelectorAll(tagSel).length === 1) return tagSel;
     }

     // 4. Structural fallback: tag + nth-child within nearest stable ancestor
     const parent = element.parentElement;
     if (parent) {
       const index = [...parent.children].indexOf(element) + 1;
       return `${element.tagName.toLowerCase()}:nth-child(${index})`;
     }

     return element.tagName.toLowerCase();
   }
   ```

2. The Angular-generated class pattern `/tns-c/` specifically matches classes like `ng-tns-c1370865089-0` seen in the existing SELECTORS — these are useless as calibration targets.

**Files**: `extension/content.js`

**Validation**:
- [ ] Returns `#id` for elements with stable IDs
- [ ] Returns attribute selector for elements with `data-testid`/`aria-label`
- [ ] Angular-generated classes (`ng-tns-c*`) are excluded from class-based selectors
- [ ] Falls back to `tag:nth-child(n)` when no stable identifier exists
- [ ] Never throws (no unguarded DOM access)

---

## Subtask T007 — Visual Picker: overlay injection + hover detection + tooltip

**Purpose**: When activated, the page shows a blue highlight around the hovered element and a tooltip with the computed selector.

**Steps**:

1. Declare the session state variable at the top of the Visual Picker section:

   ```javascript
   let _pickerSession = null;
   ```

2. Implement `activateVisualPicker(key, sendResponse)`:

   ```javascript
   function activateVisualPicker(key, sendResponse) {
     if (_pickerSession) deactivateVisualPicker(null); // tear down any existing session

     // Highlight box
     const highlight = document.createElement("div");
     highlight.id = "aibbe-picker-highlight";
     highlight.style.cssText = [
       "position:fixed", "pointer-events:none", "z-index:2147483646",
       "border:2px solid #4285f4", "background:rgba(66,133,244,0.12)",
       "border-radius:2px", "transition:all 80ms ease", "box-sizing:border-box",
     ].join(";");

     // Tooltip
     const tooltip = document.createElement("div");
     tooltip.id = "aibbe-picker-tooltip";
     tooltip.style.cssText = [
       "position:fixed", "z-index:2147483647", "background:#1a1a2e", "color:#e0e0ff",
       "padding:5px 10px", "font-size:11px", "font-family:monospace", "border-radius:4px",
       "pointer-events:none", "max-width:480px", "word-break:break-all",
       "box-shadow:0 2px 8px rgba(0,0,0,0.4)",
     ].join(";");

     document.body.appendChild(highlight);
     document.body.appendChild(tooltip);

     const onMouseOver = (e) => {
       const el = e.target;
       if (el === highlight || el === tooltip) return;
       const rect = el.getBoundingClientRect();
       Object.assign(highlight.style, {
         top: `${rect.top + window.scrollY}px`,
         left: `${rect.left + window.scrollX}px`,
         width: `${rect.width}px`,
         height: `${rect.height}px`,
       });
       tooltip.textContent = `[${key}] ${generateSelector(el)}`;
       tooltip.style.top = `${Math.min(rect.bottom + window.scrollY + 6, window.innerHeight - 40)}px`;
       tooltip.style.left = `${Math.min(rect.left + window.scrollX, window.innerWidth - 500)}px`;
     };

     document.addEventListener("mouseover", onMouseOver, true);
     _pickerSession = { highlight, tooltip, onMouseOver, onClick: null, onKeyDown: null, key, sendResponse };
   }
   ```

**Files**: `extension/content.js`

**Validation**:
- [ ] `highlight` and `tooltip` are appended to `document.body`, not shadow DOM
- [ ] Hovering any element moves the highlight and updates the tooltip
- [ ] Tooltip shows `[KEY_NAME] selector-string`
- [ ] `_pickerSession` is set before returning

---

## Subtask T008 — Visual Picker: click-capture + Escape handler + teardown

**Purpose**: Complete the Visual Picker session: capture the click, clean up DOM, send response.

**Steps**:

1. Implement `deactivateVisualPicker(response)`:

   ```javascript
   function deactivateVisualPicker(response) {
     if (!_pickerSession) return;
     const { highlight, tooltip, onMouseOver, onClick, onKeyDown, sendResponse } = _pickerSession;
     document.removeEventListener("mouseover", onMouseOver, true);
     if (onClick) document.removeEventListener("click", onClick, true);
     if (onKeyDown) document.removeEventListener("keydown", onKeyDown, true);
     highlight.remove();
     tooltip.remove();
     _pickerSession = null;
     if (response && sendResponse) sendResponse(response);
   }
   ```

2. Inside `activateVisualPicker`, AFTER setting `_pickerSession`, attach the click and keydown handlers:

   ```javascript
   const onClick = (e) => {
     e.preventDefault();
     e.stopPropagation();
     const el = e.target;
     const selector = generateSelector(el);
     const { key } = _pickerSession;
     deactivateVisualPicker({ status: "success", key, selector });
   };

   const onKeyDown = (e) => {
     if (e.key === "Escape") {
       const { key } = _pickerSession;
       deactivateVisualPicker({ status: "cancelled", key });
     }
   };

   document.addEventListener("click", onClick, true);
   document.addEventListener("keydown", onKeyDown, true);
   _pickerSession.onClick = onClick;
   _pickerSession.onKeyDown = onKeyDown;
   ```

   **Critical**: Teardown must happen BEFORE `sendResponse` is called (inside `deactivateVisualPicker`). The `highlight.remove()` and `tooltip.remove()` calls must precede the response.

**Files**: `extension/content.js`

**Validation**:
- [ ] Clicking an element removes both `#aibbe-picker-highlight` and `#aibbe-picker-tooltip` from the DOM
- [ ] Response after click: `{status: "success", key: "...", selector: "css-string"}`
- [ ] Pressing Escape: `{status: "cancelled", key: "..."}`
- [ ] After teardown, `_pickerSession` is `null`
- [ ] `deactivateVisualPicker(null)` (no response) does NOT call sendResponse

---

## Subtask T009 — Handle ACTIVATE_VISUAL_PICKER + DEACTIVATE_VISUAL_PICKER messages

**Purpose**: Wire the internal message types (sent by background.js) to the Visual Picker functions.

**Steps**:

1. In the `chrome.runtime.onMessage.addListener` callback, add handlers for both types:

   ```javascript
   if (message.type === "ACTIVATE_VISUAL_PICKER") {
     const key = message.key || "UNKNOWN";
     activateVisualPicker(key, sendResponse);
     return true; // async — response comes from click or Escape
   }

   if (message.type === "DEACTIVATE_VISUAL_PICKER") {
     if (_pickerSession) {
       const { key } = _pickerSession;
       deactivateVisualPicker({ status: "cancelled", key });
     }
     sendResponse({ status: "success" });
     return true;
   }
   ```

2. The final handler order in `chrome.runtime.onMessage.addListener` should be:
   1. `UPDATE_SELECTORS` — fire-and-forget, no return true
   2. `ACTIVATE_VISUAL_PICKER` — async, return true
   3. `DEACTIVATE_VISUAL_PICKER` — async, return true
   4. `get-active-selectors` — async, return true
   5. `generate` — async, return true (existing handler)

**Files**: `extension/content.js`

**Validation**:
- [ ] `ACTIVATE_VISUAL_PICKER` calls `activateVisualPicker(message.key, sendResponse)` and returns `true`
- [ ] `DEACTIVATE_VISUAL_PICKER` cancels any active session, responds `{status:"success"}`, returns `true`
- [ ] `DEACTIVATE_VISUAL_PICKER` is idempotent — safe to call when no picker is active

---

## Definition of Done

- [ ] `extension/content.js` passes `go vet` equivalent: no syntax errors (open browser console — zero errors on load)
- [ ] `generate` command still works end-to-end (regression: daemon → extension → NotebookLM)
- [ ] `get-active-selectors` returns the full 8-key map with correct `source` annotations
- [ ] `UPDATE_SELECTORS` reloads selectors without page reload
- [ ] Visual Picker activates on a live NotebookLM tab, highlights on hover, captures on click
- [ ] Pressing Escape during Visual Picker cancels it cleanly (no DOM orphans)
- [ ] No `SELECTORS.` references remain in the file (only `activeSelectors.` and `DEFAULT_SELECTORS.`)

## Risks

| Risk | Mitigation |
|------|-----------|
| `chrome.storage.local` not available in content script | It IS available in MV3 content scripts. Confirmed in research.md R-01. |
| Angular-generated classes contaminating generateSelector output | Filter pattern `/tns-c/` in T006 handles `ng-tns-c*` patterns seen in the current SELECTORS. |
| Visual Picker overlay interfering with page interaction | `pointer-events:none` on highlight + `pointer-events:all` capture via `addEventListener(..., true)` (capture phase) prevents interference. |
| Multiple ACTIVATE_VISUAL_PICKER calls stacking up | `activateVisualPicker` calls `deactivateVisualPicker(null)` at the start — idempotent teardown. |
| Sending response after message channel closes | Teardown happens synchronously inside the click handler before the async response is sent. |

## Reviewer Guidance

1. Verify that **all** `SELECTORS.` references are gone — grep for `SELECTORS\.` (excluding `DEFAULT_SELECTORS` and `activeSelectors`)
2. Check that `loadSelectors()` is called at init AND on `UPDATE_SELECTORS`
3. Confirm the `generate` command (existing functionality) still works end-to-end
4. Test `get-active-selectors` before and after a calibration — verify `source` annotation changes
5. Test Visual Picker: activate, hover, click, Escape, cancel — all paths
