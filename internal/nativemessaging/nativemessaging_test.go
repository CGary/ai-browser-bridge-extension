package nativemessaging

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

func TestWriteMessage_ValidPayload(t *testing.T) {
	payload := []byte(`{"cmd":"ping"}`)
	var buf bytes.Buffer

	if err := WriteMessage(&buf, payload); err != nil {
		t.Fatalf("WriteMessage returned error: %v", err)
	}

	if got, want := buf.Len(), 4+len(payload); got != want {
		t.Fatalf("buffer length = %d, want %d", got, want)
	}

	var gotLen uint32
	if err := binary.Read(&buf, binary.NativeEndian, &gotLen); err != nil {
		t.Fatalf("binary.Read length prefix: %v", err)
	}
	if got, want := gotLen, uint32(len(payload)); got != want {
		t.Fatalf("length prefix = %d, want %d", got, want)
	}
	if got := buf.Bytes(); !bytes.Equal(got, payload) {
		t.Fatalf("payload = %q, want %q", got, payload)
	}
}

func TestWriteMessage_EmptyPayload(t *testing.T) {
	var buf bytes.Buffer

	if err := WriteMessage(&buf, nil); err != nil {
		t.Fatalf("WriteMessage returned error: %v", err)
	}

	if got, want := buf.Len(), 4; got != want {
		t.Fatalf("buffer length = %d, want %d", got, want)
	}

	var gotLen uint32
	if err := binary.Read(&buf, binary.NativeEndian, &gotLen); err != nil {
		t.Fatalf("binary.Read length prefix: %v", err)
	}
	if gotLen != 0 {
		t.Fatalf("length prefix = %d, want 0", gotLen)
	}
	if got := buf.Len(); got != 0 {
		t.Fatalf("payload bytes remaining = %d, want 0", got)
	}
}

func TestWriteMessage_ExceedsLimit(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), MaxMessageSize+1)
	var buf bytes.Buffer

	err := WriteMessage(&buf, payload)
	if err == nil {
		t.Fatal("expected error for oversized payload, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds native messaging limit") {
		t.Fatalf("error = %q, want contains %q", err, "exceeds native messaging limit")
	}
	if got := buf.Len(); got != 0 {
		t.Fatalf("buffer length = %d, want 0", got)
	}
}

func TestWriteMessage_WriterError(t *testing.T) {
	wantErr := errors.New("boom")

	err := WriteMessage(errorWriter{err: wantErr}, []byte(`{"cmd":"ping"}`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "write length prefix") {
		t.Fatalf("error = %q, want contains %q", err, "write length prefix")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped error %v, got %v", wantErr, err)
	}
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write(_ []byte) (int, error) {
	return 0, w.err
}
