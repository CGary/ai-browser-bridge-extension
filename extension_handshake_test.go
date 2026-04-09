package aibbe

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type extensionManifest struct {
	Permissions    []string               `json:"permissions"`
	ContentScripts []extensionContentSpec `json:"content_scripts"`
}

type extensionContentSpec struct {
	Matches []string `json:"matches"`
	JS      []string `json:"js"`
	RunAt   string   `json:"run_at"`
}

type nodeResult struct {
	Logs                      []string         `json:"logs"`
	Sent                      []map[string]any `json:"sent"`
	MapSets                   []nodeMapSet     `json:"mapSets"`
	MapDeletes                []int            `json:"mapDeletes"`
	NativePostMessages        []map[string]any `json:"nativePostMessages"`
	ConnectNativeHost         string           `json:"connectNativeHost"`
	HandshakeListenerExists   bool             `json:"handshakeListenerExists"`
	NativeMessageListenerSeen bool             `json:"nativeMessageListenerSeen"`
	TabRemovedListenerExists  bool             `json:"tabRemovedListenerExists"`
}

type nodeMapSet struct {
	Key   int                    `json:"key"`
	Value map[string]interface{} `json:"value"`
}

func readExtensionManifest(t *testing.T) extensionManifest {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("extension", "manifest.json"))
	if err != nil {
		t.Fatalf("read extension/manifest.json: %v", err)
	}

	var manifest extensionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal extension/manifest.json: %v", err)
	}

	return manifest
}

func readExtensionFile(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("extension", name))
	if err != nil {
		t.Fatalf("read extension/%s: %v", name, err)
	}

	return string(data)
}

func TestExtensionManifest_RegistersNotebookLMContentScript(t *testing.T) {
	manifest := readExtensionManifest(t)

	if !contains(manifest.Permissions, "tabs") {
		t.Fatal(`expected extension/manifest.json permissions to include "tabs"`)
	}

	tests := []struct {
		name          string
		match         string
		js            string
		runAt         string
		wantInjection bool
	}{
		{
			name:          "injects on notebooklm at document_idle",
			match:         "https://notebooklm.google.com/*",
			js:            "content.js",
			runAt:         "document_idle",
			wantInjection: true,
		},
		{
			name:          "does not inject on unrelated google host",
			match:         "https://google.com/*",
			js:            "content.js",
			runAt:         "document_idle",
			wantInjection: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasContentScript(manifest.ContentScripts, tt.match, tt.js, tt.runAt)
			if got != tt.wantInjection {
				t.Fatalf("content script injection for %q = %v, want %v", tt.match, got, tt.wantInjection)
			}
		})
	}
}

func TestExtensionContent_SendsNotebookLMHandshakeOnLoad(t *testing.T) {
	var result nodeResult
	runNodeJSON(t, `
const path = require("path");
const logs = [];
const sent = [];

global.console = {
  log: (...args) => logs.push(args.map((arg) => typeof arg === "string" ? arg : JSON.stringify(arg)).join(" ")),
  warn: () => {},
  error: () => {},
};

global.chrome = {
  runtime: {
    sendMessage: (message) => sent.push(message),
  },
};

require(path.resolve(process.cwd(), "extension/content.js"));
process.stdout.write(JSON.stringify({ logs, sent }));
`, &result)

	if len(result.Sent) != 1 {
		t.Fatalf("content.js sent %d messages, want 1", len(result.Sent))
	}

	if got := result.Sent[0]["type"]; got != "HANDSHAKE" {
		t.Fatalf("handshake type = %v, want HANDSHAKE", got)
	}

	if got := result.Sent[0]["service"]; got != "notebooklm" {
		t.Fatalf("handshake service = %v, want notebooklm", got)
	}

	if !containsLog(result.Logs, "[aibbe] Handshake sent for notebooklm") {
		t.Fatalf("expected content.js runtime logs to include handshake confirmation, got %v", result.Logs)
	}
}

