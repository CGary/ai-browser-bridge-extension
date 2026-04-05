package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"

	"aibbe/internal/ipc"
)

func main() {
	socketPath := ipc.SocketPathForProcess()
	cmd := flag.String("cmd", "", "command identifier (required)")
	payload := flag.String("payload", "", "associated data (optional)")
	flag.Parse()

	if *cmd == "" {
		exitWithError("error: -cmd flag is required")
	}

	data, err := json.Marshal(ipc.Request{Cmd: *cmd, Payload: *payload})
	if err != nil {
		exitWithError(fmt.Sprintf("error: failed to encode request: %v", err))
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		exitWithError(fmt.Sprintf("error: could not connect to daemon at %s: %v", socketPath, err))
	}
	defer conn.Close()

	if _, err := conn.Write(data); err != nil {
		exitWithError(fmt.Sprintf("error: failed to send request: %v", err))
	}

	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		exitWithError(fmt.Sprintf("error: unexpected connection type %T", conn))
	}

	if err := unixConn.CloseWrite(); err != nil {
		exitWithError(fmt.Sprintf("error: failed to send request: %v", err))
	}

	response, err := io.ReadAll(conn)
	if err != nil {
		exitWithError(fmt.Sprintf("error: failed to read response: %v", err))
	}

	if _, err := os.Stdout.Write(response); err != nil {
		exitWithError(fmt.Sprintf("error: failed to write response: %v", err))
	}
}

func exitWithError(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
