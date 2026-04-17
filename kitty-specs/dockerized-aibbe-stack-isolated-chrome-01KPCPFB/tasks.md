# Tasks: Dockerized aibbe Stack with Isolated Chrome Account

**Mission**: dockerized-aibbe-stack-isolated-chrome-01KPCPFB  
**Mission ID**: 01KPCPFBPV1Y8QBSW7GP0PQ5XM  
**Target Branch**: main  
**Generated**: 2026-04-17T03:33:10Z

---

## Overview

All work is configuration and documentation — no Go or JavaScript source changes.
Three files are created: a Docker-specific native host manifest, a Docker Compose
configuration, and a developer quickstart guide.

---

## Subtask Index

| ID | Description | WP | Parallel |
|---|---|---|---|
| T001 | Create `configs/aibbe.nm-host.docker.json` — Docker native host manifest | WP01 | [P] |
| T002 | Create `configs/docker/docker-compose.yml` — Container configuration | WP01 | [P] |
| T003 | Cross-validate manifest `path` and compose volume bind are consistent | WP01 | — |
| T004 | Create `docs/quickstart-docker.md` — 12-step setup guide | WP02 | — |
| T005 | Verify all paths and commands in the guide match the WP01 artifacts | WP02 | — |

---

## Work Packages

---

### WP01 — Docker Configuration Artifacts

**Goal**: Create the two configuration files that define the containerized stack: the Docker-specific native host manifest and the Docker Compose service definition.  
**Priority**: High — must exist before the quickstart guide can reference them.  
**Independent test**: `docker compose -f configs/docker/docker-compose.yml config --quiet` exits 0 (YAML is valid); `cat configs/aibbe.nm-host.docker.json | python3 -m json.tool` exits 0 (JSON is valid).  
**Estimated prompt size**: ~260 lines  
**Prompt file**: [tasks/WP01-docker-config-artifacts.md](tasks/WP01-docker-config-artifacts.md)

**Included subtasks**:

- [ ] T001 Create `configs/aibbe.nm-host.docker.json` — Docker native host manifest (WP01)
- [ ] T002 Create `configs/docker/docker-compose.yml` — Container configuration (WP01)
- [ ] T003 Cross-validate manifest `path` and compose volume bind are consistent (WP01)

**Implementation sketch**:
1. Copy the existing `configs/aibbe.nm-host.json` structure; change only `path` to `/app/aibbe-daemon`.
2. Create `configs/docker/` directory; write `docker-compose.yml` with all volumes, env vars, and security options from the plan.
3. Verify that the compose bind mount for the NativeMessagingHosts JSON references the same file just created.

**Parallel opportunities**: T001 and T002 can be written simultaneously (different files, no shared state).  
**Dependencies**: None — first WP.  
**Risks**:
- Placeholder values (`<host-uid>`, `<host-gid>`, `<absolute-path-to-binary>`) must be clearly labelled in the compose file so the engineer knows to replace them.
- Named volume `aibbe-chrome-profile` must not collide with any existing Docker volume on the host — the name is distinctive enough.

---

### WP02 — Developer Quickstart Guide

**Goal**: Create `docs/quickstart-docker.md` — a complete step-by-step guide that takes the engineer from zero to a working CLI round-trip through the containerized stack.  
**Priority**: High — the feature is unusable without this guide.  
**Independent test**: All commands in the guide can be copy-pasted without modification after filling in the two placeholder values (`PUID`/`PGID`); the guide references paths that exist in the repository after WP01 is merged.  
**Estimated prompt size**: ~230 lines  
**Prompt file**: [tasks/WP02-developer-quickstart-guide.md](tasks/WP02-developer-quickstart-guide.md)

**Included subtasks**:

- [ ] T004 Create `docs/quickstart-docker.md` — 12-step setup guide (WP02)
- [ ] T005 Verify all paths and commands in the guide match the WP01 artifacts (WP02)

**Implementation sketch**:
1. Follow the 12-step outline defined in `plan.md` § Work Area 3.
2. For each step, add the exact command, the expected output or indicator of success, and a brief note about why.
3. After drafting, audit every path reference against the files created in WP01.

**Parallel opportunities**: None within this WP (T005 depends on T004).  
**Dependencies**: WP01 — the guide references `configs/docker/docker-compose.yml` and `configs/aibbe.nm-host.docker.json` by path.  
**Risks**:
- Step 10 (`go run cmd/cli/main.go -cmd ping`) requires the daemon to implement a ping handler — if it does not, the guide should note this and suggest an alternative first command. Verify against the daemon source before writing step 10.
- Cross-compile instruction (`GOOS=linux GOARCH=amd64`) is needed only if the host is not amd64 Linux. Mention both the simple and cross-compile cases.
