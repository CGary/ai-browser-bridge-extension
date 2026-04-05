package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

func TestSocketPermissions_Is0600(t *testing.T) {
	requireUnixSocketSupport(t)

	path := filepath.Join(t.TempDir(), "secure.sock")
	l, err := listenSecure("unix", path)
	if err != nil {
		t.Fatalf("listenSecure: %v", err)
	}
	defer l.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}

	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected socket permissions 0600, got %04o", got)
	}
}

func TestSocketOwnership_MatchesDaemonUID(t *testing.T) {
	requireUnixSocketSupport(t)
	cleanupFixedSocketPath(t)

	cmd, _ := startDaemonProcess(t)
	defer stopDaemonProcess(t, cmd)

	waitForDial(t, socketPath)

	info, err := os.Stat(socketPath)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("could not get syscall.Stat_t from FileInfo")
	}

	expectedUID := uint32(os.Getuid())
	if stat.Uid != expectedUID {
		t.Fatalf("expected socket UID %d, got %d", expectedUID, stat.Uid)
	}
}

func TestSocketRejectsDifferentUID_EACCES(t *testing.T) {
	requireUnixSocketSupport(t)
	uidStart, gidStart := requireCrossUIDProbeSupport(t)
	cleanupFixedSocketPath(t)

	cmd, _ := startDaemonProcess(t)
	defer stopDaemonProcess(t, cmd)

	waitForDial(t, socketPath)

	helperBinary := copyExecutableForCrossUIDProbe(t)
	probe := exec.Command(
		"unshare", "--user",
		"--map-users", fmt.Sprintf("0:%d:1", uidStart),
		"--map-groups", fmt.Sprintf("0:%d:1", gidStart),
		"--setuid", "0",
		"--setgid", "0",
		helperBinary,
		"-test.run=TestHelperDialSocketAsDifferentUID",
	)
	probe.Env = append(
		os.Environ(),
		"GO_WANT_HELPER_PROCESS=1",
		"AIBBE_SOCKET_PATH="+socketPath,
	)

	output, err := probe.CombinedOutput()
	if err != nil {
		t.Fatalf("expected different-UID dial to fail with EACCES, probe failed: %v\n%s", err, output)
	}
}

func TestHelperDialSocketAsDifferentUID(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	path := os.Getenv("AIBBE_SOCKET_PATH")
	if path == "" {
		fmt.Fprintln(os.Stderr, "missing AIBBE_SOCKET_PATH")
		os.Exit(2)
	}

	conn, err := net.DialTimeout("unix", path, 200*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		fmt.Fprintln(os.Stderr, "expected permission denied, got successful dial")
		os.Exit(1)
	}

	if !errors.Is(err, syscall.EACCES) && !strings.Contains(strings.ToLower(err.Error()), "permission denied") {
		fmt.Fprintf(os.Stderr, "expected EACCES, got %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
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

func requireCrossUIDProbeSupport(t *testing.T) (int, int) {
	t.Helper()

	if _, err := exec.LookPath("unshare"); err != nil {
		t.Skipf("cross-UID probe requires unshare: %v", err)
	}

	username := os.Getenv("USER")
	if username == "" {
		t.Skip("cross-UID probe requires USER environment variable")
	}

	uidStart, ok := lookupSubordinateIDStart("/etc/subuid", username)
	if !ok {
		t.Skipf("cross-UID probe requires subordinate UID range for %s", username)
	}

	gidStart, ok := lookupSubordinateIDStart("/etc/subgid", username)
	if !ok {
		t.Skipf("cross-UID probe requires subordinate GID range for %s", username)
	}

	probe := exec.Command(
		"unshare", "--user",
		"--map-users", fmt.Sprintf("0:%d:1", uidStart),
		"--map-groups", fmt.Sprintf("0:%d:1", gidStart),
		"--setuid", "0",
		"--setgid", "0",
		"true",
	)
	if output, err := probe.CombinedOutput(); err != nil {
		t.Skipf("cross-UID probe unsupported in this environment: %v\n%s", err, output)
	}

	return uidStart, gidStart
}

func lookupSubordinateIDStart(path, username string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}

	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) < 3 || parts[0] != username {
			continue
		}

		start, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, false
		}
		return start, true
	}

	return 0, false
}

func copyExecutableForCrossUIDProbe(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("/tmp", "aibbe-crossuid-")
	if err != nil {
		t.Fatalf("create cross-UID helper dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("chmod helper dir: %v", err)
	}

	src, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatalf("open current test binary: %v", err)
	}
	defer src.Close()

	dstPath := filepath.Join(dir, "daemon.test")
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatalf("create helper test binary: %v", err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		t.Fatalf("copy helper test binary: %v", err)
	}
	if err := dst.Close(); err != nil {
		t.Fatalf("close helper test binary: %v", err)
	}

	if err := os.Chmod(dstPath, 0o755); err != nil {
		t.Fatalf("chmod helper test binary: %v", err)
	}

	return dstPath
}

func assertProcessAlive(pid int) error {
	return syscall.Kill(pid, 0)
}
