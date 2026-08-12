# AI Browser Bridge Extension (aibbe)

> Language / Idioma: **English** | [Español](es-README.md)

## Purpose

Automation bridge between a Go CLI/daemon and **NotebookLM** (Google) running in a Chromium browser. It allows an external agent to send prompts, read responses, and adjust DOM selectors on the fly, without reloading the extension.

The extension is coupled to NotebookLM (the content script only matches `https://notebooklm.google.com/*` and `https://notebook.google.com/*` — Google migrated NotebookLM to the latter domain). The rest of the stack (CLI, daemon, protocol) is service-agnostic.

## Architecture

```
┌─────────┐    Unix Socket    ┌────────────┐    Native Messaging    ┌──────────────────────┐
│  CLI    │ ──────────────► │  Daemon    │ ────────────────────► │  Chrome Extension    │
│ cmd/cli │   ipc.Request    │  daemon/   │   4-byte LE + JSON    │  background.js (SW)  │
│         │ ◄────────────── │            │ ◄──────────────────── │  content.js          │
└─────────┘   ipc.Response   └────────────┘                       │  → notebooklm.google │
                                                                    └──────────────────────┘
```

| Layer | Format | Limit |
|---|---|---|
| CLI ↔ Daemon | JSON over Unix socket (`ipc.Request{Cmd, Target, Payload}`) | 1 MB |
| Daemon ↔ Extension | Native Messaging: uint32 LE length prefix + JSON | 1 MB |
| Extension → Tab | `chrome.tabs.sendMessage` to the content script of the target tab | — |

One request at a time, synchronous. `fail-fast`: any error aborts with exit code 1 (no retries).

## Components

| Path | Description |
|---|---|
| `cmd/cli/main.go` | Ephemeral CLI. Accepts `-cmd`, `-target`, `-payload` |
| `daemon/main.go` | Resident daemon. Unix socket → Native Messaging |
| `extension/manifest.json` | Manifest V3, static ID, permissions `nativeMessaging`/`storage`/`tabs` |
| `extension/background.js` | Service Worker. Connects to native host, routes commands to tabs |
| `extension/content.js` | Injected into NotebookLM. DOM selectors + runtime calibration |
| `internal/ipc/` | `Request`/`Response`, socket path resolution |
| `internal/nativemessaging/` | Codec 4-byte LE + JSON |
| `configs/aibbe.nm-host.json` | Native Messaging Manifest (Linux host) |
| `configs/aibbe.nm-host.docker.json` | Native Messaging Manifest for container |
| `configs/docker/docker-compose.yml` | Chromium + daemon stack in Docker |

## Tab Routing (HANDSHAKE)

Each NotebookLM tab sends a `HANDSHAKE` to the background on content script load with the notebook title as `target`. The background maintains a `tabRegistry` of available tabs.

- Without `-target` flag: the daemon routes to the first available free tab.
- With `-target "notebook title"`: routes to the tab whose title matches exactly.

To allow the CLI to route even before the title is rendered, the content script sends an initial handshake with `target=null` and updates it once the title renders. The title is extracted from `div.cover-title` if present; if that class changes ("Gemini Notebook" UI), it falls back to page `<title>` by stripping the brand suffix (`"Siat - Gemini Notebook"` → `"Siat"`).

## Available Commands

| Cmd | Payload | Response | What it does |
|---|---|---|---|
| `generate` | prompt text | `{status, result}` | Injects prompt, submits, waits for complete response |
| `probe-selectors` | — | `{status, report}` | Reports how many elements match each selector (diagnostics) |
| `get-active-selectors` | — | `{status, selectors}` | Returns active selectors indicating if default or calibrated |
| `calibrate` | JSON `{KEY: "selector", ...}` | `{status, applied}` | Overrides selectors in `chrome.storage.local` and broadcasts to all tabs |
| `reset-selectors` | — | `{status}` | Clears all calibrations, reverts to code defaults |

Valid keys for `calibrate`: `INPUT`, `SUBMIT_BUTTON`, `RESPONSE_CONTAINER`, `RESPONSE_TEXT`, `THINKING_MARKERS`, `RESPONSE_READY_MARKERS`, `CITATION_NOISE`.

## Quickstart — Docker (Recommended)

Full setup with isolated Chromium in container: see [`docs/quickstart-docker.md`](docs/quickstart-docker.md). Summary:

