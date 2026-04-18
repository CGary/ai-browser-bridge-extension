package ipc

import (
	"encoding/json"
	"testing"
)

func TestRequest_Serialization(t *testing.T) {
	tests := []struct {
		name     string
		input    Request
		wantJSON string
	}{
		{
			name:     "request with target",
			input:    Request{Cmd: "generate", Target: "SIAT", Payload: "question"},
			wantJSON: `{"cmd":"generate","target":"SIAT","payload":"question"}`,
		},
		{
			name:     "request without target",
			input:    Request{Cmd: "generate", Payload: "question"},
			wantJSON: `{"cmd":"generate","payload":"question"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != tt.wantJSON {
				t.Errorf("Marshal() got = %s, want %s", got, tt.wantJSON)
			}
		})
	}
}
