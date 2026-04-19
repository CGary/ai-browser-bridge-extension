# Quickstart: Containerized aibbe Stack

**Audience**: Engineer setting up the Docker-based aibbe environment for the first time.

---

## Prerequisites

- Docker installed and running on Debian 12
- `docker compose` v2 available (`docker compose version`)
- The `aibbe` repository cloned locally

---

## Step 1 — Compile the Daemon for Linux

From the repository root:

```bash
GOOS=linux GOARCH=amd64 go build -o ./bin/aibbe-daemon ./daemon/
```

If the host is already Linux amd64, omit the env vars.

---

## Step 2 — Find Your Host UID/GID

```bash
id -u   # example output: 1000
id -g   # example output: 1000
```

Keep these values — you will need them in the next step.

---

## Step 3 — Configure docker-compose.yml

Open `configs/docker/docker-compose.yml` and set:

- `PUID` → your UID from Step 2
- `PGID` → your GID from Step 2
- The daemon binary volume → absolute path to `./bin/aibbe-daemon` on your host

---

## Step 4 — Create the Socket Directory

```bash
mkdir -p /tmp/aibbe-docker-socket
```

This directory is the bridge between the containerized daemon and the host CLI.

---

## Step 5 — Start the Container

```bash
docker compose -f configs/docker/docker-compose.yml up -d
```

Verify it is running:

```bash
docker compose -f configs/docker/docker-compose.yml ps
```

---

## Step 6 — Access Chrome

Open your host browser and go to:

```
http://localhost:9500
```

You will see a full Chrome browser rendered via the KasmVNC web UI.

---

## Step 7 — Log Into the Isolated Google Account

In the containerized Chrome, sign into the dedicated Google account (not your primary account). Navigate to `https://notebooklm.google.com/` to verify access.

---

## Step 8 — Load the Extension

1. Go to `chrome://extensions` inside the containerized Chrome.
2. Enable **Developer mode** (toggle, top right).
3. Click **Load unpacked**.
4. Navigate to `/config/extensions/aibbe` (the volume-mounted extension directory).
5. Confirm the extension appears without errors.

---

## Step 9 — Verify Native Messaging

1. In the extension list, click **Service Worker** under the aibbe extension.
2. The DevTools console should open. Check for connection errors.
3. If you see a native messaging connection error, verify the manifest was mounted correctly at `/config/.config/chromium/NativeMessagingHosts/aibbe.json` and that the daemon binary is executable.

---

## Step 10 — Test the CLI Round-Trip

From the host:

```bash
AIBBE_SOCKET_PATH=/tmp/aibbe-docker-socket/aibbe.sock \
  go run cmd/cli/main.go -cmd ping
```

A successful response confirms the full pipeline: host CLI → socket → containerized daemon → extension → Chrome.

---

## Stopping the Container

```bash
docker compose -f configs/docker/docker-compose.yml down
```

The Chrome profile (session, cookies, login state) is preserved in the `aibbe-chrome-profile` Docker named volume and will be restored on the next `up`.

---

## Updating the Daemon

Recompile on the host:

```bash
GOOS=linux GOARCH=amd64 go build -o ./bin/aibbe-daemon ./daemon/
```

The container picks up the new binary automatically on the next Chrome-initiated Native Messaging connection (no container restart needed).

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| CLI exits immediately with connection error | Container not running or socket dir not mounted | `docker compose ps`; check volume config |
| Socket created but CLI gets permission denied | `PUID` doesn't match host UID | Set `PUID=$(id -u)` in docker-compose.yml |
| Extension fails to load | Developer mode not enabled | Re-enable Developer mode in `chrome://extensions` |
| Native Messaging error in extension console | Manifest not at correct path or binary not executable | Check volume mount paths in docker-compose.yml |
| Chrome session lost after container restart | Named volume was removed | Do not `docker volume rm aibbe-chrome-profile` |
