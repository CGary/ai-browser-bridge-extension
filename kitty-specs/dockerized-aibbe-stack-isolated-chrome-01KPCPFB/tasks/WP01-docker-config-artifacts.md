---
work_package_id: WP01
title: Docker Configuration Artifacts
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-005
planning_base_branch: main
merge_target_branch: main
branch_strategy: Feature branch cut from main; merge back to main when WP is approved.
subtasks:
- T001
- T002
- T003
history:
- date: '2026-04-17T03:33:10Z'
  event: created
authoritative_surface: configs/
execution_mode: code_change
owned_files:
- configs/aibbe.nm-host.docker.json
- configs/docker/docker-compose.yml
- configs/docker/
tags: []
---

# WP01 — Docker Configuration Artifacts

## Objective

Create the two configuration files that define the containerized aibbe stack:

1. `configs/aibbe.nm-host.docker.json` — Docker-specific native host manifest (tells Chrome inside the container where the daemon binary lives)
2. `configs/docker/docker-compose.yml` — Compose service definition with all volumes, environment variables, and security options

No Go or JavaScript source changes. No tests to write. All decisions are already resolved in `plan.md`.

---

## Context

The full architecture is:

```
Host CLI  ──(bind-mounted Unix socket)──►  Containerized Daemon  ──(Native Messaging stdio)──►  Chrome Extension inside container
```

Key resolved decisions (from `plan.md` § Phase 0):

| Concern | Decision |
|---|---|
| Daemon delivery | Volume-mount host binary at `/app/aibbe-daemon` (read-only) |
| Socket path | Bind-mount `/tmp/aibbe-docker-socket/` (host) → `/run/aibbe/` (container); `AIBBE_SOCKET_PATH=/run/aibbe/aibbe.sock` |
| Chrome profile path | `/config/.config/chromium/` inside container (linuxserver/chrome layout) |
| Native host manifest location | `/config/.config/chromium/NativeMessagingHosts/aibbe.json` inside container |
| UID alignment | `PUID`=host UID, `PGID`=host GID (prevents socket permission failures) |
| Extension loading | Volume-mount `./extension/` → `/config/extensions/aibbe/` (loaded manually in Chrome) |

Reference the existing manifest for field values:
```
configs/aibbe.nm-host.json
```

Extension ID (from spec): `bedlojjaiogmaefoadfpdecgajipcpgj`

---

## Branch Strategy

- **Planning base branch**: `main`
- **Final merge target**: `main`
- Execution worktrees are allocated per computed lane from `lanes.json`.
- Run with: `spec-kitty agent action implement WP01 --agent <name>`

---

## Subtask Guidance

### Subtask T001 — Create `configs/aibbe.nm-host.docker.json`

**Purpose**: Provide Chrome (inside the container) with the path to the daemon binary. This file is volume-mounted into the container's NativeMessagingHosts directory.

**Steps**:

1. Create file at `configs/aibbe.nm-host.docker.json`.

2. Base it on `configs/aibbe.nm-host.json`. Change ONLY the `path` field:
   - Existing (host): `"/home/gary/dev/ai-browser-bridge-extension/daemon/aibbe"`
   - Docker variant: `"/app/aibbe-daemon"` (container-internal path)

3. All other fields stay identical:
   - `name`: `"aibbe"`
   - `description`: `"AI Browser Bridge Extension Host"`
   - `type`: `"stdio"`
   - `allowed_origins`: `["chrome-extension://bedlojjaiogmaefoadfpdecgajipcpgj/"]`

**Expected file content**:
```json
{
  "name": "aibbe",
  "description": "AI Browser Bridge Extension Host",
  "path": "/app/aibbe-daemon",
  "type": "stdio",
  "allowed_origins": [
    "chrome-extension://bedlojjaiogmaefoadfpdecgajipcpgj/"
  ]
}
```

**Files**:
- `configs/aibbe.nm-host.docker.json` (new file, ~9 lines)

