package main

import (
	"bytes"
	"context"
	"net"
	"os/exec"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestCleanupSocket_FileNotExists verifies that cleanupSocket returns nil
// when the socket path does not exist (REQ-t1-04).
func TestCleanupSocket_FileNotExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.sock")

	err := cleanupSocket(path)
	if err != nil {
		t.Fatalf("expected nil error when file does not exist, got: %v", err)
	}
}

// TestCleanupSocket_FileExists verifies that cleanupSocket removes a
// pre-existing socket file and returns nil (REQ-t1-03).
func TestCleanupSocket_FileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "aibbe.sock")

	// Create a file to simulate a stale socket.
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("setup: could not create temp file: %v", err)
	}
	f.Close()

	if err := cleanupSocket(path); err != nil {
		t.Fatalf("expected nil error when file exists, got: %v", err)
	}

	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatal("expected file to be removed, but it still exists")
	}
}

// TestCleanupSocket_RemoveError verifies that cleanupSocket returns an error
// when os.Remove fails for a reason other than NotExist.
//
// Strategy: pass the path of a non-empty directory. On Linux, os.Remove on a
// non-empty directory returns ENOTEMPTY, which is not os.IsNotExist, so
// cleanupSocket must propagate the error.
func TestCleanupSocket_RemoveError(t *testing.T) {
	dir := t.TempDir()
	// Create a child file so the directory is non-empty.
	child := filepath.Join(dir, "child")
	if err := os.WriteFile(child, []byte("x"), 0o600); err != nil {
		t.Fatalf("setup: could not create child file: %v", err)
	}

	err := cleanupSocket(dir)
	if err == nil {
		t.Fatal("expected error when removing a non-empty directory, got nil")
	}
}

func TestDaemonStartup_RecreatesStaleSocketAndAcceptsConnections(t *testing.T) {
	requireUnixSocketSupport(t)
	cleanupFixedSocketPath(t)

	f, err := os.Create(socketPath)
	if err != nil {
		t.Fatalf("setup: could not create stale socket file: %v", err)
	}
	f.Close()

	cmd, stderr := startDaemonProcess(t)
	defer stopDaemonProcess(t, cmd)

	waitForDial(t, socketPath)

	if err := assertProcessAlive(cmd.Process.Pid); err != nil {
		t.Fatalf("expected daemon process to stay alive, got: %v; stderr=%s", err, stderr.String())
	}
}

func TestDaemonStartup_NoStaleSocket_StartsCleanly(t *testing.T) {
	requireUnixSocketSupport(t)
	cleanupFixedSocketPath(t)

	cmd, stderr := startDaemonProcess(t)
	defer stopDaemonProcess(t, cmd)

	waitForDial(t, socketPath)

	if strings.Contains(stderr.String(), "cleanup socket:") {
		t.Fatalf("expected no cleanup error in stderr, got: %s", stderr.String())
	}
}

func buildDaemonBinary(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "aibbe-daemon")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Env = append(
		os.Environ(),
		"CGO_ENABLED=0",
		"GOCACHE=/tmp/go-build",
		"GOMODCACHE=/tmp/go-mod-cache",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build daemon binary: %v\n%s", err, output)
	}
	return binary
}

func startDaemonProcess(t *testing.T) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()

	binary := buildDaemonBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	cmd := exec.CommandContext(ctx, binary)
	var stderr bytes.Buffer
	cmd.Stdout = &bytes.Buffer{}
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start daemon: %v", err)
	}

	return cmd, &stderr
}

func stopDaemonProcess(t *testing.T, cmd *exec.Cmd) {
	t.Helper()

	if cmd.Process == nil {
		return
	}

	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	cleanupFixedSocketPath(t)
}

func waitForDial(t *testing.T, path string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", path, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for daemon listener at %s", path)
}

func cleanupFixedSocketPath(t *testing.T) {
	t.Helper()
	_ = os.RemoveAll(socketPath)
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

func assertProcessAlive(pid int) error {
	return syscall.Kill(pid, 0)
}
