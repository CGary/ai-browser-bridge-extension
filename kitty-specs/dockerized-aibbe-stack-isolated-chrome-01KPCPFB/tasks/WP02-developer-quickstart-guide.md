---
work_package_id: WP02
title: Developer Quickstart Guide
dependencies:
- WP01
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-005
- FR-006
- FR-007
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T004
- T005
history:
- date: '2026-04-17T03:33:10Z'
  event: created
authoritative_surface: docs/
execution_mode: code_change
owned_files:
- docs/quickstart-docker.md
tags: []
---

# WP02 — Developer Quickstart Guide

## Objective

Create `docs/quickstart-docker.md` — a complete, copy-paste-ready guide that takes the engineer from zero to a verified CLI round-trip through the containerized aibbe stack, in under 30 minutes.

---

## Context

The guide is the primary (and only) interface between the configuration artifacts created in WP01 and the engineer performing the initial setup. It must:

- Explain the purpose of each step (not just list commands)
- Include the exact commands, with expected terminal output where relevant
- Cover both the happy path AND the two most common failure modes (UID mismatch, Chrome sandbox)
- Reference the actual file paths from WP01 (`configs/docker/docker-compose.yml`, `configs/aibbe.nm-host.docker.json`)

**Key facts about the daemon and pipeline** (from `daemon/main.go` and tests):
- The daemon is an echo relay — it forwards any request payload to the extension and returns whatever the extension sends back.
- The extension currently echoes requests back unchanged (background.js).
- Therefore, a round-trip test with `-cmd ping -payload hello` should return `{"cmd":"ping","payload":"hello"}`.
- The CLI binary path is `cmd/cli/main.go`; run with `go run cmd/cli/main.go` or the compiled binary.
- Socket path is controlled by `AIBBE_SOCKET_PATH` env var (default `/tmp/aibbe.sock`; Docker path: `/tmp/aibbe-docker-socket/aibbe.sock`).

**Dependencies**: This WP depends on WP01. The guide must reference the exact file paths produced by WP01. Read WP01 artifacts before writing step references.

---

## Branch Strategy

- **Planning base branch**: `main`
- **Final merge target**: `main`
- Execution worktrees are allocated per computed lane from `lanes.json`.
- Run with: `spec-kitty agent action implement WP02 --agent <name>`

---

## Subtask Guidance

### Subtask T004 — Create `docs/quickstart-docker.md`

**Purpose**: Write the authoritative setup guide. Engineers who have never touched this feature should be able to follow it from start to finish without any external knowledge.

**Structure** (follow this order exactly):

```
# Quickstart: Dockerized aibbe Stack

## Overview (3–5 sentences)
## Prerequisites
## Step-by-step Setup
### Step 1 — Compile the daemon binary
### Step 2 — Find your host UID and GID
### Step 3 — Configure docker-compose.yml
### Step 4 — Create the socket directory
### Step 5 — Start the container
### Step 6 — Open Chrome in the browser
### Step 7 — Sign in to the isolated Google account
### Step 8 — Load the extension
### Step 9 — Verify the extension loaded correctly
### Step 10 — Test the CLI round-trip
### Step 11 — Stopping the container
### Step 12 — Updating the daemon binary
## Troubleshooting
## Session Persistence
```

---

**Detailed content for each section**:

#### Overview
- Explain that the stack runs the aibbe daemon and Chrome extension inside a `linuxserver/chrome` Docker container, linked to a fresh Google account (isolated from the primary account).
- State that the host CLI communicates with the containerized daemon through a Unix socket bind-mounted from the host, requiring zero changes to the CLI binary.
- Note that the Chrome profile (login state, extension data) persists across container restarts via a Docker named volume.

#### Prerequisites
List exactly:
- Docker Engine 24+ installed on Debian 12 (or compatible Linux)
- Go 1.21+ installed (for compiling the daemon)
- The repository cloned at a known path
- A separate Google account (not the primary account) — engineer-managed

#### Step 1 — Compile the daemon binary

```bash
# Standard (host is Linux amd64):
go build -o ~/bin/aibbe-daemon ./daemon/

# Cross-compile (if host is not Linux amd64):
GOOS=linux GOARCH=amd64 go build -o ~/bin/aibbe-daemon ./daemon/
```

Note: The binary will be volume-mounted into the container at `/app/aibbe-daemon`. The container runs Linux amd64, so always use the Linux amd64 target.

Expected output: No output (success). Verify with `ls -lh ~/bin/aibbe-daemon`.

