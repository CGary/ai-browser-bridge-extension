package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aibbe/internal/ipc"
)

func TestCLI_MissingCmdFlag_ExitsWithError(t *testing.T) {
	stdout, stderr, err := runCLI(t, tempSocketPath(t), nil)
	if err == nil {
		t.Fatal("expected CLI to fail without -cmd")
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}

	if !strings.Contains(stderr, "error: -cmd flag is required") {
		t.Fatalf("expected missing flag error, got %q", stderr)
	}
}

func TestCLI_DaemonNotRunning_ExitsWithError(t *testing.T) {
	socketPath := tempSocketPath(t)

	stdout, stderr, err := runCLI(t, socketPath, nil, "-cmd", "ping")
	if err == nil {
		t.Fatal("expected CLI to fail when daemon is unavailable")
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}

	if !strings.Contains(stderr, "error: could not connect to daemon at "+socketPath) {
		t.Fatalf("expected connection failure, got %q", stderr)
	}
}

func TestCLI_ValidRequest_PrintsResponse(t *testing.T) {
	requireUnixSocketSupport(t)
	socketPath := tempSocketPath(t)

	requests, stop := startMockDaemon(t, socketPath, []byte(`{"status":"ok"}`))
	defer stop()

	stdout, stderr, err := runCLI(t, socketPath, nil, "-cmd", "test", "-payload", "hello")
	if err != nil {
		t.Fatalf("expected CLI success, got err=%v stderr=%q", err, stderr)
	}

	if strings.TrimSpace(stdout) != `{"status":"ok"}` {
		t.Fatalf("expected ACK on stdout, got %q", stdout)
	}

	req := <-requests
	if req.Cmd != "test" || req.Payload != "hello" {
		t.Fatalf("unexpected request: %+v", req)
	}
}

func TestCLI_EmptyPayload_SendsEmptyString(t *testing.T) {
	requireUnixSocketSupport(t)
	socketPath := tempSocketPath(t)

	requests, stop := startMockDaemon(t, socketPath, []byte(`{"status":"ok"}`))
	defer stop()

	stdout, stderr, err := runCLI(t, socketPath, nil, "-cmd", "test")
	if err != nil {
		t.Fatalf("expected CLI success, got err=%v stderr=%q", err, stderr)
	}

	if strings.TrimSpace(stdout) != `{"status":"ok"}` {
		t.Fatalf("expected ACK on stdout, got %q", stdout)
	}

	req := <-requests
	if req.Payload != "" {
		t.Fatalf("expected empty payload, got %q", req.Payload)
	}
}

func TestCLI_ValidRequest_ExitCodeZero(t *testing.T) {
	requireUnixSocketSupport(t)
	socketPath := tempSocketPath(t)

	_, stop := startMockDaemon(t, socketPath, []byte(`{"status":"ok"}`))
	defer stop()

	_, _, exitCode, err := runCLIWithExitCode(t, socketPath, nil, "-cmd", "ping")
	if err != nil {
		t.Fatalf("expected CLI success, got err=%v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
}

func TestCLI_DaemonNotRunning_ExitCodeOne(t *testing.T) {
	socketPath := tempSocketPath(t)

	_, _, exitCode, err := runCLIWithExitCode(t, socketPath, nil, "-cmd", "ping")
	if err == nil {
		t.Fatal("expected CLI to fail when daemon is unavailable")
	}
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
}

func runCLI(t *testing.T, socketPath string, extraEnv []string, args ...string) (string, string, error) {
	t.Helper()

	stdout, stderr, _, err := runCLIWithExitCode(t, socketPath, extraEnv, args...)
	return stdout, stderr, err
}

func runCLIWithExitCode(t *testing.T, socketPath string, extraEnv []string, args ...string) (string, string, int, error) {
	t.Helper()

	binary := buildCLIBinary(t)
	cmd := exec.Command(binary, args...)
	cmd.Env = append(
		os.Environ(),
		"CGO_ENABLED=0",
		"GOCACHE=/tmp/go-build",
		"GOMODCACHE=/tmp/go-mod-cache",
		ipc.SocketPathEnvVar+"="+socketPath,
	)
	cmd.Env = append(cmd.Env, extraEnv...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return stdout.String(), stderr.String(), exitCode, err
}

func buildCLIBinary(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "aibbe-cli")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Env = append(
		os.Environ(),
		"CGO_ENABLED=0",
		"GOCACHE=/tmp/go-build",
		"GOMODCACHE=/tmp/go-mod-cache",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build cli binary: %v\n%s", err, output)
	}

	return binary
}

func startMockDaemon(t *testing.T, socketPath string, response []byte) (<-chan ipc.Request, func()) {
	t.Helper()

	l, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen mock daemon: %v", err)
	}

	requests := make(chan ipc.Request, 1)
	done := make(chan struct{})

	go func() {
		defer close(done)

		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		data, err := ioReadAllWithDeadline(conn)
		if err != nil {
			return
		}

		var req ipc.Request
		if err := json.Unmarshal(data, &req); err != nil {
			return
		}
		requests <- req

		_, _ = conn.Write(response)
	}()

	return requests, func() {
		_ = l.Close()
		<-done
		_ = os.Remove(socketPath)
	}
}

func ioReadAllWithDeadline(conn net.Conn) ([]byte, error) {
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	defer conn.SetReadDeadline(time.Time{})

	return io.ReadAll(conn)
}

func cleanupSocketPath(t *testing.T) {
	t.Helper()
	_ = os.RemoveAll(ipc.SocketPath)
}

func tempSocketPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "aibbe.sock")
}

func requireUnixSocketSupport(t *testing.T) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "probe.sock")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Skipf("unix sockets not supported in this environment: %v", err)
	}
	_ = l.Close()
	_ = os.Remove(path)
}
