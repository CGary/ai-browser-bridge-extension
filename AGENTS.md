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

## 🚫 SDD Phase Boundaries — NO ejecutar tests fuera de `sdd-verify`

**Regla:** `sdd-archive` SOLO recupera artefactos existentes desde Engram y genera el reporte de cierre. NO ejecuta tests, builds, lint, ni verificación de código.

- **Tests y verificación** → responsabilidad exclusiva de `sdd-verify`.
- **Apply-progress con evidencia TDD** → responsabilidad de `sdd-apply`.
- **Archivar** → leer artefactos previos, sintetizar estado final, persistir en Engram.

**Por qué**: Ejecutar tests en archive infla el contexto sin necesidad, duplica trabajo ya hecho por `sdd-verify`, y viola el contrato de la fase. Si el verify-report ya existe en Engram, confiar en él.

**Excepción**: Solo ejecutar tests en `sdd-archive` si el usuario lo pide explícitamente.

## 🔍 Proactive Context Retrieval (Engram)

**Mandatory Search Strategy:** Before attempting to search for a specific SDD phase (e.g., `sdd/t15/spec`), you MUST perform a broad search using ONLY the change identifier (e.g., `t15-observador-mutacion-extraccion`).

- **Why**: Broad searches capture the entire lifecycle of a change (exploration, proposal, specs, design, tasks, and progress) in a single turn, preventing "not found" errors caused by overly specific phase queries or naming mismatches.
- **When**: Trigger this broad search at the beginning of ANY SDD phase (`/sdd-explore`, `/sdd-propose`, `/sdd-spec`, `/sdd-design`, `/sdd-tasks`, `/sdd-apply`, `/sdd-verify`).
- **How**: Use `mcp_engram_mem_search(project='aibbe', query='[CHANGE-NAME]')`.

## 🔍 Memory Retrieval Gap Protocol (Engram)

**Mandatory Compliance:** This protocol MUST be triggered whenever you query Engram (via `search_entries`, `get_entry_by_id`, etc.) and the result is empty, irrelevant, or requires the user to manually provide an ID or information.

### 📂 Log Location
Record these gaps in: `.atl/observability/memory-faults/fault-log-[YYYY-MM-DD].md`
*Note: Create the directory if it does not exist.*

### 📝 Mandatory Retrieval Gap Format
Every time you fail to find information and the user has to intervene, log it as follows:

---
#### ⚠️ Engram Data Gap
- **Agent:** [e.g., Claude Code, Cursor, OpenCode]
- **Invoked Tool:** [e.g., search_entries, get_entry_by_id, list_namespaces]
- **Query Parameter:** `[The exact string you used to search]`
- **Namespace:** `[The namespace targeted]`
- **Operational Context:** [e.g., Running /sdd-explore for the authentication module]
- **Failure Reason:** [e.g., No results found, High vector distance/low relevance, Missing entity ID]
- **Manual Resolution:** [The exact ID or information provided by the user to fix the gap]
- **Correction Status:** [PENDING | REPAIRED via engram_push]
---

### 🚀 Resolution Flow
1. **Log the Gap:** Immediately write the entry above once the user provides the missing data.
2. **Sync Memory:** Use `engram_push` or `upsert` to store the manually provided data so this gap never occurs again.
3. **Update Status:** Change 'Self-Correction' from PENDING to **REPAIRED**.