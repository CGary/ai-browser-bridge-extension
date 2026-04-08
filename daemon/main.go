package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
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

	log.Printf("daemon listening on %s with mode 0600", socketPath)

	stopSignal := make(chan struct{})
	go func() {
		select {
		case <-stop:
		case <-stopSignal:
		}
		_ = l.Close()
	}()
	defer close(stopSignal)

	go stdinLoop(stdinReader, func(data []byte) {
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
	for {
		payload, err := nativemessaging.ReadMessage(r)
		if err != nil {
			closeResponseChannel()
			_, _ = fmt.Fprintln(os.Stderr, fatalProtocolMessage)
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
		log.Fatal(err)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	data, err := io.ReadAll(io.LimitReader(conn, ipc.MaxRequestSize+1))
	if err != nil {
		log.Printf("read error: %v", err)
		return
	}

	if len(data) > ipc.MaxRequestSize {
		log.Printf("request too large: %d bytes", len(data))
		return
	}

	var req ipc.Request
	if err := json.Unmarshal(data, &req); err != nil {
		log.Printf("invalid JSON: %v", err)
		return
	}

	if req.Cmd == "" {
		log.Printf("missing required field: cmd")
		return
	}

	log.Printf("received: cmd=%s payload=%s", req.Cmd, req.Payload)

	if err := nativemessaging.WriteMessage(nativeOut, data); err != nil {
		log.Printf("native messaging write: %v", err)
		return
	}

	if responseCh == nil {
		log.Printf("native messaging response channel not initialized")
		return
	}

	resp, ok := <-responseCh
	if !ok {
		log.Printf("native messaging transport closed while waiting for response")
		return
	}

	if _, err := conn.Write(resp); err != nil {
		log.Printf("write response error: %v", err)
	}
}