func TestExtensionBackground_RegistersHandshakeTabsOnlyFromTabContexts(t *testing.T) {
	tests := []struct {
		name                string
		invocation          string
		wantRegistrations   int
		wantRegisteredTabID int
		wantLog             string
	}{
		{
			name:                "registers handshake from tab context",
			invocation:          `listener({ type: "HANDSHAKE", service: "notebooklm" }, { tab: { id: 123 } });`,
			wantRegistrations:   1,
			wantRegisteredTabID: 123,
			wantLog:             "[aibbe] Tab 123 registered for notebooklm",
		},
		{
			name:              "ignores handshake without tab context",
			invocation:        `listener({ type: "HANDSHAKE", service: "notebooklm" }, {});`,
			wantRegistrations: 0,
		},
		{
			name:              "ignores non-handshake messages",
			invocation:        `listener({ type: "PING", service: "notebooklm" }, { tab: { id: 123 } });`,
			wantRegistrations: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result nodeResult
			runNodeJSON(t, `
const path = require("path");
const logs = [];
const mapSets = [];
let listener = null;
let nativeMessageListener = null;
let connectNativeHost = "";
const NativeMap = global.Map;

global.Map = class ObservedMap extends NativeMap {
  set(key, value) {
    mapSets.push({ key, value });
    return super.set(key, value);
  }
};

const port = {
  onMessage: {
    addListener(fn) {
      nativeMessageListener = fn;
    },
  },
  onDisconnect: {
    addListener() {},
  },
  postMessage() {},
};

global.console = {
  log: (...args) => logs.push(args.map((arg) => typeof arg === "string" ? arg : JSON.stringify(arg)).join(" ")),
  warn: () => {},
  error: () => {},
};

global.chrome = {
  runtime: {
    connectNative: (host) => {
      connectNativeHost = host;
      return port;
    },
    onMessage: {
      addListener(fn) {
        listener = fn;
      },
    },
    lastError: undefined,
  },
  tabs: {
    onRemoved: {
      addListener() {},
    },
  },
};

require(path.resolve(process.cwd(), "extension/background.js"));
`+tt.invocation+`
process.stdout.write(JSON.stringify({
  logs,
  mapSets,
  connectNativeHost,
  handshakeListenerExists: typeof listener === "function",
  nativeMessageListenerSeen: typeof nativeMessageListener === "function",
}));
`, &result)

			if !result.HandshakeListenerExists {
				t.Fatal("background.js did not register a runtime onMessage listener")
			}

			if result.ConnectNativeHost != "aibbe" {
				t.Fatalf("connectNative host = %q, want aibbe", result.ConnectNativeHost)
			}

			if !result.NativeMessageListenerSeen {
				t.Fatal("background.js did not register the native port onMessage listener")
			}

			if len(result.MapSets) != tt.wantRegistrations {
				t.Fatalf("tab registrations = %d, want %d", len(result.MapSets), tt.wantRegistrations)
			}

			if tt.wantRegistrations == 0 {
				return
			}

			if got := result.MapSets[0].Key; got != tt.wantRegisteredTabID {
				t.Fatalf("registered tab id = %d, want %d", got, tt.wantRegisteredTabID)
			}

			if got := result.MapSets[0].Value["service"]; got != "notebooklm" {
				t.Fatalf("registered service = %v, want notebooklm", got)
			}

			if got := result.MapSets[0].Value["state"]; got != "free" {
				t.Fatalf("registered state = %v, want free", got)
			}

			lastSeen, ok := result.MapSets[0].Value["lastSeen"].(float64)
			if !ok || lastSeen <= 0 {
				t.Fatalf("registered lastSeen = %v, want positive numeric timestamp", result.MapSets[0].Value["lastSeen"])
			}

			if !containsLog(result.Logs, tt.wantLog) {
				t.Fatalf("expected logs to include %q, got %v", tt.wantLog, result.Logs)
			}
		})
	}
}

func TestExtensionBackground_EchoesNativeMessagesBackToPortAtRuntime(t *testing.T) {
	var result nodeResult
	runNodeJSON(t, `
const path = require("path");
const logs = [];
const nativePostMessages = [];
let nativeMessageListener = null;
let listener = null;

const port = {
  onMessage: {
    addListener(fn) {
      nativeMessageListener = fn;
    },
  },
  onDisconnect: {
    addListener() {},
  },
  postMessage(message) {
    nativePostMessages.push(message);
  },
};

global.console = {
  log: (...args) => logs.push(args.map((arg) => typeof arg === "string" ? arg : JSON.stringify(arg)).join(" ")),
  warn: () => {},
  error: () => {},
};

global.chrome = {
  runtime: {
    connectNative: () => port,
    onMessage: {
      addListener(fn) {
        listener = fn;
      },
    },
    lastError: undefined,
  },
  tabs: {
    onRemoved: {
      addListener() {},
    },
  },
};

require(path.resolve(process.cwd(), "extension/background.js"));
nativeMessageListener({ ok: true, payload: "echo" });
process.stdout.write(JSON.stringify({
  logs,
  nativePostMessages,
  nativeMessageListenerSeen: typeof nativeMessageListener === "function",
  handshakeListenerExists: typeof listener === "function",
}));
`, &result)

	if !result.NativeMessageListenerSeen {
		t.Fatal("background.js did not register the native port onMessage listener")
	}

	if len(result.NativePostMessages) != 1 {
		t.Fatalf("native echo postMessage calls = %d, want 1", len(result.NativePostMessages))
	}

	if got := result.NativePostMessages[0]["payload"]; got != "echo" {
		t.Fatalf("echoed payload = %v, want echo", got)
	}

	okValue, ok := result.NativePostMessages[0]["ok"].(bool)
	if !ok || !okValue {
		t.Fatalf("echoed ok flag = %v, want true", result.NativePostMessages[0]["ok"])
	}
}

