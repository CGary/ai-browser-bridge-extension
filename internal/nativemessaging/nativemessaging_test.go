package nativemessaging

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
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
	if err := binary.Read(&buf, binary.LittleEndian, &gotLen); err != nil {
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
	if err := binary.Read(&buf, binary.LittleEndian, &gotLen); err != nil {
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

func TestReadMessage(t *testing.T) {
	tests := []struct {
		name        string
		reader      io.Reader
		wantPayload []byte
		wantErr     error
		wantErrText string
	}{
		{
			name:        "valid message",
			reader:      bytes.NewReader(mustWireMessage(t, []byte(`{"cmd":"ping"}`))),
			wantPayload: []byte(`{"cmd":"ping"}`),
		},
		{
			name:        "empty payload",
			reader:      bytes.NewReader(mustWireMessage(t, nil)),
			wantPayload: []byte{},
		},
		{
			name:        "unexpected EOF on prefix",
			reader:      bytes.NewReader([]byte{0x05, 0x00}),
			wantErr:     io.ErrUnexpectedEOF,
			wantErrText: "read length prefix",
		},
		{
			name:        "unexpected EOF on payload",
			reader:      bytes.NewReader(append(mustLengthPrefix(5), []byte("hi")...)),
			wantErr:     io.ErrUnexpectedEOF,
			wantErrText: "read payload",
		},
		{
			name:        "short reads are assembled correctly",
			reader:      &chunkedReader{data: mustWireMessage(t, []byte("hello")), chunkSize: 1},
			wantPayload: []byte("hello"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := ReadMessage(tt.reader)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want wrapped %v", err, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErrText) {
					t.Fatalf("error = %q, want contains %q", err, tt.wantErrText)
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadMessage returned error: %v", err)
			}
			if !bytes.Equal(payload, tt.wantPayload) {
				t.Fatalf("payload = %q, want %q", payload, tt.wantPayload)
			}
		})
	}
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write(_ []byte) (int, error) {
	return 0, w.err
}

type chunkedReader struct {
	data      []byte
	chunkSize int
}

func (r *chunkedReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	if len(p) > r.chunkSize {
		p = p[:r.chunkSize]
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

func mustWireMessage(t *testing.T, payload []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	if err := WriteMessage(&buf, payload); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	return buf.Bytes()
}

func mustLengthPrefix(n uint32) []byte {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, n)
	return buf.Bytes()
}
