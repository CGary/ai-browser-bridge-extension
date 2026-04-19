<!-- Verified against configs/docker/docker-compose.yml and configs/aibbe.nm-host.docker.json on 2026-04-17 -->

# Quickstart: Dockerized aibbe Stack

## Overview

This guide sets up the full aibbe automation stack (daemon + Chrome extension) inside a
`linuxserver/chrome` Docker container, linked to a fresh Google account isolated from your
primary account. The host CLI communicates with the containerized daemon through a Unix socket
bind-mounted from the host — requiring **zero changes to the CLI binary**. The Chrome profile
(login state, extension data) persists across container restarts via a Docker named volume, so
you only log in once.

---

## Prerequisites

- Docker Engine 24+ installed on Debian 12 (or compatible Linux)
- Go 1.21+ installed (for compiling the daemon)
- The repository cloned at a known absolute path
- A separate Google account (not your primary account) — create one at accounts.google.com

---

## Step-by-step Setup

### Step 1 — Compile the daemon binary

The container runs Linux amd64. Always compile for that target.

```bash
# Standard (host is Linux amd64):
go build -o ~/bin/aibbe-daemon ./daemon/

# Cross-compile (if host is not Linux amd64):
GOOS=linux GOARCH=amd64 go build -o ~/bin/aibbe-daemon ./daemon/
```

Verify: `ls -lh ~/bin/aibbe-daemon` — should show a non-zero file size.

### Step 2 — Find your host UID and GID

```bash
id -u   # Example output: 1000
id -g   # Example output: 1000
```

Keep these values handy. A UID/GID mismatch between the host user and the container is the
most common setup error — it prevents the host CLI from writing to the socket.

### Step 3 — Configure docker-compose.yml

Edit `configs/docker/docker-compose.yml`. Make exactly three substitutions:

1. Replace `PUID=1000` with your UID from Step 2.
2. Replace `PGID=1000` with your GID from Step 2.
3. Replace `/REPLACE/WITH/ABSOLUTE/PATH/TO/aibbe-daemon` with the absolute path to the
   binary compiled in Step 1.

Example after editing (your values will differ):

```yaml
- PUID=1000          # your actual UID
- PGID=1000          # your actual GID
...
- /home/youruser/bin/aibbe-daemon:/app/aibbe-daemon:ro
```

### Step 4 — Create the socket directory

Docker bind-mounts require the source directory to exist on the host before the container
starts.

```bash
mkdir -p /tmp/aibbe-docker-socket
```

### Step 5 — Start the container

```bash
docker compose -f configs/docker/docker-compose.yml up -d
```

Expected output (first run creates the named volume):

```
[+] Running 2/2
 ✔ Volume "aibbe-chrome-profile" Created
 ✔ Container docker-chrome-1  Started
```

If port 9500 is already in use, stop the conflicting process or change the host-side port in
`configs/docker/docker-compose.yml` (e.g., `"9501:3000"`).

### Step 6 — Open Chrome in the browser

Open `http://localhost:9500` in your host browser. You will see the KasmVNC interface with a
full Chrome instance running inside the container.

### Step 7 — Sign in to the isolated Google account

Inside the container's Chrome:

1. Navigate to `https://accounts.google.com`.
2. Sign in with the **isolated** Google account — not your primary account.
3. Complete any 2FA or account-verification steps.

This session is persisted in the `aibbe-chrome-profile` Docker named volume.

### Step 8 — Load the extension

Inside the container's Chrome:

1. Navigate to `chrome://extensions`.
2. Enable **Developer mode** (toggle in the top-right corner of the page).
3. Click **Load unpacked**.
4. Enter the path `/config/extensions/aibbe` — this is where the extension source is
   volume-mounted from `./extension/` in the repository.
5. Confirm the "AI Browser Bridge Extension" appears in the extension list with no errors.

This step is required only once. After the first load, the extension installation persists in
the named volume across container restarts.