#### Step 2 — Find your host UID and GID

```bash
id -u   # Example output: 1000
id -g   # Example output: 1000
```

You will use these values in the next step. Mismatched values cause a socket permission error.

#### Step 3 — Configure docker-compose.yml

Edit `configs/docker/docker-compose.yml`. Make exactly two changes:

1. Replace `PUID=1000` with your UID from step 2.
2. Replace `PGID=1000` with your GID from step 2.
3. Replace `/REPLACE/WITH/ABSOLUTE/PATH/TO/aibbe-daemon` with the absolute path to the binary compiled in Step 1.

Example after editing:
```yaml
- PUID=1000
- PGID=1000
...
- /home/engineer/bin/aibbe-daemon:/app/aibbe-daemon:ro
```

#### Step 4 — Create the socket directory

```bash
mkdir -p /tmp/aibbe-docker-socket
```

Docker bind-mounts require the source directory to exist on the host before the container starts.

#### Step 5 — Start the container

```bash
docker compose -f configs/docker/docker-compose.yml up -d
```

Expected output:
```
[+] Running 2/2
 ✔ Volume "aibbe-chrome-profile" Created
 ✔ Container docker-chrome-1  Started
```

If you see errors about port 3000 in use, stop the conflicting process or change the port mapping in `docker-compose.yml`.

#### Step 6 — Open Chrome in the browser

Open `http://localhost:3000` in your host browser. You will see the KasmVNC interface showing a full Chrome instance running inside the container.

#### Step 7 — Sign in to the isolated Google account

Inside the container's Chrome:
1. Navigate to `accounts.google.com`.
2. Sign in with the **isolated** Google account (not your primary account).
3. Complete any 2FA or verification steps.

This session will be persisted in the `aibbe-chrome-profile` Docker volume.

#### Step 8 — Load the extension

Inside the container's Chrome:
1. Navigate to `chrome://extensions`.
2. Enable **Developer mode** (toggle in the top-right corner).
3. Click **Load unpacked**.
4. Select the path `/config/extensions/aibbe` (this is where the extension source is mounted).
5. The "AI Browser Bridge Extension" should appear in the extensions list.

This step is required only once per Chrome profile. After the first load, the extension persists in the named volume.

#### Step 9 — Verify the extension loaded correctly

1. Click the **Service Worker** link next to the extension in `chrome://extensions`.
2. The DevTools console for the background worker should open.
3. You should see a log line indicating the native messaging port connected successfully.

If you see errors, check:
- The extension ID in `configs/aibbe.nm-host.docker.json` matches the ID shown in `chrome://extensions`.
- The daemon binary at `/app/aibbe-daemon` is executable inside the container.

#### Step 10 — Test the CLI round-trip

From the **host machine** terminal (not inside the container):

```bash
AIBBE_SOCKET_PATH=/tmp/aibbe-docker-socket/aibbe.sock \
  go run cmd/cli/main.go -cmd ping -payload hello
```

Expected output (the daemon echoes the request back through the extension):
```json
{"cmd":"ping","payload":"hello"}
```

If the CLI hangs or exits with code 1:
- Verify the container is running: `docker compose -f configs/docker/docker-compose.yml ps`
- Verify the socket file exists: `ls -la /tmp/aibbe-docker-socket/`
- Verify UID/GID match: the socket file should be owned by your user

#### Step 11 — Stopping the container

```bash
docker compose -f configs/docker/docker-compose.yml down
```

The Chrome profile (login state, extension data) is preserved in the `aibbe-chrome-profile` named volume. The next `up -d` will restore the session automatically.

**Do not** run `docker compose down -v` — the `-v` flag deletes named volumes, including your Chrome profile.

#### Step 12 — Updating the daemon binary

Recompile on the host (Step 1) and restart the container:

```bash
go build -o ~/bin/aibbe-daemon ./daemon/
docker compose -f configs/docker/docker-compose.yml restart chrome
```

The new binary is picked up immediately because the volume is bind-mounted — no image rebuild required.

#### Troubleshooting section

Include a table:

