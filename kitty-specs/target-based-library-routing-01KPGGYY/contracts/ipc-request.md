# Contract: IPC Request (CLI → Daemon → Extension)

## Wire Format

JSON, max 1 MB. Transmitted over Unix socket (`/tmp/aibbe.sock` or `$AIBBE_SOCKET_PATH`).

```json
{
  "cmd": "<string, required, non-empty>",
  "target": "<string, optional — omitted when empty>",
  "payload": "<string, optional>"
}
```

## Examples

```json
// Targeted query
{ "cmd": "generate", "target": "SIAT", "payload": "¿qué es el SIAT?" }

// Untargeted query (backward-compatible, no target key)
{ "cmd": "generate", "payload": "general question" }

// Echo (no target needed)
{ "cmd": "echo", "payload": "ping" }
```

## Validation Rules

| Rule | Owner | Behavior on violation |
|------|-------|-----------------------|
| `cmd` must be non-empty | CLI | `exitWithError("-cmd flag is required")` |
| Total JSON ≤ 1 MB | CLI (ipc.MaxRequestSize) | encode fails, exit 1 |
| `target` may be empty string or absent | CLI | serialized as omitted (`omitempty`) |

## CLI Flags

```
-cmd     string   command identifier (required)
-target  string   library name to route to (optional)
-payload string   associated data (optional)
```