func TestExtensionBackground_PurgesClosedTabsReactively(t *testing.T) {
	tests := []struct {
		name            string
		setup           string
		invocation      string
		wantDeletes     []int
		wantLog         string
		wantAbsentLog   string
		wantRemovedHook bool
	}{
		{
			name: "purges registered tab on closure",
			setup: `
listener({ type: "HANDSHAKE", service: "notebooklm" }, { tab: { id: 123 } });
`,
			invocation:      `removedListener(123, { isWindowClosing: false, windowId: 1 });`,
			wantDeletes:     []int{123},
			wantLog:         "[aibbe] Tab 123 purged from registry",
			wantRemovedHook: true,
		},
		{
			name: "ignores non registered tab closure",
			setup: `
listener({ type: "HANDSHAKE", service: "notebooklm" }, { tab: { id: 123 } });
`,
			invocation:      `removedListener(456, { isWindowClosing: false, windowId: 1 });`,
			wantDeletes:     nil,
			wantAbsentLog:   "purged from registry",
			wantRemovedHook: true,
		},
		{
			name: "purges tab even when window is closing",
			setup: `
listener({ type: "HANDSHAKE", service: "notebooklm" }, { tab: { id: 789 } });
`,
			invocation:      `removedListener(789, { isWindowClosing: true, windowId: 7 });`,
			wantDeletes:     []int{789},
			wantLog:         "[aibbe] Tab 789 purged from registry",
			wantRemovedHook: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result nodeResult
			runNodeJSON(t, `
const path = require("path");
const logs = [];
const mapSets = [];
const mapDeletes = [];
let listener = null;
let removedListener = null;
let nativeMessageListener = null;
const NativeMap = global.Map;

global.Map = class ObservedMap extends NativeMap {
  set(key, value) {
    mapSets.push({ key, value });
    return super.set(key, value);
  }

  delete(key) {
    mapDeletes.push(key);
    return super.delete(key);
  }
};

const port = {
  onMessage: {
    addListener(fn) {
      nativeMessageListener = fn;
    },
  },
  onDisconnect: {
    addListener() {},
  },
  postMessage() {},
};

global.console = {
  log: (...args) => logs.push(args.map((arg) => typeof arg === "string" ? arg : JSON.stringify(arg)).join(" ")),
  warn: () => {},
  error: () => {},
};

global.chrome = {
  runtime: {
    connectNative: () => port,
    onMessage: {
      addListener(fn) {
        listener = fn;
      },
    },
    lastError: undefined,
  },
  tabs: {
    onRemoved: {
      addListener(fn) {
        removedListener = fn;
      },
    },
  },
};

require(path.resolve(process.cwd(), "extension/background.js"));
`+tt.setup+`
`+tt.invocation+`
process.stdout.write(JSON.stringify({
  logs,
  mapSets,
  mapDeletes,
  handshakeListenerExists: typeof listener === "function",
  nativeMessageListenerSeen: typeof nativeMessageListener === "function",
  tabRemovedListenerExists: typeof removedListener === "function",
}));
`, &result)

			if !result.HandshakeListenerExists {
				t.Fatal("background.js did not register a runtime onMessage listener")
			}

			if !result.NativeMessageListenerSeen {
				t.Fatal("background.js did not register the native port onMessage listener")
			}

			if result.TabRemovedListenerExists != tt.wantRemovedHook {
				t.Fatalf("tab removed listener exists = %v, want %v", result.TabRemovedListenerExists, tt.wantRemovedHook)
			}

			if len(result.MapDeletes) != len(tt.wantDeletes) {
				t.Fatalf("tab purges = %d, want %d", len(result.MapDeletes), len(tt.wantDeletes))
			}

			for i, wantDelete := range tt.wantDeletes {
				if got := result.MapDeletes[i]; got != wantDelete {
					t.Fatalf("purged tab id[%d] = %d, want %d", i, got, wantDelete)
				}
			}

			if tt.wantAbsentLog != "" && containsLog(result.Logs, tt.wantAbsentLog) {
				t.Fatalf("expected logs not to include %q, got %v", tt.wantAbsentLog, result.Logs)
			}

			if tt.wantLog == "" {
				return
			}

			if !containsLog(result.Logs, tt.wantLog) {
				t.Fatalf("expected logs to include %q, got %v", tt.wantLog, result.Logs)
			}
		})
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}

func hasContentScript(scripts []extensionContentSpec, match, js, runAt string) bool {
	for _, script := range scripts {
		if contains(script.Matches, match) && contains(script.JS, js) && script.RunAt == runAt {
			return true
		}
	}

	return false
}

func containsLog(logs []string, target string) bool {
	for _, logLine := range logs {
		if strings.Contains(logLine, target) {
			return true
		}
	}

	return false
}

func runNodeJSON(t *testing.T, script string, target interface{}) {
	t.Helper()

	nodeBinary, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node binary not available in PATH")
	}

	cmd := exec.Command(nodeBinary, "-e", script)
	cmd.Dir = "."

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run node script: %v\noutput:\n%s", err, output)
	}

	if err := json.Unmarshal(output, target); err != nil {
		t.Fatalf("unmarshal node result: %v\noutput:\n%s", err, output)
	}
}
