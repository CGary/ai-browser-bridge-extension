# Research: Dockerized aibbe Stack with Isolated Chrome Account

**Mission**: dockerized-aibbe-stack-isolated-chrome-01KPCPFB  
**Date**: 2026-04-17

---

## 1. linuxserver/chrome — Filesystem Layout

**Decision**: Use `/config` as the persistent data root inside the container.

**Rationale**: The `linuxserver/chrome` image maps all persistent Chrome state to `/config`. The effective home directory is `/config`, which means:
- Chrome profile: `/config/.config/chromium/Default/`
- Native host manifests: `/config/.config/chromium/NativeMessagingHosts/`
- Chrome itself runs as user `abc` with UID/GID remapped to `PUID`/`PGID` env vars.

**Alternatives considered**: `/home/abc/` (default in some images) — not applicable here; linuxserver explicitly uses `/config`.

---

## 2. Native Messaging in a Docker Container

**Decision**: The native host manifest and binary are both accessible from inside the container. Chrome launches the daemon as a subprocess at the path declared in the manifest.

**Rationale**: Native Messaging works as follows inside the container:
1. `background.js` calls `chrome.runtime.connectNative('aibbe')`.
2. Chrome reads the manifest at `/config/.config/chromium/NativeMessagingHosts/aibbe.json`.
3. Chrome launches the binary declared in `manifest.path` (e.g. `/app/aibbe-daemon`).
4. The daemon communicates with Chrome via `stdin`/`stdout` (existing protocol, unchanged).
5. The daemon creates its Unix socket at `AIBBE_SOCKET_PATH` inside the container.

The binary path in the manifest must be absolute and point to the binary's location inside the container. A volume-mounted binary at `/app/aibbe-daemon` satisfies this without image rebuilds.

**Alternatives considered**: Building the binary inside the container image — requires a custom Dockerfile and image rebuild on every daemon update. Rejected in favour of volume mount.

---

## 3. Unix Socket Across Container Boundary

**Decision**: Use a dedicated host directory (e.g. `/tmp/aibbe-docker-socket/`) bind-mounted into the container at a fixed path (e.g. `/run/aibbe/`). The daemon writes its socket into that mount. The host CLI targets the host-side path.

**Rationale**:
- Bind-mounting a directory (not a file) is the reliable pattern for sharing a Unix socket that a process creates at runtime.
- Mounting `/tmp` directly risks cross-contamination with other host processes.
- A dedicated directory keeps the socket path explicit and avoids conflicts with a local (non-Docker) daemon that also uses `/tmp/aibbe.sock`.
- The existing `AIBBE_SOCKET_PATH` environment variable already parameterises both the daemon and (by convention) the CLI, so no source-code changes are needed.

**Socket paths**:
- Inside container: `/run/aibbe/aibbe.sock` (via `AIBBE_SOCKET_PATH=/run/aibbe/aibbe.sock`)
- On host: `/tmp/aibbe-docker-socket/aibbe.sock` (bind-mounted from host dir)
- Host CLI invocation: `AIBBE_SOCKET_PATH=/tmp/aibbe-docker-socket/aibbe.sock go run cmd/cli/main.go ...`

**Alternatives considered**: Mounting `/tmp` wholesale — works but is too broad. Mounting the socket file directly — fails because the file doesn't exist before the daemon creates it.

---

## 4. UID/GID Alignment for Socket Permissions

**Decision**: Set `PUID` in `docker-compose.yml` to match the host engineer's UID. Set `PGID` to match the engineer's primary GID.

**Rationale**: The daemon creates the socket with permissions `0600` and owner = the container user (remapped to `PUID`). For the host CLI to connect, the host user's UID must match `PUID`. The `linuxserver/chrome` image supports arbitrary `PUID`/`PGID` via environment variables — no image modification needed.

**How to find host UID**: `id -u` (typically `1000` on a single-user Debian system).

**Alternatives considered**: Relaxing socket permissions to `0660` — breaks the security model established in the charter (0600 is a hard constraint). Rejected.

---

## 5. Extension Loading in linuxserver/chrome

**Decision**: Volume-mount the host `extension/` directory into the container; load it once via Chrome's "Load unpacked" UI at the mount path. This is a one-time manual step per container instance.

**Rationale**: The `linuxserver/chrome` web UI (port 9500) gives full access to Chrome's interface including DevTools and the Extensions page. Loading via volume mount means extension updates on the host are reflected inside the container on reload — no re-copy needed.

**Mount point inside container**: `/config/extensions/aibbe/` (writable by the Chrome user).

**Alternatives considered**: Packing the extension as a `.crx` — requires key management and signing. Rejected for complexity.

---

## 6. Native Host Manifest — Docker Variant

**Decision**: Create a separate manifest file `configs/aibbe.nm-host.docker.json` with the container-internal binary path. Mount it into the container at the correct NativeMessagingHosts location via Docker volume.

**Rationale**: The existing `configs/aibbe.nm-host.json` points to the host binary path (for the host Chrome installation). A Docker-specific variant avoids modifying the production manifest and makes the separation explicit.

**Container manifest path**: `/config/.config/chromium/NativeMessagingHosts/aibbe.json`  
**Binary path in manifest**: `/app/aibbe-daemon`

---

## 7. docker-compose.yml as First-Class Repo Artifact

**Decision**: Commit `configs/docker/docker-compose.yml` to the repository.

**Rationale**: Captures the full container configuration (image, volumes, environment, ports, UID) in version control. Eliminates manual `docker run` flag reconstruction on every setup. Consistent with the project's fail-fast philosophy: configuration is explicit, not reconstructed from memory.
