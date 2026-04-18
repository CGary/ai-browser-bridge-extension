---
work_package_id: WP01
title: 'Go Backend: IPC + CLI + Daemon'
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-target-based-library-routing-01KPGGYY
base_commit: 90a033cd27b551753d1e354eb5cc52e44b3e31b1
created_at: '2026-04-18T18:35:24.765694+00:00'
subtasks:
- T001
- T002
- T003
- T004
- T005
shell_pid: "384933"
agent: "gemini"
history:
- date: '2026-04-18'
  event: created
authoritative_surface: internal/ipc/
execution_mode: code_change
owned_files:
- internal/ipc/**
- cmd/cli/**
- daemon/main.go
tags: []
---

# WP01 — Go Backend: IPC + CLI + Daemon

**Mission**: `target-based-library-routing-01KPGGYY`  
**Branch strategy**: Plan on `main`, merge to `main`. Work in the lane worktree allocated by spec-kitty.  
**No dependencies** — starts immediately, parallelizable with WP02.

## Objective

Extend the Go IPC contract and CLI to support an optional `-target` flag. The flag carries the NotebookLM library name through the Unix socket to the daemon, which forwards it to the Chrome Extension without any logic changes. Backward compatibility must be 100% preserved for invocations that omit `-target`.

## Context

### Current state

**`internal/ipc/ipc.go`** — `Request` struct:
```go
type Request struct {
    Cmd     string `json:"cmd"`
    Payload string `json:"payload"`
}
```

**`cmd/cli/main.go`** — current flags:
```go
cmd     := flag.String("cmd", "", "command identifier (required)")
payload := flag.String("payload", "", "associated data (optional)")
flag.Parse()
// ...
data, err := json.Marshal(ipc.Request{Cmd: *cmd, Payload: *payload})
```

**`daemon/main.go`** — log line (approximate):
```go
fmt.Fprintf(os.Stderr, "[INFO] [Daemon] received: cmd=%s payload=%s\n", ...)
```

### Architecture note

The daemon forwards raw JSON bytes to the Chrome Extension without inspecting the `target` field (confirmed in `daemon/main.go` — it reads the request into bytes and passes them directly to `nativemessaging.WriteMessage`). **No daemon routing logic changes are needed.**

### Key constraints

- `omitempty` on `Target` is mandatory — without it, empty-string targets serialize as `"target":""` in JSON, breaking backward compatibility with the Extension's `message.target` check.
- The daemon log update is the only daemon change and is optional for correctness (but required for observability).

---

## Subtask T001 — Add `Target` field to `ipc.Request`

**Purpose**: Extend the IPC contract so the `target` field is serialized into JSON when non-empty and omitted when empty.

**File**: `internal/ipc/ipc.go`

**Change**:
```go
// Before
type Request struct {
    Cmd     string `json:"cmd"`
    Payload string `json:"payload"`
}

// After
type Request struct {
    Cmd     string `json:"cmd"`
    Target  string `json:"target,omitempty"`
    Payload string `json:"payload"`
}
```

**Field order**: `Cmd` → `Target` → `Payload`. Matches the conceptual read order (what → where → content).

**Validation**:
- [ ] Field added with correct JSON tag `"target,omitempty"`
- [ ] Existing code that creates `Request{Cmd: ..., Payload: ...}` still compiles without changes (zero-value `Target` is omitted)
- [ ] `go vet ./internal/ipc/` passes

---

## Subtask T002 — Update ipc package tests for `Target` field

**Purpose**: Verify that `Target` serializes correctly when set and is omitted when empty.

**File**: `internal/ipc/ipc_test.go` (or wherever the ipc package tests live)

**Run existing tests first** to confirm they pass before modifying anything:
```bash
go test ./internal/ipc/ -v
```

**Add two table-driven test cases** to the existing request serialization test (or create one if it doesn't exist):

```go
// Case 1: Target set — must appear in JSON
{
    name: "request with target",
    input: Request{Cmd: "generate", Target: "SIAT", Payload: "question"},
    wantJSON: `{"cmd":"generate","target":"SIAT","payload":"question"}`,
},
// Case 2: Target empty — must NOT appear in JSON (omitempty)
{
    name: "request without target",
    input: Request{Cmd: "generate", Payload: "question"},
    wantJSON: `{"cmd":"generate","payload":"question"}`,
},
```

Use `encoding/json.Marshal` + `string(data)` for comparison. Use `assert.JSONEq` if the project has testify, or `bytes.Equal(got, want)` with sorted keys if not.

**Validation**:
- [ ] Both new cases pass
- [ ] Existing cases still pass
- [ ] `go test ./internal/ipc/ -v` passes

---

## Subtask T003 — Add `-target` flag to CLI

**Purpose**: Expose the `target` parameter to users via a new `-target` flag; pass it into `ipc.Request`.

**File**: `cmd/cli/main.go`

**Change**:
```go
// Add flag (after payload line)
target  := flag.String("target", "", "target library name (optional, defaults to first free tab)")

// Update Request marshaling
data, err := json.Marshal(ipc.Request{
    Cmd:     *cmd,
    Target:  *target,   // ← ADD
    Payload: *payload,
})
```

**Notes**:
- Flag is optional (default `""`). When empty, `omitempty` ensures `target` is absent from JSON.
- No validation of `-target` in the CLI — it is the library name exactly as typed by the user.
- Do NOT require `-target` when `-cmd` is set — it must remain optional.

**Validation**:
- [ ] `-target "SIAT"` produces JSON with `"target":"SIAT"`
- [ ] Omitting `-target` produces JSON without `"target"` key
- [ ] `-cmd` still required; `-target` omission does not cause error
- [ ] `go vet ./cmd/cli/` passes

---

## Subtask T004 — Update CLI integration tests

**Purpose**: Verify that the CLI correctly passes `-target` through the socket and that omitting it preserves backward compatibility.

**File**: `cmd/cli/` test files (e.g., `cmd/cli/main_test.go` or similar)

**Run existing tests first**:
```bash
go test ./cmd/cli/ -v
```

Understand the existing test patterns — the CLI tests use `startMockDaemon()` and `buildCLIBinary()` helpers.

**Add two test cases** to the existing table-driven suite:

**Case 1 — `-target` present**:
```go
{
    name:        "with_target_flag",
    args:        []string{"-cmd", "generate", "-target", "SIAT", "-payload", "question"},
    wantReqJSON: `{"cmd":"generate","target":"SIAT","payload":"question"}`,
}
```

**Case 2 — `-target` absent (backward compat)**:
```go
{
    name:        "without_target_flag",
    args:        []string{"-cmd", "generate", "-payload", "question"},
    wantReqJSON: `{"cmd":"generate","payload":"question"}`,
    // "target" key MUST NOT appear in JSON
}
```

The mock daemon should capture the raw request bytes and assert the JSON. If the test infrastructure captures the request as a `Request` struct, assert `req.Target == "SIAT"` for Case 1 and `req.Target == ""` for Case 2.

**Validation**:
- [ ] Both new test cases pass
- [ ] Existing CLI test cases still pass
- [ ] `go test ./cmd/cli/ -v` passes

---

## Subtask T005 — Update daemon log line

**Purpose**: Include `target` in the daemon's INFO log when it is present, improving observability during debugging.

**File**: `daemon/main.go`

**Find the existing log line** (search for `[INFO] [Daemon] received`):
```go
// Current (approximate)
fmt.Fprintf(os.Stderr, "[INFO] [Daemon] received: cmd=%s payload=%s\n", req.Cmd, req.Payload)
```

**Update** to include `target` conditionally:
```go
if req.Target != "" {
    fmt.Fprintf(os.Stderr, "[INFO] [Daemon] received: cmd=%s target=%s payload=%s\n",
        req.Cmd, req.Target, req.Payload)
} else {
    fmt.Fprintf(os.Stderr, "[INFO] [Daemon] received: cmd=%s payload=%s\n",
        req.Cmd, req.Payload)
}
```

Or more concisely with a helper if the codebase uses one:
```go
targetStr := ""
if req.Target != "" {
    targetStr = fmt.Sprintf(" target=%s", req.Target)
}
fmt.Fprintf(os.Stderr, "[INFO] [Daemon] received: cmd=%s%s payload=%s\n",
    req.Cmd, targetStr, req.Payload)
```

**Note**: The daemon reads the request by deserializing JSON into `ipc.Request`. After T001, `req.Target` will be populated when the JSON includes `"target"`. This log change requires no struct changes — only the format string update.

**Validation**:
- [ ] Daemon log includes `target=SIAT` when `-target SIAT` is passed
- [ ] Daemon log format unchanged when no `-target` passed
- [ ] `go test ./daemon/ -v` passes (no daemon tests should be broken)
- [ ] `go vet ./daemon/` passes

---

## Branch Strategy

Planning base: `main`. Merge target: `main`.

Worktree for this WP is allocated by `spec-kitty next`. Do not manually create branches. When your work is complete, run:

```bash
spec-kitty agent action implement WP01 --agent <name>
```

## Definition of Done

- [ ] `go test ./...` passes with zero failures
- [ ] `go vet ./...` passes
- [ ] `ipc.Request` has `Target string \`json:"target,omitempty"\``
- [ ] CLI accepts `-target` flag; omission produces backward-compatible JSON
- [ ] Daemon log includes `target` when non-empty
- [ ] No changes to daemon routing logic

## Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Forgetting `omitempty` | High — breaks backward compat | Verify with test Case 2 (no-target JSON has no `"target"` key) |
| Daemon tests fail due to struct change | Low | Run `go test ./daemon/ -v` before committing |

## Reviewer Guidance

- Confirm `omitempty` is present in the struct tag — the absence is a silent correctness bug
- Confirm CLI tests cover both the with-target and without-target paths
- Confirm daemon routing code has no new logic (only log update)

## Activity Log

- 2026-04-18T18:35:25Z – gemini – shell_pid=384933 – Assigned agent via action command
