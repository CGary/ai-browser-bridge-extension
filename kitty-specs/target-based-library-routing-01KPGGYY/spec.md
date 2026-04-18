# Target-based Library Routing

## Overview

Allow users to direct CLI queries to a **specific NotebookLM library** by name when multiple library tabs are open simultaneously. Without this capability, the routing logic arbitrarily picks the first available tab, making it impossible to reliably query a specific library from the terminal.

## Actors

- **CLI User**: Person invoking `aibbe-cli` from the terminal with an optional `-target` flag.
- **Daemon**: Resident process that relays messages between CLI and Chrome Extension transparently.
- **Background Script**: Routes incoming requests to the correct NotebookLM tab based on its registry.
- **Content Script**: Reads the active library title from the NotebookLM DOM and registers the tab identity with the Background Script.

## Problem Statement

When a user has multiple NotebookLM libraries open (e.g., "SIAT", "PF", "React"), the current `findFreeTab` routing takes the first available tab without considering which library it displays. There is no way to target a specific library from the CLI.

## Proposed Solution

Add a `-target` flag to the CLI carrying the library name. The Content Script reads the active library title from the NotebookLM DOM (`div.cover-title`) once Angular renders it, then includes it in the HANDSHAKE message sent to the Background Script. The Background Script stores the target per tab in its registry and uses it to route requests only to the tab displaying the requested library.

## User Scenarios & Testing

### Primary Flow — Targeted Query

1. User opens NotebookLM with "SIAT" library in Tab A and "PF" library in Tab B.
2. Content Script in each tab waits for Angular to render `div.cover-title`, then sends HANDSHAKE with the library name to the Background Script.
3. Background Script registers each tab with its corresponding library name (`tabId → { state, service, target }`).
4. User executes: `aibbe-cli -cmd "generate" -target "SIAT" -payload "¿qué es el SIAT?"`
5. CLI sends IPC request with `target: "SIAT"` over Unix socket to the Daemon.
6. Daemon forwards the JSON payload to the Extension without modification.
7. Background Script finds Tab A (state: free, target: "SIAT") and routes the request there.
8. Content Script in Tab A processes the query and returns the response.
9. User receives the response scoped to the "SIAT" library.

### Edge Case — No Target Specified

1. User executes: `aibbe-cli -cmd "generate" -payload "general question"` (no `-target`).
2. Request carries no `target` field.
3. Background Script falls back to current behavior: first free tab regardless of library.

### Edge Case — Target Not Found

1. User executes: `aibbe-cli -cmd "generate" -target "Nonexistent" -payload "..."`
2. Background Script finds no free tab with a matching library name.
3. Extension returns: `{ "status": "error", "error": "target_not_found" }`.
4. CLI exits with code 1 and prints the error to stderr.

### Edge Case — SPA Navigation (Stale Target)

1. User navigates from "SIAT" to "PF" in the same tab without reloading.
2. MutationObserver in Content Script detects the title change in `div.cover-title`.
3. Content Script re-sends HANDSHAKE with updated `target: "PF"`.
4. Background Script updates the registry entry for that tab.
5. Subsequent requests to "PF" are correctly routed to this tab.

### Edge Case — Multiple Tabs with Same Target

1. Two tabs both display the "SIAT" library.
2. User queries with `target: "SIAT"`.
3. Background Script picks the first free tab among all tabs matching that target.

### Edge Case — Target Not Yet Rendered (HANDSHAKE Timing)

1. Content Script loads synchronously before Angular renders the library title.
2. MutationObserver watches for `div.cover-title` to appear and populate.
3. Once the element is ready, Content Script reads the title and sends HANDSHAKE.
4. No HANDSHAKE is sent with an empty or undefined target.

## Functional Requirements

