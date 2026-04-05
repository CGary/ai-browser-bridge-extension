package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"syscall"
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

// listenSecure creates a Unix listener with 0600 permissions by applying
// a restrictive umask during socket creation.
func listenSecure(network, address string) (net.Listener, error) {
	prevMask := syscall.Umask(0o177)
	l, err := net.Listen(network, address)
	syscall.Umask(prevMask)
	return l, err
}

func main() {
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
		conn.Close()
	}
}
