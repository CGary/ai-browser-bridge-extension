package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"syscall"

	"aibbe/internal/ipc"
	"aibbe/internal/nativemessaging"
)

// nativeOut is the writer for Native Messaging output.
// Production uses os.Stdout; tests override it with a buffer.
var nativeOut io.Writer = os.Stdout

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

func main() {
	socketPath := ipc.SocketPathForProcess()

	if err := cleanupSocket(socketPath); err != nil {
		log.Fatalf("cleanup socket: %v", err)
	}

	l, err := listenSecure("unix", socketPath)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer l.Close()

	log.Printf("daemon listening on %s with mode 0600", socketPath)

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		handleConnection(conn)
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

	resp, err := json.Marshal(ipc.Response{Status: "ok"})
	if err != nil {
		log.Printf("marshal response error: %v", err)
		return
	}

	if _, err := conn.Write(resp); err != nil {
		log.Printf("write response error: %v", err)
	}
}
