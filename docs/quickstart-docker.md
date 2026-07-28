# Quickstart: Dockerized aibbe Stack

## Overview

This guide sets up the full aibbe automation stack (daemon + Chrome extension) inside a
`linuxserver/chrome` Docker container whose traffic is routed through a VPN container
(gluetun + ProtonVPN), linked to a fresh Google account isolated from your primary account.
The daemon runs inside the container and listens on `/tmp/aibbe.sock` **inside the container's
filesystem**, so the CLI is used via `docker exec`. The Chrome profile (login state, extension
data) persists across container restarts via a Docker named volume, so you only log in once.

The compose stack defines two services:

- `vpn` (`qmcgaw/gluetun`): OpenVPN tunnel to ProtonVPN. Exposes port `9500` for the browser UI.
- `chrome` (`linuxserver/chrome`): shares the VPN's network namespace (`network_mode: service:vpn`)
  and only starts once the VPN is healthy.

---

## Prerequisites

- Docker Engine 24+ on Linux
- Go 1.21+ (for compiling daemon and CLI)
- The repository cloned at a known absolute path
- A separate Google account (not your primary account)
- Proton VPN OpenVPN credentials (free tier works — the compose sets `FREE_ONLY=on`)

---

## Step-by-step Setup

### Step 1 — Compile the binaries into `bin/`

The compose file bind-mounts `bin/aibbe-daemon` and `bin/aibbe-cli` from the repository root.
The container runs Linux amd64 — always compile for that target.

```bash
GOOS=linux GOARCH=amd64 go build -o bin/aibbe-daemon ./daemon/
GOOS=linux GOARCH=amd64 go build -o bin/aibbe-cli ./cmd/cli/
```

`bin/` is git-ignored; these binaries only live on your machine.

### Step 2 — Configure VPN credentials

```bash
cp configs/docker/vpn.env.example configs/docker/vpn.env
# Edit configs/docker/vpn.env with your Proton VPN OpenVPN username/password
# (get them at https://account.proton.me/ → OpenVPN/IKEv2 credentials)
```

`configs/docker/*.env` is git-ignored — secrets stay local. Without this file,
`docker compose up` fails.

### Step 3 — Check PUID/PGID (usually no edit needed)

The compose sets `PUID=1000` / `PGID=1000`. If `id -u` / `id -g` on your host differ,
update those values in `configs/docker/docker-compose.yml`.

### Step 4 — Start the stack

```bash
docker compose -f configs/docker/docker-compose.yml up -d
```

This starts two containers: `vpn` first (waits for its healthcheck against `1.1.1.1`),
then `chrome`. First run also creates the `aibbe-chrome-profile` named volume.

If port 9500 is already in use, change the host-side port on the `vpn` service
(e.g., `"9501:3000"`).

### Step 5 — Open Chrome in the browser

Open `http://localhost:9500` (user `admin`, pass `admin`). You will see the KasmVNC
interface with a full Chrome instance running inside the container.

### Step 6 — Sign in to the isolated Google account

Inside the container's Chrome:

1. Navigate to `https://accounts.google.com`.
2. Sign in with the **isolated** Google account — not your primary account.
3. Complete any 2FA or account-verification steps.

This session persists in the `aibbe-chrome-profile` named volume.

### Step 7 — Load the extension

Inside the container's Chrome:

1. Navigate to `chrome://extensions`.
2. Enable **Developer mode**.
3. Click **Load unpacked** and enter `/config/extensions/aibbe` (volume-mounted from
   `./extension/` in the repository).
4. Confirm the extension appears with no errors and with ID
   `bedlojjaiogmaefoadfpdecgajipcpgj`.

Required only once — the installation persists in the named volume.

### Step 8 — Verify the native messaging host

1. In `chrome://extensions`, click the **Service worker** link under the extension entry.
2. You should see a connection log line when Chrome launches the native host.

If you see **"Native host not found"**: the manifest is mounted at
`/config/.config/google-chrome/NativeMessagingHosts/aibbe.json` (note: `google-chrome`,
not `chromium` — the image runs Google Chrome). Confirm the compose volume bind for
`../aibbe.nm-host.docker.json` and restart the container.

### Step 9 — Test the CLI round-trip

Open a NotebookLM notebook in the container's Chrome, then from the host:

```bash
docker exec chrome aibbe-cli -cmd probe-selectors
docker exec chrome aibbe-cli -cmd generate -payload "hola"
```

The daemon socket (`/tmp/aibbe.sock`) lives **inside** the container, so the CLI must run
there too. The compose file does not bind-mount the socket to the host.

If the CLI exits with code 1 or hangs, see Troubleshooting below.

### Step 10 — Stopping the stack

```bash
docker compose -f configs/docker/docker-compose.yml down
```

The `aibbe-chrome-profile` named volume is **preserved** — login session and extension
installation survive the stop.

> **Warning**: Do **not** run `docker compose down -v`. The `-v` flag deletes named volumes,
> destroying the Chrome profile (repeat Steps 6–7 to recover).

### Step 11 — Updating the binaries

Recompile on the host (Step 1), then restart the chrome container:

```bash
GOOS=linux GOARCH=amd64 go build -o bin/aibbe-daemon ./daemon/
docker compose -f configs/docker/docker-compose.yml restart chrome
```

No image rebuild needed — the binaries are bind-mounted.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `docker compose up` fails reading `vpn.env` | Missing VPN credentials file | Step 2: copy `vpn.env.example` → `vpn.env` and fill it in |
| `chrome` container never starts | VPN healthcheck failing (bad credentials, no tunnel) | `docker logs vpn`; verify ProtonVPN OpenVPN credentials |
| CLI: "connection refused" / "no such file" | Daemon not running inside container, or CLI run on the host | Run via `docker exec chrome aibbe-cli ...`; check container status |
| `no_free_tabs` | No NotebookLM tab registered via handshake | Open/refresh a notebook tab in the container's Chrome |
| Chrome crashes on launch inside container | Shared memory too small | `shm_size: "1gb"` is set; increase to `"2gb"` if it persists |
| "Native host not found" in extension console | Manifest not mounted at the `google-chrome` path | See Step 8; restart container after compose edits |
| Extension missing after restart | Profile volume not mounted | Ensure `aibbe-chrome-profile:/config:rw` is present |

---

## Session Persistence

The `aibbe-chrome-profile` named volume stores the entire `/config` directory:
Chrome user profile (cookies, login state), the loaded extension, and calibration
overrides in `chrome.storage.local`.

A restart (`down` + `up -d`) restores everything automatically. If the volume is deleted,
repeat Steps 6–7.

```bash
docker volume ls | grep aibbe
```
