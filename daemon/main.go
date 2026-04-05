package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

const socketPath = "/tmp/aibbe.sock"

// cleanupSocket removes the socket file at socketPath if it exists.
// Returns nil if the file does not exist (os.IsNotExist).
// Returns an error if removal fails for any other reason.
func cleanupSocket(socketPath string) error {
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove stale socket: %w", err)
	}
	return nil
}

func main() {
	if err := cleanupSocket(socketPath); err != nil {
		log.Fatalf("cleanup socket: %v", err)
	}

	l, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer l.Close()

	log.Printf("daemon listening on %s", socketPath)

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		conn.Close()
	}
}