| ID | Requirement | Status |
|----|-------------|--------|
| FR-001 | The CLI MUST accept an optional `-target` flag that takes a library name string | Proposed |
| FR-002 | When `-target` is omitted, the system MUST behave identically to current routing (first free tab) | Proposed |
| FR-003 | The IPC `Request` struct MUST include an optional `target` field serialized as `"target"` in JSON (omitted when empty) | Proposed |
| FR-004 | The Daemon MUST forward the complete IPC JSON payload to the Extension without modifying or inspecting the `target` field | Proposed |
| FR-005 | The Content Script MUST read the active library name from the text content of `div.cover-title` in the NotebookLM DOM | Proposed |
| FR-006 | The Content Script MUST use a MutationObserver to detect when `div.cover-title` is available and populated before sending the HANDSHAKE | Proposed |
| FR-007 | The Content Script MUST re-send a HANDSHAKE with the updated library name when MutationObserver detects a change to `div.cover-title` content (SPA navigation) | Proposed |
| FR-008 | The HANDSHAKE message MUST include a `target` field with the library name read from the DOM | Proposed |
| FR-009 | The Background Script tab registry MUST store the `target` (library name) alongside `state` and `service` for each registered tab | Proposed |
| FR-010 | When `request.target` is non-empty, the Background Script routing MUST select only a tab whose registry entry has a matching `target` value and `state: "free"` | Proposed |
| FR-011 | When `request.target` is set and no matching free tab exists, the Extension MUST return `{ "status": "error", "error": "target_not_found" }` | Proposed |
| FR-012 | When multiple free tabs match the same target, the system MUST route to the first matching tab in registry insertion order | Proposed |

## Non-Functional Requirements

| ID | Requirement | Threshold | Status |
|----|-------------|-----------|--------|
| NFR-001 | HANDSHAKE must be sent within 5 seconds of the library title becoming available in the DOM | ≤ 5 s from DOM readiness | Proposed |
| NFR-002 | The `-target` flag must not break existing CLI invocations that omit it | Zero regressions on existing test suite | Proposed |
| NFR-003 | Target matching is exact string equality; no fuzzy, partial, or case-insensitive matching | 100% exact-match accuracy | Proposed |

## Constraints

| ID | Constraint | Status |
|----|------------|--------|
| C-001 | The Daemon requires no routing logic changes; it forwards JSON payloads transparently | Proposed |
| C-002 | The DOM selector for the library title is `div.cover-title` (Angular class `mat-headline-medium`) in the NotebookLM UI | Proposed |
| C-003 | No disk persistence is allowed; the target registry lives exclusively in Background Script RAM | Proposed |
| C-004 | IPC message size limit remains 1 MB | Proposed |
| C-005 | Native Messaging message size limit remains 1 MB | Proposed |
| C-006 | Target matching is exact string equality only | Proposed |

## Success Criteria

1. A user with multiple NotebookLM libraries open can direct a query to any specific library by exact name and receive a response from that library with 100% routing accuracy.
2. CLI invocations without `-target` continue to work without any behavior change or test regressions.
3. When the requested library is not currently open (or all matching tabs are busy), the user receives a structured error in the same response cycle.
4. When a user navigates to a different library within the same tab, subsequent queries using the new library name are correctly routed within one request cycle after navigation.
5. No HANDSHAKE is ever sent with an undefined or empty `target` due to Angular timing — the Content Script always waits for DOM readiness.

## Key Entities

| Entity | Description |
|--------|-------------|
| `Request` | IPC message struct: `Cmd string`, `Target string (omitempty)`, `Payload string` |
| `TabRegistry` | In-memory Background Script map: `tabId → { state, service, target }` |
| `HANDSHAKE` | Content Script → Background Script message: `{ type, tabId, service, target }` |
| Library Title | Trimmed text content of `div.cover-title` in the NotebookLM Angular DOM |

## Assumptions

1. Library titles are unique enough within a user's open tabs to serve as routing identifiers; the user is responsible for keeping library names unambiguous.
2. The `div.cover-title` element renders within 5 seconds of Angular initializing for the NotebookLM library view.
3. The `-target` value must match the library name exactly as shown in the NotebookLM UI (case-sensitive, trimmed whitespace).
4. The Chromium extension ID and native host manifest remain unchanged.

## Out of Scope

- Fuzzy or case-insensitive target matching
- Persisting the tab registry to `chrome.storage.*`
- Routing by tab URL, tab index, or any identifier other than library name
- Multiple simultaneous in-flight requests
- Daemon-level target validation or awareness
