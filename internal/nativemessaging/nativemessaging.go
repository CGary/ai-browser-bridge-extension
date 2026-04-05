package nativemessaging

import (
	"encoding/binary"
	"fmt"
	"io"
)

// MaxMessageSize is the Chrome Native Messaging protocol limit (1 MB).
const MaxMessageSize = 1 << 20

// WriteMessage writes payload to w using the Native Messaging wire format:
// a 4-byte native-endian uint32 length prefix followed by the raw payload.
func WriteMessage(w io.Writer, payload []byte) error {
	if len(payload) > MaxMessageSize {
		return fmt.Errorf("payload exceeds native messaging limit: %d bytes", len(payload))
	}

	if err := binary.Write(w, binary.NativeEndian, uint32(len(payload))); err != nil {
		return fmt.Errorf("write length prefix: %w", err)
	}

	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}

	return nil
}
