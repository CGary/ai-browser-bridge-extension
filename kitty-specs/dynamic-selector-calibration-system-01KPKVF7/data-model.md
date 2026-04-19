# Data Model: Dynamic Selector Calibration System

**Date**: 2026-04-19

---

## Entities

### SelectorKey

The canonical set of selector identifiers. These are the only valid keys in any calibration payload.

| Key | Default Value (content.js) | Recommended Calibration Target |
|-----|---------------------------|-------------------------------|
| `INPUT` | `body > ... > textarea` | ✅ Yes |
| `SUBMIT_BUTTON` | `body > ... > button` | ✅ Yes |
| `RESPONSE_CONTAINER` | `div.chat-panel-content > ... mat-card-content` | ✅ Yes |
| `RESPONSE_TEXT` | `div.message-text-content, ...` | ✅ Yes |
| `THINKING_MARKERS` | `.thinking-message, thinking-animation` | ⚠️ Not recommended (transient, negative signal only) |
| `RESPONSE_READY_MARKERS` | `.message-actions, .xap-copy-to-clipboard, [aria-label*="copy"]` | ✅ Yes — primary completion signal |
| `CITATION_NOISE` | `button.citation-marker, ...` | ✅ Yes |
| `CODE_BLOCK` | `code, pre` | ✅ Yes |

---

### CalibrationStore (chrome.storage.local)

**Storage key**: `aibbe_calibrations`

```typescript
type CalibrationStore = {
  [key: SelectorKey]: string;  // CSS selector string
}
```

**Example**:
```json
{
  "INPUT": "#new-input-id",
  "SUBMIT_BUTTON": ".btn-send-v2"
}
```

- Only keys that differ from defaults are stored.
- Absent keys fall back to code defaults.
- Maximum size: 50 KB (well within `chrome.storage.local` 10 MB quota).
- Cleared entirely by `reset-selectors`.

---

### ActiveSelectorMap (runtime, in-memory)

The resolved selector map used at runtime. Built by `loadSelectors()`.

```typescript
type SelectorSource = "calibration" | "default";

type ActiveSelectorEntry = {
  value: string;
  source: SelectorSource;
};

type ActiveSelectorMap = {
  [key: SelectorKey]: ActiveSelectorEntry;
};
```

**Example** (after calibrating `INPUT`):
```json
{
  "INPUT":                 { "value": "#new-input-id",                           "source": "calibration" },
  "SUBMIT_BUTTON":         { "value": "body > ... > button",                     "source": "default" },
  "RESPONSE_CONTAINER":    { "value": "div.chat-panel-content > ...",            "source": "default" },
  "RESPONSE_READY_MARKERS":{ "value": ".message-actions, .xap-copy-to-clipboard", "source": "default" }
}
```

---

### VisualPickerSession (ephemeral, in-memory)

Exists only while a Visual Picker is active on a tab. Torn down on click, Escape, or cancel command.

```typescript
type VisualPickerSession = {
  active: boolean;
  targetKey: SelectorKey;     // The selector key the operator is calibrating
  overlayElements: Element[]; // DOM nodes injected; removed on teardown
};
```

---

## State Transitions

### Selector Map Lifecycle

```
[content.js init]
      │
      ▼
loadSelectors()
  reads chrome.storage.local["aibbe_calibrations"]
  merges with CODE_DEFAULTS
      │
      ▼
activeSelectors = merged map
      │
      ├─── UPDATE_SELECTORS message received ──► loadSelectors() (re-merge)
      │
      ├─── "calibrate" command ──► persist overrides ──► broadcast UPDATE_SELECTORS
      │
      └─── "reset-selectors" ──► clear storage ──► broadcast UPDATE_SELECTORS
```

### Visual Picker Session Lifecycle

```
"visual-picker-start" cmd (target tab)
      │
      ▼
Inject overlay DOM
Attach mouseover/mouseout/click/keydown listeners
      │
      ├─── user hovers ──► compute selector ──► show tooltip
      │
      ├─── user clicks ──► capture selector ──► teardown overlay ──► respond {status:"success", selector}
      │
      └─── "visual-picker-cancel" OR Escape ──► teardown overlay ──► respond {status:"cancelled"}
```