**Validation**:
- [ ] `cat configs/aibbe.nm-host.docker.json | python3 -m json.tool` exits 0 (valid JSON)
- [ ] `path` field is exactly `/app/aibbe-daemon`
- [ ] `allowed_origins` contains `chrome-extension://bedlojjaiogmaefoadfpdecgajipcpgj/`
- [ ] `name` is `aibbe`, `type` is `stdio`

---

### Subtask T002 — Create `configs/docker/docker-compose.yml`

**Purpose**: Define the `chrome` service that runs the full containerized stack. This file is the single point of truth for the container's runtime configuration.

**Steps**:

1. Create directory `configs/docker/` and file `configs/docker/docker-compose.yml`.

2. Define a single `chrome` service with the following specification:

   **Image**: `linuxserver/chrome:latest`

   **Environment variables**:
   ```yaml
   environment:
     - PUID=1000    # PLACEHOLDER — engineer must replace with: id -u
     - PGID=1000    # PLACEHOLDER — engineer must replace with: id -g
     - AIBBE_SOCKET_PATH=/run/aibbe/aibbe.sock
   ```

   **Ports**:
   ```yaml
   ports:
     - "3000:3000"  # KasmVNC web UI for browser access
   ```

   **Shared memory** (Chrome requires large shm for stability):
   ```yaml
   shm_size: "1gb"
   ```

   **Security** (Chrome sandbox requires this inside Docker):
   ```yaml
   security_opt:
     - seccomp:unconfined
   ```

   **Restart policy**:
   ```yaml
   restart: unless-stopped
   ```

   **Volume mounts** (in this exact order for readability):
   ```yaml
   volumes:
     # Daemon binary (read-only) — engineer must set absolute path
     - /REPLACE/WITH/ABSOLUTE/PATH/TO/aibbe-daemon:/app/aibbe-daemon:ro
     # Chrome extension source (read-only, relative to compose file)
     - ../../extension:/config/extensions/aibbe:ro
     # Docker-specific native host manifest (read-only)
     - ./aibbe.nm-host.docker.json:/config/.config/chromium/NativeMessagingHosts/aibbe.json:ro
     # Socket directory — shared with host CLI
     - /tmp/aibbe-docker-socket:/run/aibbe:rw
     # Chrome profile persistence (named volume)
     - aibbe-chrome-profile:/config:rw
   ```

   **Named volume declaration** (at top level, outside `services`):
   ```yaml
   volumes:
     aibbe-chrome-profile:
   ```

3. Add a header comment block at the top of the file explaining the two placeholders the engineer must fill in before running:
   - `PUID` / `PGID`: output of `id -u` / `id -g`
   - Binary volume path: absolute path to the compiled `aibbe-daemon` binary

**Complete expected structure**:
```yaml
# Docker Compose configuration for the containerized aibbe stack.
#
# BEFORE RUNNING:
#   1. Replace PUID and PGID with your host UID/GID:
#        PUID=$(id -u)   PGID=$(id -g)
#   2. Replace the daemon binary volume source with the absolute path to your
#      compiled aibbe-daemon binary:
#        go build -o ~/bin/aibbe-daemon ./daemon/
#      Then set: /home/<you>/bin/aibbe-daemon:/app/aibbe-daemon:ro
#
# START:    docker compose -f configs/docker/docker-compose.yml up -d
# STOP:     docker compose -f configs/docker/docker-compose.yml down
# BROWSER:  http://localhost:3000

services:
  chrome:
    image: linuxserver/chrome:latest
    environment:
      - PUID=1000    # Replace with: $(id -u)
      - PGID=1000    # Replace with: $(id -g)
      - AIBBE_SOCKET_PATH=/run/aibbe/aibbe.sock
    ports:
      - "3000:3000"
    shm_size: "1gb"
    security_opt:
      - seccomp:unconfined
    restart: unless-stopped
    volumes:
      - /REPLACE/WITH/ABSOLUTE/PATH/TO/aibbe-daemon:/app/aibbe-daemon:ro
      - ../../extension:/config/extensions/aibbe:ro
      - ./aibbe.nm-host.docker.json:/config/.config/chromium/NativeMessagingHosts/aibbe.json:ro
      - /tmp/aibbe-docker-socket:/run/aibbe:rw
      - aibbe-chrome-profile:/config:rw

volumes:
  aibbe-chrome-profile:
```

