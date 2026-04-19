# IPC Command Contracts: CLI ↔ Daemon ↔ Chrome Extension

**Note**: The daemon passes all commands through unchanged. These are the command shapes the Chrome extension receives and the responses it must return.

---

## calibrate

Update one or more selector overrides. Partial updates are allowed — only specified keys are overridden.

**Request** (CLI → Daemon → Extension)
```json
{
  "cmd": "calibrate",
  "payload": "{\"INPUT\": \"#new-input-id\", \"SUBMIT_BUTTON\": \".btn-send-v2\"}"
}
```
`payload` is a JSON-encoded object. Keys must be valid SelectorKey values. Unknown keys are ignored.

**Response** (Extension → Daemon → CLI)
```json
{ "status": "success", "applied": ["INPUT", "SUBMIT_BUTTON"] }
```
On error:
```json
{ "status": "error", "error": "invalid_payload" }
```

---

## reset-selectors

Clear all stored calibrations and revert to factory defaults immediately.

**Request**
```json
{ "cmd": "reset-selectors", "payload": "" }
```

**Response**
```json
{ "status": "success" }
```

---

## get-active-selectors

Return the full selector map with per-key source annotation.

**Request**
```json
{ "cmd": "get-active-selectors", "payload": "" }
```

**Response**
```json
{
  "status": "success",
  "selectors": {
    "INPUT":                  { "value": "#new-input-id",                             "source": "calibration" },
    "SUBMIT_BUTTON":          { "value": "body > ... > button",                       "source": "default" },
    "RESPONSE_CONTAINER":     { "value": "div.chat-panel-content > ...",              "source": "default" },
    "RESPONSE_TEXT":          { "value": "div.message-text-content, ...",             "source": "default" },
    "THINKING_MARKERS":       { "value": ".thinking-message, thinking-animation",     "source": "default" },
    "RESPONSE_READY_MARKERS": { "value": ".message-actions, .xap-copy-to-clipboard", "source": "default" },
    "CITATION_NOISE":         { "value": "button.citation-marker, ...",               "source": "default" },
    "CODE_BLOCK":             { "value": "code, pre",                                 "source": "default" }
  }
}
```

---

## visual-picker-start

Activate highlight mode on the tab matching `target`. The command blocks until the operator clicks an element or cancels.

**Request**
```json
{
  "cmd": "visual-picker-start",
  "target": "My Notebook Title",
  "payload": "{\"key\": \"INPUT\"}"
}
```
`target` must match a tab registered in `tabRegistry`. `key` is the SelectorKey the operator intends to calibrate (used for labeling in the overlay tooltip).

**Response — success (element selected)**
```json
{ "status": "success", "key": "INPUT", "selector": ".new-input-class" }
```

**Response — cancelled**
```json
{ "status": "cancelled", "key": "INPUT" }
```

**Response — error**
```json
{ "status": "error", "error": "target_not_found" }
```

---

## visual-picker-cancel

Cancel an active Visual Picker session on the specified tab without selecting any element.

**Request**
```json
{
  "cmd": "visual-picker-cancel",
  "target": "My Notebook Title",
  "payload": ""
}
```

**Response**
```json
{ "status": "success" }
```
Returns `{ "status": "success" }` even if no picker was active (idempotent).
