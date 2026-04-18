package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"syscall"

	"aibbe/internal/ipc"
	"aibbe/internal/nativemessaging"
)

// nativeOut is the writer for Native Messaging output.
// Production uses os.Stdout; tests override it with a buffer.
var nativeOut io.Writer = os.Stdout

// stdinReader is the reader for Native Messaging input.
// Production uses os.Stdin; tests override it with in-memory readers.
var stdinReader io.Reader = os.Stdin

// exitFunc terminates the process on fatal protocol desynchronization.
// Tests override it to capture the exit code without terminating the test process.
var exitFunc = os.Exit

// responseCh is the single in-flight handoff channel between Native Messaging
// stdin and the active IPC handler.
var responseCh chan []byte

var responseChCloseOnce sync.Once

const fatalProtocolMessage = "[FATAL] [Daemon] Desincronización de protocolo Native Messaging"

func initResponseChannel() chan []byte {
	responseCh = make(chan []byte)
	responseChCloseOnce = sync.Once{}
	return responseCh
}

func closeResponseChannel() {
	if responseCh == nil {
		return
	}

	responseChCloseOnce.Do(func() {
		close(responseCh)
	})
}

// cleanupSocket removes the socket file at socketPath if it exists.
// Returns nil if the file does not exist (os.IsNotExist).
// Returns an error if removal fails for any other reason.
func cleanupSocket(socketPath string) error {
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove stale socket: %w", err)
	}
	return nil
}

// listenSecure creates a Unix listener with 0600 permissions by applying
// a restrictive umask during socket creation.
func listenSecure(network, address string) (net.Listener, error) {
	prevMask := syscall.Umask(0o177)
	l, err := net.Listen(network, address)
	syscall.Umask(prevMask)
	return l, err
}

func run(socketPath string, stop <-chan struct{}) error {
	if err := cleanupSocket(socketPath); err != nil {
		return fmt.Errorf("cleanup socket: %w", err)
	}

	l, err := listenSecure("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer l.Close()

	initResponseChannel()

	fmt.Fprintf(os.Stderr, "[INFO] [Daemon] daemon listening on %s with mode 0600\n", socketPath)

	stopSignal := make(chan struct{})
	go func() {
		select {
		case <-stop:
		case <-stopSignal:
		}
		_ = l.Close()
	}()
	defer close(stopSignal)

	go stdinLoopUntilStop(stdinReader, stop, func(data []byte) {
		responseCh <- data
	})

	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-stop:
				return nil
			default:
			}
			return fmt.Errorf("accept: %w", err)
		}
		handleConnection(conn)
	}
}

func stdinLoop(r io.Reader, onMessage func([]byte)) {
	stdinLoopUntilStop(r, nil, onMessage)
}

func stdinLoopUntilStop(r io.Reader, stop <-chan struct{}, onMessage func([]byte)) {
	for {
		payload, err := nativemessaging.ReadMessage(r)
		if err != nil {
			closeResponseChannel()
			if stop != nil {
				select {
				case <-stop:
					return
				default:
				}
			}
			fmt.Fprintln(os.Stderr, fatalProtocolMessage)
			exitFunc(1)
			return
		}

		onMessage(payload)
	}
}

func main() {
	socketPath := ipc.SocketPathForProcess()
	stop := make(chan struct{})
	if err := run(socketPath, stop); err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] [Daemon] %v\n", err)
		os.Exit(1)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	data, err := io.ReadAll(io.LimitReader(conn, ipc.MaxRequestSize+1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] [Daemon] read error: %v\n", err)
		return
	}

	if len(data) == 0 {
		return
	}

	if len(data) > ipc.MaxRequestSize {
		fmt.Fprintf(os.Stderr, "[ERROR] [Daemon] request too large: %d bytes\n", len(data))
		return
	}

	var req ipc.Request
	if err := json.Unmarshal(data, &req); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] [Daemon] invalid JSON: %v\n", err)
		return
	}

	if req.Cmd == "" {
		fmt.Fprintf(os.Stderr, "[ERROR] [Daemon] missing required field: cmd\n")
		return
	}

	targetStr := ""
	if req.Target != "" {
		targetStr = fmt.Sprintf(" target=%s", req.Target)
	}
	fmt.Fprintf(os.Stderr, "[INFO] [Daemon] received: cmd=%s%s payload=%s\n", req.Cmd, targetStr, req.Payload)

	if err := nativemessaging.WriteMessage(nativeOut, data); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] [Daemon] native messaging write: %v\n", err)
		return
	}

	if responseCh == nil {
		fmt.Fprintf(os.Stderr, "[ERROR] [Daemon] native messaging response channel not initialized\n")
		return
	}

	resp, ok := <-responseCh
	if !ok {
		fmt.Fprintf(os.Stderr, "[ERROR] [Daemon] native messaging transport closed while waiting for response\n")
		return
	}

	if _, err := conn.Write(resp); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] [Daemon] write response error: %v\n", err)
	}
}
