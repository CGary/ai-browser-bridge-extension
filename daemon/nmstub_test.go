package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"aibbe/internal/nativemessaging"
)

var errNMStubClosed = errors.New("nm stub closed")

// StubConfig configures the NM stub behavior.
type StubConfig struct {
	ResponseFunc func(request json.RawMessage) (response json.RawMessage, delay time.Duration)
}

// NMStub simulates the Chrome Extension Native Messaging endpoint in tests.
type NMStub struct {
	mu        sync.Mutex
	received  []json.RawMessage
	config    StubConfig
	input     *io.PipeReader
	output    *io.PipeWriter
	closed    chan struct{}
	done      chan struct{}
	closeOnce sync.Once
	closeErr  error
}

// NewNMStub creates a programmable Native Messaging stub and returns the ends
// the daemon should use for stdinReader and nativeOut, respectively.
func NewNMStub(config StubConfig) (*NMStub, io.ReadCloser, io.WriteCloser) {
	daemonToStubR, daemonToStubW := io.Pipe()
	stubToDaemonR, stubToDaemonW := io.Pipe()

	stub := &NMStub{
		config: config,
		input:  daemonToStubR,
		output: stubToDaemonW,
		closed: make(chan struct{}),
		done:   make(chan struct{}),
	}

	go stub.loop()

	return stub, stubToDaemonR, daemonToStubW
}

func (s *NMStub) loop() {
	defer close(s.done)

	for {
		payload, err := nativemessaging.ReadMessage(s.input)
		if err != nil {
			return
		}

		request := cloneRawMessage(payload)
		s.mu.Lock()
		s.received = append(s.received, request)
		s.mu.Unlock()

		response, delay := s.config.ResponseFunc(request)
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-s.closed:
				if !timer.Stop() {
					<-timer.C
				}
				return
			}
		}

		if err := nativemessaging.WriteMessage(s.output, response); err != nil {
			return
		}
	}
}

// ReceivedMessages returns ordered copies of all messages received by the stub.
func (s *NMStub) ReceivedMessages() []json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]json.RawMessage, len(s.received))
	for i, msg := range s.received {
		result[i] = cloneRawMessage(msg)
	}
	return result
}

// Close terminates the processing goroutine and closes internal pipes.
func (s *NMStub) Close() error {
	s.closeOnce.Do(func() {
		close(s.closed)
		errIn := s.input.CloseWithError(errNMStubClosed)
		errOut := s.output.CloseWithError(errNMStubClosed)
		<-s.done
		s.closeErr = errors.Join(errIn, errOut)
	})

	return s.closeErr
}

func WithEchoResponse() StubConfig {
	return StubConfig{
		ResponseFunc: func(request json.RawMessage) (json.RawMessage, time.Duration) {
			return cloneRawMessage(request), 0
		},
	}
}

func WithSuccessResponse(result string) StubConfig {
	return StubConfig{
		ResponseFunc: func(json.RawMessage) (json.RawMessage, time.Duration) {
			response, err := json.Marshal(map[string]string{
				"status": "success",
				"result": result,
			})
			if err != nil {
				panic(err)
			}
			return response, 0
		},
	}
}

func WithErrorResponse(errorCode string) StubConfig {
	return StubConfig{
		ResponseFunc: func(json.RawMessage) (json.RawMessage, time.Duration) {
			response, err := json.Marshal(map[string]string{
				"status": "error",
				"error":  errorCode,
			})
			if err != nil {
				panic(err)
			}
			return response, 0
		},
	}
}

func WithDelayedResponse(response json.RawMessage, delay time.Duration) StubConfig {
	payload := cloneRawMessage(response)
	return StubConfig{
		ResponseFunc: func(json.RawMessage) (json.RawMessage, time.Duration) {
			return cloneRawMessage(payload), delay
		},
	}
}

func cloneRawMessage(payload []byte) json.RawMessage {
	cloned := make(json.RawMessage, len(payload))
	copy(cloned, payload)
	return cloned
}

