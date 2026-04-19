# Internal Message Contracts: Background Service Worker ↔ Content Script

These messages are sent via `chrome.tabs.sendMessage(tabId, message)` from `background.js` to `content.js`.

---

## UPDATE_SELECTORS

Broadcast from background to **all** registered tabs when calibrations change.

**Sender**: `background.js`
**Receiver**: `content.js` (all registered tabs)

```json
{
  "type": "UPDATE_SELECTORS",
  "calibrations": {
    "INPUT": "#new-input-id"
  }
}
```

`calibrations` is the full contents of `aibbe_calibrations` from `chrome.storage.local`. Content script calls `loadSelectors()` upon receipt.

**Content script response**: none (fire-and-forget).

---

## ACTIVATE_VISUAL_PICKER

Sent to a single targeted tab to enter highlight mode.

**Sender**: `background.js`
**Receiver**: `content.js` (one tab)

```json
{
  "type": "ACTIVATE_VISUAL_PICKER",
  "key": "INPUT"
}
```

`key` is displayed in the overlay tooltip to guide the operator.

**Content script response** (via `sendResponse`):
```json
{ "status": "success", "key": "INPUT", "selector": ".captured-class" }
```
or:
```json
{ "status": "cancelled", "key": "INPUT" }
```

The response is forwarded by `background.js` back to the native host as the command response.

---

## DEACTIVATE_VISUAL_PICKER

Sent to cancel an active Visual Picker session.

**Sender**: `background.js`
**Receiver**: `content.js` (one tab)

```json
{ "type": "DEACTIVATE_VISUAL_PICKER" }
```

**Content script response**:
```json
{ "status": "success" }
```

Returns `{ "status": "success" }` even if no picker was active.
