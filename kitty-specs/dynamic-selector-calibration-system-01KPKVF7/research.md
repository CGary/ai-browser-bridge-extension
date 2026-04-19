# Research: Dynamic Selector Calibration System

**Date**: 2026-04-19
**Phase**: 0 — Outline & Research

---

## R-01: chrome.storage.local — Selector Map Persistence

**Decision**: Use `chrome.storage.local` with a single key `aibbe_calibrations` storing a flat JSON object (selector key → CSS selector string).

**Rationale**:
- `chrome.storage.local` persists across browser restarts with no server dependency.
- Flat object (not nested) keeps reads/writes atomic and the structure self-documenting.
- A single storage key means one read on init; no pagination or cursor needed.
- Storage quota is 10 MB for MV3 — well above the 50 KB constraint.

**Alternatives considered**:
- `chrome.storage.sync`: Shares across Chrome profiles on the same Google Account. Rejected — calibrations are machine-specific (tied to local DOM inspection). Sync quota (100 KB) also more restrictive.
- Individual keys per selector (e.g., `aibbe_INPUT`, `aibbe_SUBMIT_BUTTON`): Rejected — requires enumerating all keys on read, harder to atomic-clear on reset.
- `localStorage` inside content script: Rejected — not accessible from background service worker; lost on page reload.

**Learned**: `chrome.storage.local` is async (Promise-based in MV3). `loadSelectors()` must be `async` and all callers must `await` it. On init, the content script must `await loadSelectors()` before processing any incoming command.

---

## R-02: Broadcast Pattern — Background to All Registered Tabs

**Decision**: On calibration commands, iterate `tabRegistry` (already maintained in `background.js`) and call `chrome.tabs.sendMessage(tabId, {type: "UPDATE_SELECTORS", calibrations})` for each registered tab. Fire-and-forget — do not wait for individual tab ACKs.

**Rationale**:
- The existing `tabRegistry` in `background.js` is the authoritative list of tabs running the content script. No new discovery mechanism needed.
- Fire-and-forget is acceptable: if a tab is in a broken state, the calibration is still in `chrome.storage.local` and will be loaded on next init.
- A single `Promise.allSettled` over all `sendMessage` calls allows logging failures without blocking the CLI response.

**Alternatives considered**:
- `chrome.tabs.query` + `executeScript` to push updates: More invasive, requires `scripting` permission in manifest. Rejected.
- Pub/sub via `chrome.runtime.sendMessage`: Content scripts can receive broadcasts, but requires all content scripts to listen on `chrome.runtime.onMessage` rather than just responding to tab-targeted messages. Architecture change too large. Rejected.

**Learned**: Background service workers in MV3 can be terminated by Chrome between events. `tabRegistry` is in-memory; it is rebuilt as tabs re-send HANDSHAKE. Calibrations must be in `chrome.storage.local` (not only in the background's memory) so they survive service worker restarts.

---

## R-03: CSS Selector Generation for Visual Picker

**Decision**: Implement a minimal `generateSelector(element)` function in `content.js` that builds a CSS selector using this priority: `#id` > stable `[data-*]` attributes > `.className` (shortest unique class) > tag + nth-child fallback. No external library.

**Rationale**:
- The extension ships as raw JS with no build step. Adding a library (e.g., `finder`, `optimal-select`) requires a bundler or inline copy — both increase complexity.
- The existing codebase uses no external JS dependencies.
- For the use case (operator choosing a visible element), a "good enough" selector (unique in the current DOM) is sufficient. The operator visually verifies it before confirming.

**Selector quality tiers** (tried in order):
1. `#id` — if element has a non-generated `id`
2. `[data-testid="…"]` or `[aria-label="…"]` — stable semantic attributes
3. Shortest unique CSS class combination
4. `tagName:nth-child(n)` within nearest identified ancestor — fallback

**Alternatives considered**:
- Full XPath: More verbose, fragile to reordering. Rejected.
- `getComputedStyle`-based matching: Cannot generate a selector, only verify one. Not applicable.
- Inlining `finder` library (~3 KB minified): Would work, but adds a dependency and a copy-paste maintenance burden. Rejected for now; revisit if quality proves insufficient.

**Learned**: Angular-generated class names (e.g., `ng-tns-c1370865089-0`) change per build. The selector generator must skip classes that match `/ng-[a-z]+-[0-9]+-[0-9]+/` or similar generated patterns, falling back to structural selectors.

---

## R-04: Visual Picker Overlay — Isolation and Teardown

**Decision**: Inject the highlight overlay as a `<div id="aibbe-picker-overlay">` with `pointer-events: none` plus a transparent `<div id="aibbe-picker-shield">` with `pointer-events: all` capturing events. Tear down on click, `Escape` key, or `visual-picker-cancel` command.

**Rationale**:
- Using `pointer-events: none` on the visual highlight div prevents it from interfering with the element detection (events still bubble from the real target).
- A separate shield div intercepts click events without modifying the page DOM.
- Injecting with a known `id` makes teardown deterministic: `document.getElementById("aibbe-picker-shield")?.remove()`.

**Alternatives considered**:
- CSS `outline` on hovered element: Changes page layout (outline can shift elements). Rejected.
- Shadow DOM for overlay: Adds complexity with no benefit for this use case. Rejected.

**Learned**: The overlay must be removed before sending the CLI response, otherwise Chrome may close the message channel before the content script finishes cleanup. Teardown must be synchronous within the click handler.

---

## R-05: THINKING_MARKERS vs RESPONSE_READY_MARKERS — Confirmed Behavior

**Decision**: Do not add `THINKING_MARKERS` to the default calibration key set as a recommended target. `RESPONSE_READY_MARKERS` is the primary signal for response-completion detection and is already used correctly in `waitForAIResponse()`.

**Rationale** (from live codebase, `content.js` lines 291–297):
```javascript
if (!state.hasThinkingMarkers && state.hasReadyMarkers && state.result.trim() && state.result === pendingSnapshot) {
  // → response is complete
}
```
The logic already correctly gates on `!hasThinkingMarkers && hasReadyMarkers`. `THINKING_MARKERS` is used only as a negative signal ("not still thinking"). Making it a calibration target would be useless — an operator would never need to override a selector used only as a negative guard.

**Learned**: `RESPONSE_READY_MARKERS` (`.message-actions, .xap-copy-to-clipboard, [aria-label*="copy"]`) is the actionable selector. `THINKING_MARKERS` should be documented as read-only / not recommended for calibration in the operator quickstart.
