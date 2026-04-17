# Spec: Dockerized aibbe Stack with Isolated Chrome Account

**Mission ID**: 01KPCPFBPV1Y8QBSW7GP0PQ5XM  
**Mission Slug**: dockerized-aibbe-stack-isolated-chrome-01KPCPFB  
**Target Branch**: main  
**Created**: 2026-04-17

---

## Problem Statement

Google's bot-detection system (reCAPTCHA Enterprise) blocked the engineer's primary Google account after detecting that NotebookLM interactions were automated rather than human-initiated. The block affected both the NotebookLM session and the Gemini CLI due to shared IP and cookie state. The engineer cannot use aibbe against their primary account.

The system needs a way to run the full automation stack in a fully isolated browser environment linked to a separate Google account, without altering the CLI interface or the core communication protocol.

---

## Goals

- Enable the full aibbe request-response pipeline to operate from inside an isolated container environment using a fresh Google account.
- Allow the host CLI to communicate with the containerized daemon without any changes to CLI code or socket-path configuration.
- Persist the containerized Chrome session across container restarts so the engineer does not need to log in repeatedly.

## Non-Goals

- Humanization of input events (simulated keystrokes, mouse jitter) — separate task.
- Automated lifecycle management of the container (start, stop, restart) — manual responsibility.
- Automated creation or management of the Google account — engineer's responsibility.
- Multi-container orchestration or load balancing across accounts.
- Any network egress beyond the local Docker bridge to the host.

---

## User Scenarios & Testing

### Scenario 1 — Happy Path: Full pipeline from host CLI to NotebookLM

**Given** the container is running, Chrome is logged into the isolated Google account, the extension is installed, and the daemon is registered as a native host inside the container  
**When** the engineer issues a command from the host CLI  
**Then** the command travels through the volume-mounted socket to the containerized daemon, which forwards it to the extension, which interacts with NotebookLM and returns a response to the CLI

### Scenario 2 — Container not running

**Given** the container is stopped  
**When** the engineer issues a command from the host CLI  
**Then** the CLI fails immediately with a connection error (Fail-Fast, exit 1) — no hanging, no retry

### Scenario 3 — Chrome session expired inside container

**Given** the container is running but the Google session has expired (logged out)  
**When** the extension attempts to interact with NotebookLM  
**Then** the extension returns a structured authentication-failure error, which propagates through the daemon back to the CLI as a typed error (exit 1)

### Scenario 4 — Container restart with persistent session

**Given** the container is stopped and restarted  
**When** Chrome launches inside the container  
**Then** the Chrome profile (cookies, session tokens, extension state) is restored from the persistent volume and the engineer does not need to log in again

### Scenario 5 — Socket volume not mounted

**Given** the container is running but the Docker volume for the socket was not configured  
**When** the engineer issues a command from the host CLI  
**Then** the CLI fails immediately with a connection error (Fail-Fast, exit 1)

---

## Functional Requirements

| ID | Requirement | Status |
|---|---|---|
| FR-001 | The aibbe daemon must be deployable inside the container and must register as a native host for the Chrome instance running in that same container | Proposed |
| FR-002 | The daemon's Unix socket must be accessible from the host machine via a Docker volume mount, using the same socket path already configured in the CLI | Proposed |
| FR-003 | The Chrome extension must load and operate correctly when installed in the containerized Chrome instance | Proposed |
| FR-004 | The Chrome profile (session cookies, login state, extension data) must persist across container restarts via a dedicated persistent volume | Proposed |
| FR-005 | The complete request-response pipeline (CLI → Daemon → Extension → NotebookLM → response) must function end-to-end without any modifications to the existing CLI binary | Proposed |
| FR-006 | When the container is not reachable, the CLI must fail immediately with a typed error — no retries, no hanging | Proposed |
| FR-007 | When the Chrome session inside the container is expired, the extension must return a typed authentication-failure error that propagates to the CLI | Proposed |

---

## Non-Functional Requirements

| ID | Requirement | Threshold | Status |
|---|---|---|---|
| NFR-001 | The containerized daemon must respond to host CLI commands within the existing local latency budget | Under 2 seconds for the IPC round-trip | Proposed |
| NFR-002 | The Chrome session must survive container restarts without requiring re-authentication | 100% of restarts — no manual login if session was valid before stop | Proposed |
| NFR-003 | The setup procedure must be completable by the engineer in a single session | Under 30 minutes from zero to first successful CLI command | Proposed |

---

## Constraints

| ID | Constraint | Status |
|---|---|---|
| C-001 | The container image must be `linuxserver/chrome` | Accepted |
| C-002 | The Unix socket must remain the sole IPC channel between host CLI and daemon — no new network protocols introduced in this feature | Accepted |
| C-003 | Container lifecycle (start, stop, restart) is managed manually by the engineer — aibbe does not control it | Accepted |
| C-004 | Anti-detection / humanization of input events is out of scope | Accepted |
| C-005 | Operational cost must remain $0 | Accepted |
| C-006 | The isolated Google account is created and managed by the engineer — not by aibbe | Accepted |
| C-007 | The solution must not require changes to the host CLI binary or its socket-path configuration | Accepted |

---

## Success Criteria

1. A command issued from the host CLI reaches NotebookLM inside the container and returns a valid response, with zero changes to the CLI binary.
2. After a container stop and restart, the Chrome session is restored automatically — the engineer does not need to log in again.
3. When the container is unreachable, the CLI terminates within 2 seconds with exit code 1 — no indefinite block.

---

## Assumptions

- The engineer has Docker installed and operational on Debian 12.
- The `linuxserver/chrome` image supports loading unpacked Chrome extensions.
- The daemon binary can be compiled for the container's OS and architecture without modification to its source code.
- The container's Chrome profile path follows the standard Chromium convention for native host manifest registration.
- A single container instance is sufficient — no multi-account concurrency required.
- The engineer is capable of performing the initial manual setup (logging in, installing the extension, registering the daemon) inside the container's browser UI.

---

## Dependencies

- Existing aibbe daemon binary (compilable for the container's target platform)
- Existing Chrome extension (`extension/`)
- Docker runtime on the host (Debian 12)
- `linuxserver/chrome` Docker image
- A separate Google account (engineer-managed)
