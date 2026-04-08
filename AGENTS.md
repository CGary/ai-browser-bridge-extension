This file provides guidance when working with code in this repository.

## Engram Project Name

**Project Name**: `aibbe`

Todos los agentes deben usar `project: aibbe` al guardar/encontrar memorias en Engram. No usar el nombre del directorio `ai-browser-bridge-extension`.

## Commands

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./daemon/
go test ./cmd/cli/
go test ./internal/nativemessaging/
go test ./internal/ipc/

# Run a single test by name
go test ./daemon/ -run TestCleanupSocket_FileExists

# Static analysis
go vet ./...

# Build (use explicit output to avoid naming collision with daemon/ directory)
go build -o /tmp/aibbe-daemon ./daemon/
go build -o /tmp/aibbe-cli ./cmd/cli/

# Run daemon
go run daemon/main.go

# Send a command via CLI (daemon must be running)
go run cmd/cli/main.go -cmd "mycommand" -payload "some data"
```

## Architecture

Three-layer messaging bridge: CLI → Daemon → Chrome Extension (via Native Messaging).

```
CLI (ephemeral)  ──JSON/Unix socket──►  Daemon (resident)  ──4-byte LE + JSON──►  Chrome Extension
cmd/cli/main.go                          daemon/main.go                              extension/background.js
```

**CLI** (`cmd/cli/`): Ephemeral process. Parses `-cmd` (required) and `-payload` flags, sends `ipc.Request` over Unix socket, blocks for response, exits 0/1.

**Daemon** (`daemon/`): Resident process. Listens on Unix socket (default `/tmp/aibbe.sock`, configurable via `AIBBE_SOCKET_PATH`). Handles one IPC request at a time synchronously. Forwards payloads to the Chrome Extension via Native Messaging stdin/stdout. Returns extension response to CLI via channel.

**Native Messaging** (`internal/nativemessaging/`): Wire format — 4-byte little-endian uint32 length prefix followed by JSON payload. Max 1 MB per Chrome protocol limit.

**IPC** (`internal/ipc/`): `Request{Cmd, Payload}` struct. Max 1 MB. Socket path from `AIBBE_SOCKET_PATH` env or `/tmp/aibbe.sock`.

**Chrome Extension** (`extension/`): Manifest V3, static ID `bedlojjaiogmaefoadfpdecgajipcpgj`. Service Worker (`background.js`) connects to native host `aibbe`, currently echoes messages back.

**Native Host Manifest** (`configs/aibbe.nm-host.json`): Must be installed manually to `~/.config/chromium/NativeMessagingHosts/aibbe.json` with the compiled daemon binary path updated.

## Key Design Decisions

- **Fail-Fast**: No retries. Any error (protocol desync, selector mismatch, size violation) aborts with exit code 1.
- **Volatile Storage Only**: No persistence to disk or `chrome.storage.*`. All data lives in RAM during a transaction.
- **Socket Permissions 0600**: Set via umask `0o177` during socket creation. Restricts access to owner only.
- **Two-Layer Size Validation**: IPC layer (1 MB) is primary; Native Messaging layer (1 MB) is defensive secondary.
- **Synchronous CLI Semantics**: CLI blocks on daemon response. One request in flight at a time.

## Test Patterns

Tests use table-driven style throughout. Key helpers:

- `tempSocketPath()` — uses `t.TempDir()` for socket isolation
- `startMockDaemon()` — goroutine Unix socket listener for CLI tests
- `buildCLIBinary()` — compiles test binary via `go build`
- `requireUnixSocketSupport()` — skip on non-Unix platforms
- `ioReadAllWithDeadline()` — 2-second deadline to prevent hanging reads

## Development Status

Milestones 1–2 complete (CLI, Daemon, IPC, Native Messaging, security hardening, tasks t1–t10). Milestone 3 (Tab Orchestrator — tab registry, exclusive routing, DOM injection) is in progress. Pending tasks: `t11-handshake-registro-tabs`, `t12-gestion-ciclo-vida-tabs`, `t13-orquestacion-enrutamiento-transaccional`.
