package ipc

import "os"

const SocketPath = "/tmp/aibbe.sock"
const SocketPathEnvVar = "AIBBE_SOCKET_PATH"

// MaxRequestSize is the maximum allowed request payload size in bytes.
const MaxRequestSize = 1 << 20

// Request represents a CLI-to-daemon IPC request.
type Request struct {
	Cmd     string `json:"cmd"`
	Payload string `json:"payload"`
}

// Response represents a daemon-to-CLI IPC response.
type Response struct {
	Status string `json:"status"`
}

// SocketPathForProcess returns the configured socket path for the current
// process, defaulting to the production path when no override is set.
func SocketPathForProcess() string {
	if path := os.Getenv(SocketPathEnvVar); path != "" {
		return path
	}
	return SocketPath
}