```bash
# 1. Build binaries for linux/amd64 (compose mounts them from bin/)
GOOS=linux GOARCH=amd64 go build -o bin/aibbe-daemon ./daemon/
GOOS=linux GOARCH=amd64 go build -o bin/aibbe-cli ./cmd/cli/

# 2. VPN credentials (ProtonVPN OpenVPN; vpn.env is git-ignored)
cp configs/docker/vpn.env.example configs/docker/vpn.env  # and fill it out

# 3. Start the stack (services: vpn + chrome)
docker compose -f configs/docker/docker-compose.yml up -d

# 4. Load extension in Chrome (http://localhost:9500) from /config/extensions/aibbe

# 5. Use CLI inside the container (socket lives inside container)
docker exec chrome aibbe-cli -cmd generate -payload "hello"
```

## Quickstart — Local Host (Without Docker)

```bash
# Build
go build -o /tmp/aibbe-daemon ./daemon/
go build -o /tmp/aibbe-cli ./cmd/cli/

# Register native host (Chromium)
mkdir -p ~/.config/chromium/NativeMessagingHosts/
cp configs/aibbe.nm-host.json ~/.config/chromium/NativeMessagingHosts/aibbe.json
# edit the "path" in manifest to point to binary

# Load extension
#   chrome://extensions → Developer mode → Load unpacked → extension/

# Start daemon
/tmp/aibbe-daemon

# Use CLI
/tmp/aibbe-cli -cmd generate -payload "what is eslint"
```

## Calibration Workflow when NotebookLM changes DOM

```bash
# 1. Diagnose
aibbe-cli -cmd probe-selectors
#   → identifies which selectors are marked as "missing" or "multiple"

# 2. Inspect DOM in DevTools, find a stable semantic class
#    (ignore ng-*, mat-mdc-*, cdk-*, _nghost-*, ng-tns-*)

# 3. Runtime override (without reloading anything)
aibbe-cli -cmd calibrate -payload '{"SUBMIT_BUTTON": "button.new-class"}'

# 4. Validate
aibbe-cli -cmd probe-selectors
aibbe-cli -cmd generate -payload "test"

# 5. If working well, update default selectors in extension/content.js and reset:
aibbe-cli -cmd reset-selectors
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `AIBBE_SOCKET_PATH` | `/tmp/aibbe.sock` | Unix socket path (CLI and daemon must match) |

## Chromium Extension

- **Version**: 0.1.0
- **Static ID**: `bedlojjaiogmaefoadfpdecgajipcpgj` (pinned via `key` in manifest)
- **Permissions**: `nativeMessaging`, `storage`, `tabs`
- **Host matches**: `https://notebooklm.google.com/*`, `https://notebook.google.com/*`
- **Native host name**: `aibbe`

## Design Decisions

- **Fail-fast**: no retries or fallbacks. Any protocol error aborts with exit code 1.
- **Volatile storage for automation data**: no persistence to disk. Only calibrations live in `chrome.storage.local` (persistent by design).
- **Socket permissions 0600**: created with `umask 0o177`, owner access only.
- **Two-layer size validation**: IPC layer (1 MB primary) + Native Messaging layer (1 MB defensive).
- **Locale-agnostic default selectors**: prefer NotebookLM semantic CSS classes (`query-box-input`, `submit-button`, `to-user-message-inner-content`, `message-actions`) over `aria-label`, so it works across `es`, `nl`, `en`, etc.

## Troubleshooting

| Symptom | Probable Cause | Fix |
|---|---|---|
| `generate` returns `response_timeout` | A selector (likely `RESPONSE_READY_MARKERS` or `RESPONSE_CONTAINER`) does not match | `probe-selectors` + `calibrate` |
| `generate` returns partial text | `RESPONSE_TEXT` points to a node that is too narrow | Inspect and recalibrate |
| `no_free_tabs` | No NotebookLM tab registered via handshake | Open/refresh tab; check content script console |
| `target_not_found` | `-target` does not match title of any notebook | Omit `-target` or use exact name |
| `native messaging host has not registered` | Manifest misplaced or binary path incorrect | Verify `~/.config/chromium/NativeMessagingHosts/aibbe.json` |
| Socket connection refused | Daemon not running | Start daemon |
| Permission denied on socket | Incorrect owner (common in Docker due to UID/GID mismatch) | See `docs/quickstart-docker.md` |

## Development

```bash
# Tests
go test ./...
go test ./daemon/ -run TestCleanupSocket_FileExists

# Static analysis
go vet ./...

# Extension syntax check
node --check extension/content.js
node --check extension/background.js
```

## Additional Documentation

- [`docs/quickstart-docker.md`](docs/quickstart-docker.md) — step-by-step Docker setup
- [`docs/Software Design Document.md`](docs/Software%20Design%20Document.md) — architectural decisions
- [`docs/propuesta-calibracion-dinamica.md`](docs/propuesta-calibracion-dinamica.md) — calibration system design
- [`CLAUDE.md`](CLAUDE.md) — guide for agents working in this repository