**Files**:
- `configs/docker/docker-compose.yml` (new file, ~35 lines)

**Validation**:
- [ ] `docker compose -f configs/docker/docker-compose.yml config --quiet` exits 0 (valid YAML, all keys recognized)
- [ ] Service name is `chrome`, image is `linuxserver/chrome:latest`
- [ ] `AIBBE_SOCKET_PATH=/run/aibbe/aibbe.sock` is set
- [ ] Named volume `aibbe-chrome-profile` is declared at top level
- [ ] Binary volume path placeholder is clearly labelled and NOT a real path
- [ ] `PUID`/`PGID` are labelled as placeholders with instructions in comments

---

### Subtask T003 — Cross-validate manifest path and compose volume bind are consistent

**Purpose**: Ensure that the path in the native host manifest (`path` field) and the compose volume mount for the manifest file are coherent. A mismatch here would silently break Native Messaging at runtime.

**Steps**:

1. Confirm that `configs/aibbe.nm-host.docker.json` has `"path": "/app/aibbe-daemon"`.

2. Confirm that `configs/docker/docker-compose.yml` includes the volume bind:
   ```
   - /REPLACE/WITH/ABSOLUTE/PATH/TO/aibbe-daemon:/app/aibbe-daemon:ro
   ```
   The container-side path must be exactly `/app/aibbe-daemon` — matching the manifest.

3. Confirm that the compose volume for the manifest is:
   ```
   - ./aibbe.nm-host.docker.json:/config/.config/chromium/NativeMessagingHosts/aibbe.json:ro
   ```
   The source (`./aibbe.nm-host.docker.json`) must be relative to the compose file's directory (`configs/docker/`). This means the actual source path resolves to `configs/aibbe.nm-host.docker.json` — one level up. Verify the `./` prefix is correct (it is, because the compose file is in `configs/docker/` and the manifest is in `configs/`).

4. If any path is inconsistent, fix it now — do not defer.

**Validation**:
- [ ] Manifest `path` == compose container-side binary path (`/app/aibbe-daemon`)
- [ ] Compose manifest bind source (`./aibbe.nm-host.docker.json`) resolves to `configs/aibbe.nm-host.docker.json` relative to compose file location
- [ ] No other path inconsistencies found

---

## Definition of Done

- [ ] `configs/aibbe.nm-host.docker.json` exists and is valid JSON
- [ ] `configs/docker/docker-compose.yml` exists and passes `docker compose config --quiet`
- [ ] Manifest `path` matches the container-side binary mount in compose
- [ ] Both PUID/PGID and binary path are clearly labelled as placeholders with instructions
- [ ] No source code files modified

---

## Risks

| Risk | Mitigation |
|---|---|
| Engineer forgets to replace PUID/PGID | Header comment + inline `# Replace with:` annotations |
| Engineer forgets to replace binary path | PLACEHOLDER string in caps, impossible to miss |
| `./aibbe.nm-host.docker.json` resolves wrongly | T003 explicitly verifies the relative path resolution |
| Chrome profile volume accidentally removed | Named volume `aibbe-chrome-profile` is distinct; guide will warn (WP02) |

---

## Reviewer Guidance

1. Validate the manifest JSON against the Chrome Native Messaging spec — specifically that `type` is `"stdio"` and `allowed_origins` uses the correct extension ID.
2. Run `docker compose -f configs/docker/docker-compose.yml config` (without `--quiet`) to see the fully-interpolated compose output and spot any path issues.
3. Verify the manifest volume bind resolves correctly: `configs/docker/../aibbe.nm-host.docker.json` == `configs/aibbe.nm-host.docker.json`.
