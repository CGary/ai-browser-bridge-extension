package aibbe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtensionBackground_EchoesNativeMessagesBackToPort(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("extension", "background.js"))
	if err != nil {
		t.Fatalf("read extension/background.js: %v", err)
	}

	source := string(data)

	if !strings.Contains(source, "port.onMessage.addListener((message) => {") {
		t.Fatal("expected background.js to register a Native Messaging onMessage listener")
	}

	if !strings.Contains(source, "port.postMessage(message);") {
		t.Fatal("expected background.js to echo Native Messaging payloads back with port.postMessage(message)")
	}
}
