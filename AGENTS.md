This file provides guidance when working with code in this repository.

## Engram Project Name

**Project Name**: `aibbe`

Todos los agentes deben usar `project: aibbe` al guardar/encontrar memorias en Engram. No usar el nombre del directorio `ai-browser-bridge-extension`.

## Commands

```bash
# Run all tests (includes root-level extension e2e tests that validate extension/*.js)
go test ./...

# Run tests for a specific package
go test ./daemon/
go test ./cmd/cli/
go test ./internal/nativemessaging/
go test ./internal/ipc/
go test ./configs/
go test .                     # root: extension handshake/routing tests

# Run a single test by name
go test ./daemon/ -run TestCleanupSocket_FileExists
go test . -run TestRouting_SuccessfulRoutingToFreeTab

# Static analysis
go vet ./...

# Syntax check the extension JS
node --check extension/background.js
node --check extension/content.js

# Build (use explicit output to avoid naming collision with daemon/ directory)
go build -o /tmp/aibbe-daemon ./daemon/
go build -o /tmp/aibbe-cli ./cmd/cli/

# Run daemon
go run daemon/main.go

# Send a command via CLI (daemon must be running)
go run cmd/cli/main.go -cmd generate -payload "some prompt"
go run cmd/cli/main.go -cmd generate -target "Notebook title" -payload "some prompt"
```

## Architecture

Automation bridge between a Go CLI/daemon and **NotebookLM** running in a Chromium browser. The extension is coupled to NotebookLM (content script only matches `https://notebooklm.google.com/*`); CLI, daemon, and protocol are service-agnostic.

```
CLI (ephemeral)  ──JSON/Unix socket──►  Daemon (resident)  ──4-byte LE + JSON──►  Extension SW  ──tabs.sendMessage──►  content.js (NotebookLM tab)
cmd/cli/main.go                          daemon/main.go                            background.js                        extension/content.js
```

**CLI** (`cmd/cli/`): Ephemeral process. Parses `-cmd` (required), `-target`, and `-payload` flags, sends `ipc.Request{Cmd, Target, Payload}` over Unix socket, blocks for response, exits 0/1.

**Daemon** (`daemon/`): Resident process. Listens on Unix socket (default `/tmp/aibbe.sock`, configurable via `AIBBE_SOCKET_PATH`). Handles one IPC request at a time synchronously. Forwards payloads to the extension via Native Messaging stdin/stdout and returns the extension response to the CLI.

**Native Messaging** (`internal/nativemessaging/`): Wire format — 4-byte little-endian uint32 length prefix followed by JSON payload. Max 1 MB per Chrome protocol limit.

**IPC** (`internal/ipc/`): `Request{Cmd, Target, Payload}` struct. Max 1 MB. Socket path from `AIBBE_SOCKET_PATH` env or `/tmp/aibbe.sock`.

**Extension** (`extension/`): Manifest V3, static ID `bedlojjaiogmaefoadfpdecgajipcpgj`, native host name `aibbe`.
- `background.js` (Service Worker): connects to the native host and routes commands to tabs via a `tabRegistry`.
- `content.js`: injected into NotebookLM. Executes commands against the DOM using a selector cascade (calibrated overrides from `chrome.storage.local` → code defaults).

**Tab routing (HANDSHAKE)**: each NotebookLM tab sends a `HANDSHAKE` to the background on content-script load — first with `target=null`, then updated with the notebook title once it renders. Without `-target`, requests route to the first free tab; with `-target`, to the tab whose notebook title matches exactly. Errors: `no_free_tabs`, `busy`, `target_not_found`.

**Supported commands** (`-cmd`): `generate` (inject prompt, submit, wait for full response), `probe-selectors` (diagnostic match counts), `get-active-selectors`, `calibrate` (runtime selector override, payload `{"KEY": "css"}`), `reset-selectors`. Valid calibrate keys: `INPUT`, `SUBMIT_BUTTON`, `RESPONSE_CONTAINER`, `RESPONSE_TEXT`, `THINKING_MARKERS`, `RESPONSE_READY_MARKERS`, `CITATION_NOISE`.

**Native Host Manifests** (`configs/`): `aibbe.nm-host.json` (local host — install to `~/.config/chromium/NativeMessagingHosts/aibbe.json` with the daemon binary path updated) and `aibbe.nm-host.docker.json` (containerized Chromium). `configs/docker/docker-compose.yml` runs the Chromium+daemon stack; see `docs/quickstart-docker.md`.

## Key Design Decisions

- **Fail-Fast**: No retries. Any error (protocol desync, selector mismatch, size violation) aborts with exit code 1.
- **Volatile storage for automation data**: no persistence to disk. Only selector calibrations persist, in `chrome.storage.local` (by design).
- **Socket Permissions 0600**: Set via umask `0o177` during socket creation. Restricts access to owner only.
- **Two-Layer Size Validation**: IPC layer (1 MB) is primary; Native Messaging layer (1 MB) is defensive secondary.
- **Synchronous CLI Semantics**: CLI blocks on daemon response. One request in flight at a time.
- **Locale-agnostic default selectors**: prefer NotebookLM semantic CSS classes (`query-box-input`, `submit-button`, `to-user-message-inner-content`, `message-actions`) over `aria-label`, so it works in `es`, `nl`, `en`, etc. Ignore generated classes (`ng-*`, `mat-mdc-*`, `cdk-*`) when calibrating.

## Test Patterns

Tests use table-driven style throughout. Key helpers:

- `tempSocketPath()` — uses `t.TempDir()` for socket isolation
- `startMockDaemon()` — goroutine Unix socket listener for CLI tests
- `buildCLIBinary()` — compiles test binary via `go build`
- `requireUnixSocketSupport()` — skip on non-Unix platforms
- `ioReadAllWithDeadline()` — 2-second deadline to prevent hanging reads

Root-level tests (`extension_handshake_test.go`, `extension_routing_test.go`) validate the extension JS from Go by inspecting/executing `extension/*.js` — no browser required.

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