| Symptom | Likely cause | Fix |
|---|---|---|
| CLI exits 1 immediately, socket not found | Socket directory not created or wrong path | `mkdir -p /tmp/aibbe-docker-socket`; verify `AIBBE_SOCKET_PATH` |
| CLI exits 1, connection refused | Container not running | `docker compose ... up -d` |
| CLI hangs indefinitely | UID mismatch — socket owned by different user | Verify PUID/PGID in compose match `id -u` / `id -g` |
| Chrome crashes on launch | Shared memory too small | `shm_size: "1gb"` is set; if still crashing, increase to `"2gb"` |
| "Native host not found" in extension console | Manifest not mounted correctly or wrong path | Verify `/config/.config/chromium/NativeMessagingHosts/aibbe.json` exists inside the container |
| Extension not loaded after restart | Extension was loaded but not into the persistent profile | Reload from `chrome://extensions` → the named volume must be mounted at `/config` |

#### Session Persistence section

Explain:
- The `aibbe-chrome-profile` named volume stores the entire `/config` directory inside the container, including the Chromium profile.
- Session cookies, login state, and extension installation all persist inside this volume.
- A container restart (not `down -v`) restores the profile automatically.
- If the volume is deleted, the engineer must repeat Steps 7–8.

**Files**:
- `docs/quickstart-docker.md` (new file, ~150 lines)

**Validation**:
- [ ] All 12 steps are present and follow the plan's outline
- [ ] All commands are copy-paste ready (no syntax errors, no missing quotes)
- [ ] Troubleshooting table covers the five documented risk scenarios from `plan.md`
- [ ] Session persistence section explains the named volume behavior
- [ ] File is valid Markdown (renders correctly)

---

### Subtask T005 — Verify path consistency between the guide and WP01 artifacts

**Purpose**: A guide that references wrong paths is worse than no guide — it wastes the engineer's time and erodes trust. Audit every path reference in the guide against the actual files on disk.

**Steps**:

1. Read `configs/aibbe.nm-host.docker.json`. Confirm every reference to this file in the guide uses the correct relative path from the repo root.

2. Read `configs/docker/docker-compose.yml`. Confirm:
   - The guide's `docker compose -f` command references `configs/docker/docker-compose.yml` (relative to repo root).
   - The socket path `/tmp/aibbe-docker-socket/aibbe.sock` in the guide's `AIBBE_SOCKET_PATH` matches the compose bind mount target (`/run/aibbe`) + the socket filename `aibbe.sock`.
   - The extension path in the guide's "Load unpacked" step (`/config/extensions/aibbe`) matches the compose volume bind container-side path.
   - The manifest path in the guide's "Native host not found" troubleshooting entry matches the actual compose mount target.

3. Verify that the CLI command in Step 10 is correct:
   - The repo has `cmd/cli/main.go` (confirm with a file existence check, not just memory).
   - The `-cmd` and `-payload` flags are the correct flag names (check `cmd/cli/main.go` flag definitions if uncertain).

4. Fix any discrepancies found. If everything is consistent, add a one-line comment at the top of the file (YAML front matter style is NOT used for markdown docs — just a HTML comment):

```markdown
<!-- Verified against configs/docker/docker-compose.yml and configs/aibbe.nm-host.docker.json on 2026-04-17 -->
```

**Validation**:
- [ ] All `docker compose -f` commands reference the correct relative path
- [ ] `AIBBE_SOCKET_PATH` value in the guide matches the host-side bind mount path + socket filename
- [ ] CLI flags (`-cmd`, `-payload`) match the actual flag definitions in `cmd/cli/main.go`
- [ ] Extension path in "Load unpacked" step matches compose mount
- [ ] Verification comment added to top of file

---

## Definition of Done

- [ ] `docs/quickstart-docker.md` exists and is valid Markdown
- [ ] All 12 steps are present, accurate, and reference correct file paths
- [ ] Troubleshooting table covers all documented risk scenarios
- [ ] All paths verified against WP01 artifacts (T005)
- [ ] No source code files modified

---

## Risks

| Risk | Mitigation |
|---|---|
| Step 10 command is wrong (wrong flags or path) | T005 explicitly verifies CLI flags against source |
| Guide becomes stale if file paths change | Verification comment timestamps the audit |
| Engineer runs `down -v` and loses profile | Explicit warning in Step 11 and Session Persistence section |
| Extension ID mismatch (guide hardcodes ID) | Guide references `chrome://extensions` to find the real ID, not the hardcoded value |

---

## Reviewer Guidance

1. Follow the guide yourself (mentally trace each command) — does each step lead naturally to the next?
2. Verify the CLI command in Step 10 produces the expected output given the echo behavior of the current extension.
3. Check the troubleshooting table covers scenarios 2, 3, and 5 from the spec (`spec.md`).
4. Confirm the guide is self-contained — no external links or assumed knowledge beyond what's listed in Prerequisites.