func TestNMStub_Presets(t *testing.T) {
	tests := []struct {
		name     string
		config   StubConfig
		request  json.RawMessage
		wantResp json.RawMessage
		minDelay time.Duration
	}{
		{
			name:     "echo returns request unchanged",
			config:   WithEchoResponse(),
			request:  json.RawMessage(`{"cmd":"ping","payload":"data"}`),
			wantResp: json.RawMessage(`{"cmd":"ping","payload":"data"}`),
		},
		{
			name:     "success returns formatted JSON",
			config:   WithSuccessResponse("print('hello')"),
			request:  json.RawMessage(`{"cmd":"generate","payload":"data"}`),
			wantResp: json.RawMessage(`{"status":"success","result":"print('hello')"}`),
		},
		{
			name:     "error returns formatted JSON",
			config:   WithErrorResponse("no_free_tabs"),
			request:  json.RawMessage(`{"cmd":"generate","payload":"data"}`),
			wantResp: json.RawMessage(`{"status":"error","error":"no_free_tabs"}`),
		},
		{
			name:     "delayed returns configured payload after delay",
			config:   WithDelayedResponse(json.RawMessage(`{"status":"success","result":"slow"}`), 50*time.Millisecond),
			request:  json.RawMessage(`{"cmd":"generate","payload":"data"}`),
			wantResp: json.RawMessage(`{"status":"success","result":"slow"}`),
			minDelay: 50 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub, daemonStdin, daemonStdout := NewNMStub(tt.config)
			t.Cleanup(func() {
				_ = daemonStdin.Close()
				_ = daemonStdout.Close()
				_ = stub.Close()
			})

			start := time.Now()
			if err := nativemessaging.WriteMessage(daemonStdout, tt.request); err != nil {
				t.Fatalf("WriteMessage(request): %v", err)
			}

			gotResp, err := nativemessaging.ReadMessage(daemonStdin)
			if err != nil {
				t.Fatalf("ReadMessage(response): %v", err)
			}
			if elapsed := time.Since(start); elapsed < tt.minDelay {
				t.Fatalf("response delay = %v, want >= %v", elapsed, tt.minDelay)
			}

			assertJSONEqual(t, gotResp, tt.wantResp)
		})
	}
}

func TestNMStub_WireProtocol(t *testing.T) {
	request := json.RawMessage(`{"cmd":"wire","payload":"check"}`)
	response := json.RawMessage(`{"status":"success","result":"encoded"}`)
	stub, daemonStdin, daemonStdout := NewNMStub(WithDelayedResponse(response, 0))
	defer func() {
		_ = daemonStdin.Close()
		_ = daemonStdout.Close()
		_ = stub.Close()
	}()

	var raw bytes.Buffer
	if err := nativemessaging.WriteMessage(&raw, request); err != nil {
		t.Fatalf("WriteMessage(raw): %v", err)
	}
	if err := nativemessaging.WriteMessage(daemonStdout, request); err != nil {
		t.Fatalf("WriteMessage(request): %v", err)
	}
	if _, err := nativemessaging.ReadMessage(daemonStdin); err != nil {
		t.Fatalf("ReadMessage(response): %v", err)
	}

	got := stub.ReceivedMessages()
	if len(got) != 1 {
		t.Fatalf("ReceivedMessages len = %d, want 1", len(got))
	}
	assertJSONEqual(t, got[0], request)
	assertNativeMessage(t, raw.Bytes(), request)
}

