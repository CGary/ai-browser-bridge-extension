# Implementation Plan: Dockerized aibbe Stack with Isolated Chrome Account

**Mission**: dockerized-aibbe-stack-isolated-chrome-01KPCPFB  
**Mission ID**: 01KPCPFBPV1Y8QBSW7GP0PQ5XM  
**Branch**: `main` → merge target: `main`  
**Date**: 2026-04-17  
**Spec**: [spec.md](spec.md)

---

## Summary

Run the full aibbe stack (daemon + extension) inside a `linuxserver/chrome` Docker container using a fresh Google account. The daemon binary is volume-mounted from the host (read-only). Chrome launches it via Native Messaging using a Docker-specific manifest. The daemon creates a Unix socket inside the container into a bind-mounted directory, making it accessible to the host CLI through `AIBBE_SOCKET_PATH`. No source code changes are required — all work is configuration and documentation.

---

## Technical Context

**Language/Version**: Go 1.21 (daemon, pre-compiled on host); JavaScript MV3 (extension, no changes)  
**Primary Dependencies**: `linuxserver/chrome` Docker image; Docker Compose v2  
**Storage**: Named Docker volume for Chrome profile persistence  
**Testing**: Manual end-to-end verification (CLI round-trip); no new Go tests (no source changes)  
**Target Platform**: Debian 12 host + `linuxserver/chrome` container (Linux amd64)  
**Performance Goals**: CLI round-trip under 2 seconds (unchanged from charter)  
**Constraints**: Socket permissions 0600, PUID must equal host user UID, operational cost $0

---

## Charter Check

| Constraint | Status | Notes |
|---|---|---|
| Socket permissions 0600 | PASS | Daemon creates socket with 0600; PUID alignment grants host CLI access without relaxing permissions |
| Fail-Fast — no retries | PASS | No changes to daemon or CLI behaviour |
| Operational cost $0 | PASS | `linuxserver/chrome` is free; no paid services added |
| No backwards-compat shims | PASS | Docker config is purely additive; existing host setup unchanged |
| Conventional commits | REQUIRED | All commits in this feature must follow conventional commit format |
| go vet clean | N/A | No Go source changes |

---

## Project Structure

### Documentation (this feature)

```
kitty-specs/dockerized-aibbe-stack-isolated-chrome-01KPCPFB/
├── spec.md
├── plan.md              ← this file
├── research.md          ← Phase 0 complete
├── quickstart.md        ← Phase 1 output
└── tasks/
```

### Source Code (new files in repo root)

```
configs/
├── aibbe.nm-host.json            ← existing (host setup, unchanged)
├── aibbe.nm-host.docker.json     ← NEW: Docker-specific native host manifest
└── docker/
    └── docker-compose.yml        ← NEW: Container configuration

docs/
└── quickstart-docker.md          ← NEW: Setup guide for containerised stack
```

---

## Phase 0: Research (Complete)

See `research.md`. All unknowns resolved:

| Question | Decision |
|---|---|
| linuxserver/chrome filesystem layout | Chrome profile at `/config/.config/chromium/`; NativeMessagingHosts at `/config/.config/chromium/NativeMessagingHosts/` |
| Daemon delivery strategy | Volume-mount host binary at `/app/aibbe-daemon` (read-only) |
| Socket cross-container strategy | Bind-mount dedicated host dir `/tmp/aibbe-docker-socket/` → container `/run/aibbe/`; set `AIBBE_SOCKET_PATH=/run/aibbe/aibbe.sock` in container |
| UID alignment | Set `PUID`=host UID, `PGID`=host GID in docker-compose.yml |
| Extension loading | Volume-mount `./extension/` → `/config/extensions/aibbe/`; load via Chrome "Load unpacked" (one-time) |
| Source code changes | None required |

---

## Phase 1: Design & Artifacts

### Work Area 1 — Docker Native Host Manifest

**File**: `configs/aibbe.nm-host.docker.json`

Variant of the existing `configs/aibbe.nm-host.json`. Only difference: `path` field set to `/app/aibbe-daemon` (container-internal binary location). All other fields (`name`, `description`, `type`, `allowed_origins`) identical to the host manifest.

This file is volume-mounted into the container at:
`/config/.config/chromium/NativeMessagingHosts/aibbe.json`

