# Data Model: Target-based Library Routing

## Entities

### 1. IPC Request (Go)

**Location**: `internal/ipc/ipc.go`

```go
type Request struct {
    Cmd     string `json:"cmd"`
    Target  string `json:"target,omitempty"`  // NEW: library name; empty = fallback to first free tab
    Payload string `json:"payload"`
}
```

| Field | Type | Required | Constraints |
|-------|------|----------|-------------|
| `Cmd` | string | yes | non-empty |
| `Target` | string | no | trimmed library name; omitted from JSON when empty |
| `Payload` | string | no | max 1 MB total request |

**State transitions**: none — `Request` is immutable after creation.

---

### 2. TabRegistry Entry (JS)

**Location**: `extension/background.js` — `tabRegistry: Map<number, TabEntry>`

```js
// Before (current)
{ state: "free" | "busy", service: string, lastSeen: number }

// After (new)
{ state: "free" | "busy", service: string, lastSeen: number, target: string }
```

| Field | Type | Description |
|-------|------|-------------|
| `state` | `"free" \| "busy"` | Routing availability |
| `service` | string | Always `"notebooklm"` for this feature |
| `lastSeen` | number | `Date.now()` at last HANDSHAKE |
| `target` | string | Library name read from `div.cover-title` |

**State transitions**:
```
[unregistered] --HANDSHAKE--> free
free           --route--->    busy
busy           --done--->     free
free/busy      --tab closed-> [removed from map]
free/busy      --HANDSHAKE--> free (target updated, lastSeen refreshed)
```

---

### 3. HANDSHAKE Message (JS → JS, Content → Background)

**Transport**: `chrome.runtime.sendMessage`

```js
// Before (current)
{ type: "HANDSHAKE", service: "notebooklm" }

// After (new)
{ type: "HANDSHAKE", service: "notebooklm", target: "SIAT" }
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | `"HANDSHAKE"` | yes | Message discriminator |
| `service` | string | yes | Always `"notebooklm"` |
| `target` | string | yes | Library title from `div.cover-title` (non-empty) |

**Invariant**: HANDSHAKE is never sent until `target` is a non-empty string (enforced by MutationObserver wait).

---

### 4. Native Messaging Request (Background → Extension wire format)

**Transport**: Chrome Native Messaging (4-byte LE prefix + JSON)

```json
{ "cmd": "generate", "target": "SIAT", "payload": "¿qué es el SIAT?" }
```

`target` is optional. When absent, routing falls back to `findFreeTab()`.

---

### 5. Native Messaging Response (Extension → Background → CLI)

No changes to the success response shape. New error variant:

```json
{ "status": "error", "error": "target_not_found" }
```

Existing error variants (unchanged):
```json
{ "status": "error", "error": "no_free_tabs" }
{ "status": "error", "error": "input_not_found" }
{ "status": "error", "error": "submit_button_not_found" }
{ "status": "error", "error": "response_timeout" }
```

---

## Routing Decision Table

| `message.target` | Matching free tab exists | Result |
|------------------|--------------------------|--------|
| empty / absent | any free tab exists | Route to first free tab (`findFreeTab`) |
| empty / absent | no free tabs | `{ error: "no_free_tabs" }` |
| non-empty | matching free tab exists | Route to first matching free tab |
| non-empty | no matching free tab | `{ error: "target_not_found" }` |
| non-empty | matching tab exists but busy | `{ error: "target_not_found" }` |
