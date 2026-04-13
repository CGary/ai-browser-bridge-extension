# Technical Design: t21-validacion-contrato-salida-cli

This document outlines the technical approach to formalize and validate the CLI output contract, ensuring byte-perfect purity for `stdout`, structured telemetry in `stderr`, and standardized exit codes.

## Architecture Decisions (ADRs)

### ADR-01: Extension Errors as Pass-through (Exit Code 0)
- **Decision**: Logical errors originating from the Chrome Extension (e.g., `no_free_tabs`, `timeout`) will be treated as successful executions of the bridge.
- **Rationale**: The CLI's responsibility is to deliver the message and return the response. A valid error payload from the extension is a successful delivery. This allows downstream consumers (pipes) to parse the JSON error and decide how to handle it.
- **Impact**: CLI returns Exit Code 0 for extension errors. Exit Code 1 is reserved for system/bridge failures (connection lost, daemon down).

### ADR-02: Stderr for Telemetry and Formatting
- **Decision**: `stderr` will be the exclusive channel for telemetry and errors. Logs must follow the `[LEVEL] [COMPONENT] Message` format.
- **Rationale**: Keeps `stdout` "pure" for programmatic consumption (e.g., `aibbe -cmd ping | jq .`).
- **Constraint**: `stdout` MUST NOT contain any log prefixes or level indicators.

## Data Structures
No new data structures are introduced. The design relies on existing contracts:
- `ipc.Request`: Command and payload sent to the daemon.
- `extensionResponse`: JSON structure returned by the extension/daemon.

## Implementation Details

### CLI Output Purity
- In `cmd/cli/main.go`, the response from the daemon must be written to `os.Stdout` using `fmt.Print` or `os.Stdout.Write` without adding extra newlines (`\n`).
- Implement ANSI stripping or ensure no ANSI sequences are ever injected into the `stdout` stream.

### Exit Code Logic
```go
// Pseudo-code for CLI main loop
resp, err := client.Send(req)
if err != nil {
    fmt.Fprintln(os.Stderr, "error:", err)
    os.Exit(1)
}
fmt.Print(string(resp)) // Pure output
os.Exit(0)             // Success (includes extension errors)
```

## Testing Strategy

### 1. Mock Daemon Tests (`cmd/cli/main_test.go`)
- **Purity Check**: Use `startMockDaemon` to send raw bytes `{"status":"ok"}`. Verify `stdout` using `bytes.Equal` to ensure no trailing `\n`.
- **ANSI Safety**: Inject `\033[31m{"status":"ok"}\033[0m` from the mock and verify `stdout` is stripped of ANSI or fails the test if contamination is found.
- **System Failure**: Simulate a closed socket and verify Exit Code 1 and error message in `stderr`.

### 2. E2E Tests (`daemon/main_test.go`)
- **NMStub Integration**: Use `NewNMStub` with `WithErrorResponse("no_free_tabs")`.
- **Validation**:
    - `stdout`: Must match the error JSON exactly.
    - `exitCode`: Must be 0.
    - `stderr`: Use `regexp.MatchString(`^\[[A-Z]+\]`)` to validate log format if `stderr` is not empty.
- **Contamination Check**: Verify that `stdout` does not contain any substrings like `[INFO]` or `[ERROR]`.

## Migration / Compatibility
- Scripts relying on CLI Exit Code 1 to detect extension errors will need to be updated to parse the JSON `status` field. This aligns with standard API bridge behaviors.
- Byte-perfect `stdout` improves compatibility with `jq` and other JSON processors.