**Acceptance criteria**:
- File is valid JSON
- `name` is `aibbe`
- `path` is `/app/aibbe-daemon`
- `type` is `stdio`
- `allowed_origins` matches the extension ID `bedlojjaiogmaefoadfpdecgajipcpgj`

---

### Work Area 2 — Docker Compose Configuration

**File**: `configs/docker/docker-compose.yml`

Defines a single `chrome` service:

| Parameter | Value |
|---|---|
| Image | `linuxserver/chrome:latest` |
| `PUID` | `<host-uid>` (documented placeholder — engineer sets `id -u`) |
| `PGID` | `<host-gid>` (documented placeholder — engineer sets `id -g`) |
| `AIBBE_SOCKET_PATH` | `/run/aibbe/aibbe.sock` |
| Port | `9500:3000` (KasmVNC web UI) |
| `shm_size` | `1gb` |
| `security_opt` | `seccomp:unconfined` |
| `restart` | `unless-stopped` |

**Volume mounts**:

| Host path | Container path | Mode |
|---|---|---|
| `<absolute-path-to-binary>` | `/app/aibbe-daemon` | `ro` |
| `../../extension` (relative to compose file) | `/config/extensions/aibbe` | `ro` |
| `./aibbe.nm-host.docker.json` (relative to compose file) | `/config/.config/chromium/NativeMessagingHosts/aibbe.json` | `ro` |
| `/tmp/aibbe-docker-socket` | `/run/aibbe` | `rw` |
| `aibbe-chrome-profile` (named volume) | `/config` | `rw` |

**Named volumes declaration**: `aibbe-chrome-profile` (Docker-managed persistence for Chrome profile).

**Acceptance criteria**:
- `docker compose -f configs/docker/docker-compose.yml up -d` starts without error
- Chrome accessible at `http://localhost:9500`
- `/tmp/aibbe-docker-socket/` directory exists on host after container start

---

### Work Area 3 — Setup Guide

**File**: `docs/quickstart-docker.md`

Step-by-step guide for the engineer:

1. **Prerequisites**: Docker installed, daemon binary compiled (`go build -o ./bin/aibbe-daemon ./daemon/`)
2. **Find host UID/GID**: `id -u` and `id -g`
3. **Edit docker-compose.yml**: Set `PUID`, `PGID`, and absolute binary path volume
4. **Create socket directory**: `mkdir -p /tmp/aibbe-docker-socket`
5. **Start container**: `docker compose -f configs/docker/docker-compose.yml up -d`
6. **Access Chrome**: Open `http://localhost:9500`
7. **Log in**: Sign into the isolated Google account
8. **Load extension**: `chrome://extensions` → Developer Mode → Load unpacked → `/config/extensions/aibbe`
9. **Verify extension**: Check background service worker console for no errors
10. **Test CLI**: `AIBBE_SOCKET_PATH=/tmp/aibbe-docker-socket/aibbe.sock go run cmd/cli/main.go -cmd ping`
11. **Stopping**: `docker compose -f configs/docker/docker-compose.yml down` (profile preserved in named volume)
12. **Updating daemon**: Recompile on host → container picks up new binary automatically (volume mount)

**Acceptance criteria**:
- Engineer completes setup and achieves a successful CLI round-trip in one session

---

## Risks & Mitigations

| Risk | Likelihood | Mitigation |
|---|---|---|
| UID mismatch breaks socket access | High if misconfigured | Setup guide covers `id -u`/`id -g` explicitly; docker-compose.yml uses labelled placeholders |
| Chrome sandbox incompatibility | Medium | `seccomp:unconfined` + `--no-sandbox` documented for linuxserver/chrome |
| Extension load fails (security policy) | Low | Developer Mode supported by linuxserver/chrome |
| Named volume removed accidentally | Low | Guide warns; volume name is explicit and distinct |
| Binary architecture mismatch | Low | Guide covers `GOOS=linux GOARCH=amd64 go build` cross-compile step |

---

## Branch Contract (Final)

- **Branch at plan start**: `main`
- **Planning base**: `main`
- **Merge target**: `main`

---

## Next Step

Run `/spec-kitty.tasks` to generate work packages from this plan.
