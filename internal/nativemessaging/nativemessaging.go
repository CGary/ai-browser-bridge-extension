package nativemessaging

import (
	"encoding/binary"
	"fmt"
	"io"
)

// MaxMessageSize is the Chrome Native Messaging protocol limit (1 MB).
const MaxMessageSize = 1 << 20

// WriteMessage writes payload to w using the Native Messaging wire format:
// a 4-byte little-endian uint32 length prefix followed by the raw payload.
func WriteMessage(w io.Writer, payload []byte) error {
	if len(payload) > MaxMessageSize {
		return fmt.Errorf("payload exceeds native messaging limit: %d bytes", len(payload))
	}

	if err := binary.Write(w, binary.LittleEndian, uint32(len(payload))); err != nil {
		return fmt.Errorf("write length prefix: %w", err)
	}

	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}

	return nil
}

// ReadMessage reads a Native Messaging payload from r using the wire format:
// a 4-byte little-endian uint32 length prefix followed by the raw payload.
func ReadMessage(r io.Reader) ([]byte, error) {
	var payloadLen uint32
	if err := binary.Read(r, binary.LittleEndian, &payloadLen); err != nil {
		return nil, fmt.Errorf("read length prefix: %w", err)
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}

	return payload, nil
}