func TestNMStub_MessageLog(t *testing.T) {
	stub, daemonStdin, daemonStdout := NewNMStub(StubConfig{
		ResponseFunc: func(request json.RawMessage) (json.RawMessage, time.Duration) {
			var payload struct {
				Cmd string `json:"cmd"`
			}
			if err := json.Unmarshal(request, &payload); err != nil {
				t.Fatalf("json.Unmarshal(request): %v", err)
			}
			return json.RawMessage(`{"ack":"` + payload.Cmd + `"}`), 0
		},
	})
	defer func() {
		_ = daemonStdin.Close()
		_ = daemonStdout.Close()
		_ = stub.Close()
	}()

	requests := []json.RawMessage{
		json.RawMessage(`{"cmd":"one","payload":"first"}`),
		json.RawMessage(`{"cmd":"two","payload":"second"}`),
		json.RawMessage(`{"cmd":"three","payload":"third"}`),
	}
	for i, request := range requests {
		if err := nativemessaging.WriteMessage(daemonStdout, request); err != nil {
			t.Fatalf("WriteMessage(request %d): %v", i, err)
		}
		if _, err := nativemessaging.ReadMessage(daemonStdin); err != nil {
			t.Fatalf("ReadMessage(response %d): %v", i, err)
		}
	}

	got := stub.ReceivedMessages()
	if len(got) != len(requests) {
		t.Fatalf("ReceivedMessages len = %d, want %d", len(got), len(requests))
	}
	for i := range requests {
		assertJSONEqual(t, got[i], requests[i])
	}

	got[0][0] = 'X'
	fresh := stub.ReceivedMessages()
	assertJSONEqual(t, fresh[0], requests[0])
}

func TestNMStub_Shutdown(t *testing.T) {
	stub, daemonStdin, daemonStdout := NewNMStub(WithSuccessResponse("ok"))
	defer func() {
		_ = daemonStdin.Close()
		_ = daemonStdout.Close()
	}()

	request := json.RawMessage(`{"cmd":"close","payload":"now"}`)
	if err := nativemessaging.WriteMessage(daemonStdout, request); err != nil {
		t.Fatalf("WriteMessage(request): %v", err)
	}
	if _, err := nativemessaging.ReadMessage(daemonStdin); err != nil {
		t.Fatalf("ReadMessage(response): %v", err)
	}

	if err := stub.Close(); err != nil {
		t.Fatalf("Close(first): %v", err)
	}
	if err := stub.Close(); err != nil {
		t.Fatalf("Close(second): %v", err)
	}

	got := stub.ReceivedMessages()
	if len(got) != 1 {
		t.Fatalf("ReceivedMessages len after Close = %d, want 1", len(got))
	}
	assertJSONEqual(t, got[0], request)
}

func TestNMStub_OversizedResponseStopsWithoutWritingGarbage(t *testing.T) {
	stub, daemonStdin, daemonStdout := NewNMStub(StubConfig{
		ResponseFunc: func(json.RawMessage) (json.RawMessage, time.Duration) {
			return json.RawMessage(bytes.Repeat([]byte("x"), nativemessaging.MaxMessageSize+1)), 0
		},
	})
	defer func() {
		_ = daemonStdin.Close()
		_ = daemonStdout.Close()
	}()

	if err := nativemessaging.WriteMessage(daemonStdout, json.RawMessage(`{"cmd":"oversize"}`)); err != nil {
		t.Fatalf("WriteMessage(request): %v", err)
	}
	if err := stub.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	buf := make([]byte, 1)
	n, err := daemonStdin.Read(buf)
	if n != 0 {
		t.Fatalf("Read bytes = %d, want 0", n)
	}
	if !errors.Is(err, io.EOF) && !errors.Is(err, errNMStubClosed) {
		t.Fatalf("Read error = %v, want EOF or %v", err, errNMStubClosed)
	}
}

func assertJSONEqual(t *testing.T, got, want []byte) {
	t.Helper()

	var gotValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("json.Unmarshal(got): %v", err)
	}

	var wantValue any
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("json.Unmarshal(want): %v", err)
	}

	gotJSON, err := json.Marshal(gotValue)
	if err != nil {
		t.Fatalf("json.Marshal(gotValue): %v", err)
	}
	wantJSON, err := json.Marshal(wantValue)
	if err != nil {
		t.Fatalf("json.Marshal(wantValue): %v", err)
	}
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Fatalf("json = %s, want %s", gotJSON, wantJSON)
	}
}
