package aibbe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtensionBackground_RoutesNativeMessagesToTabs(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("extension", "background.js"))
	if err != nil {
		t.Fatalf("read extension/background.js: %v", err)
	}

	source := string(data)

	if !strings.Contains(source, "port.onMessage.addListener(async (message) => {") {
		t.Fatal("expected background.js to register an async Native Messaging onMessage listener")
	}

	if !strings.Contains(source, "findFreeTab()") {
		t.Fatal("expected background.js to call findFreeTab() for tab selection")
	}

	if !strings.Contains(source, "chrome.tabs.sendMessage(tabId,") {
		t.Fatal("expected background.js to relay messages via chrome.tabs.sendMessage")
	}

	if !strings.Contains(source, `{ status: "error", error: "no_free_tabs" }`) {
		t.Fatal("expected background.js to handle no_free_tabs error")
	}
}