### Step 9 — Verify the extension loaded correctly

1. In `chrome://extensions`, click the **Service Worker** link under the AI Browser Bridge
   Extension entry.
2. The DevTools console for the background service worker opens.
3. You should see a connection log line when Chrome launches the native messaging host.

If you see errors:

- **"Native host not found"**: The manifest at
  `/config/.config/chromium/NativeMessagingHosts/aibbe.json` is missing or has a wrong path.
  Confirm the compose volume bind for `../aibbe.nm-host.docker.json` is correct and the
  container was restarted after editing `docker-compose.yml`.
- **Extension ID mismatch**: The ID shown in `chrome://extensions` must match the
  `allowed_origins` value in `configs/aibbe.nm-host.docker.json`. If the extension was
  side-loaded with a different profile, the ID may differ — note the displayed ID and update
  the manifest file.

### Step 10 — Test the CLI round-trip

From the **host machine** terminal (not inside the container):

```bash
AIBBE_SOCKET_PATH=/tmp/aibbe-docker-socket/aibbe.sock \
  go run cmd/cli/main.go -cmd ping -payload hello
```

Expected output (the daemon relays the request through the extension, which echoes it back):

```json
{"cmd":"ping","payload":"hello"}
```

The daemon acts as a transparent relay — it forwards any command to the extension and returns
whatever the extension sends back. The current extension echoes requests unchanged, so the
round-trip output mirrors the input.

If the CLI exits with code 1 or hangs, see the Troubleshooting section below.

### Step 11 — Stopping the container

```bash
docker compose -f configs/docker/docker-compose.yml down
```

The `aibbe-chrome-profile` named volume is **preserved** — your login session and extension
installation survive the stop.

> **Warning**: Do **not** run `docker compose down -v`. The `-v` flag deletes named volumes,
> which would destroy your Chrome profile and require you to repeat Steps 7–8.

### Step 12 — Updating the daemon binary

Recompile on the host (repeat Step 1), then restart the container:

```bash
go build -o ~/bin/aibbe-daemon ./daemon/
docker compose -f configs/docker/docker-compose.yml restart chrome
```

No image rebuild is needed. The binary is bind-mounted from the host, so the container picks
up the new version on the next Chrome launch.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| CLI exits 1 immediately: "no such file" or "connection refused" | Container not running | `docker compose -f configs/docker/docker-compose.yml ps` — start if stopped |
| CLI exits 1: socket path not found | Socket directory missing or wrong `AIBBE_SOCKET_PATH` | `mkdir -p /tmp/aibbe-docker-socket`; verify env var |
| CLI hangs indefinitely, then times out | UID/GID mismatch — socket owned by different user | Confirm `PUID`/`PGID` in compose match `id -u` / `id -g` on host |
| Chrome crashes on launch inside container | Shared memory too small | `shm_size: "1gb"` is set; increase to `"2gb"` if crashes persist |
| "Native host not found" in extension console | Manifest not mounted at correct path | Confirm `../aibbe.nm-host.docker.json` volume bind; restart container |
| Extension missing after container restart | Extension loaded into ephemeral layer, not named volume | Ensure `aibbe-chrome-profile:/config:rw` volume is mounted — reload extension if needed |

---

## Session Persistence

The `aibbe-chrome-profile` Docker named volume stores the entire `/config` directory inside
the container. This includes:

- The Chromium user profile (cookies, session tokens, login state)
- The installed extension (loaded via Developer Mode in Step 8)
- Any other Chrome configuration

A container restart (`docker compose down` followed by `up -d`) automatically restores the
profile from the named volume. You do not need to log in again as long as the volume exists.

If the volume is accidentally deleted (`docker volume rm aibbe-chrome-profile` or
`docker compose down -v`), repeat Steps 7 and 8 to restore the session.

To list your Docker volumes and confirm the profile volume exists:

```bash
docker volume ls | grep aibbe
```
