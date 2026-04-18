# Quickstart: Target-based Library Routing

## Prerequisites

- Daemon running: `go run daemon/main.go`
- Two NotebookLM tabs open in Chromium (e.g., "SIAT" and "PF" libraries)
- Extension installed with the updated `content.js` and `background.js`

## Usage

```bash
# Route to a specific library
go run cmd/cli/main.go -cmd "generate" -target "SIAT" -payload "¿qué es el SIAT?"

# Backward-compatible: no -target, routes to first free tab
go run cmd/cli/main.go -cmd "generate" -payload "general question"

# Error: library not open
go run cmd/cli/main.go -cmd "generate" -target "Nonexistent" -payload "..."
# → {"status":"error","error":"target_not_found"}
```

## How to Verify Target Registration

Open the Chromium DevTools for the extension background page (`chrome://extensions/` → Details → Inspect views: background page) and check:

```js
// In the background page console:
[...tabRegistry.entries()].map(([id, e]) => ({ id, target: e.target, state: e.state }))
// Expected: [{ id: 123, target: "SIAT", state: "free" }, { id: 456, target: "PF", state: "free" }]
```

## Testing Lane A (Go)

```bash
go test ./internal/ipc/ -v
go test ./cmd/cli/ -v
go vet ./...
```

## Testing Lane B (Extension)

1. Load extension in Chromium (Developer mode)
2. Open two NotebookLM tabs with different libraries
3. Wait 2–3 seconds for Angular to render (MutationObserver fires)
4. Check background page console for registration logs
5. Send targeted CLI command and verify response comes from the correct library
