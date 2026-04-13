package aibbe

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

type nodeResult struct {
	Logs                      []string          `json:"logs"`
	Sent                      []map[string]any  `json:"sent"`
	SentTabMessages           []sentTabMessage  `json:"sentTabMessages"`
	MapSets                   []nodeMapSet      `json:"mapSets"`
	MapDeletes                []int             `json:"mapDeletes"`
	NativePostMessages        []map[string]any  `json:"nativePostMessages"`
	ContentResponses          []map[string]any  `json:"contentResponses"`
	InputValue                string            `json:"inputValue"`
	InputEvents               int               `json:"inputEvents"`
	ButtonClicks              int               `json:"buttonClicks"`
	ObserverDisconnected      bool              `json:"observerDisconnected"`
	ObserverConfig            map[string]any    `json:"observerConfig"`
	QuerySelectorCalls        []string          `json:"querySelectorCalls"`
	ConnectNativeHost         string            `json:"connectNativeHost"`
	HandshakeListenerExists   bool              `json:"handshakeListenerExists"`
	NativeMessageListenerSeen bool              `json:"nativeMessageListenerSeen"`
	TabRemovedListenerExists  bool              `json:"tabRemovedListenerExists"`
	ListenerReturnedTrue      bool              `json:"listenerReturnedTrue"`
	SettleTimerPending        bool              `json:"settleTimerPending"`
	FinalTabState             string            `json:"finalTabState"`
	FinalTabStates            map[string]string `json:"finalTabStates"`
	TabStateTransitions       []string          `json:"tabStateTransitions"`
}

type nodeMapSet struct {
	Key   int                    `json:"key"`
	Value map[string]interface{} `json:"value"`
}

type sentTabMessage struct {
	TabID   int            `json:"tabId"`
	Message map[string]any `json:"message"`
}

func TestRouting_RegistersHandshakeTabsOnlyFromTabContexts(t *testing.T) {
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

func TestRouting_SuccessfulRoutingToFreeTab(t *testing.T) {
	result := runRoutingScenario(t, `
listener({ type: "HANDSHAKE", service: "notebooklm" }, { tab: { id: 42 } });
snapshotState(42);

let resolveSend;
sendMessageImpl = () => new Promise((resolve) => {
  resolveSend = resolve;
});

const nativeMessagePromise = nativeMessageListener({ cmd: "generate", payload: "test" });
await flush();
snapshotState(42);
resolveSend({ status: "success", result: "mocked code source" });
await nativeMessagePromise;
snapshotState(42);
`)

	if result.ConnectNativeHost != "aibbe" {
		t.Fatalf("connectNative host = %q, want aibbe", result.ConnectNativeHost)
	}

	if !result.HandshakeListenerExists {
		t.Fatal("background.js did not register a runtime onMessage listener")
	}

	if !result.NativeMessageListenerSeen {
		t.Fatal("background.js did not register the native port onMessage listener")
	}

	if !result.TabRemovedListenerExists {
		t.Fatal("background.js did not register the tabs.onRemoved listener")
	}

	wantTransitions := []string{"free", "busy", "free"}
	if len(result.TabStateTransitions) != len(wantTransitions) {
		t.Fatalf("tab state transitions = %v, want %v", result.TabStateTransitions, wantTransitions)
	}

	for i, want := range wantTransitions {
		if got := result.TabStateTransitions[i]; got != want {
			t.Fatalf("tab state transitions[%d] = %q, want %q (all=%v)", i, got, want, result.TabStateTransitions)
		}
	}

	if len(result.SentTabMessages) != 1 {
		t.Fatalf("chrome.tabs.sendMessage calls = %d, want 1", len(result.SentTabMessages))
	}

	if got := result.SentTabMessages[0].TabID; got != 42 {
		t.Fatalf("chrome.tabs.sendMessage tabId = %d, want 42", got)
	}

	if got := result.SentTabMessages[0].Message["cmd"]; got != "generate" {
		t.Fatalf("chrome.tabs.sendMessage cmd = %v, want generate", got)
	}

	if got := result.SentTabMessages[0].Message["payload"]; got != "test" {
		t.Fatalf("chrome.tabs.sendMessage payload = %v, want test", got)
	}

	if len(result.NativePostMessages) != 1 {
		t.Fatalf("native postMessage calls = %d, want 1", len(result.NativePostMessages))
	}

	if got := result.NativePostMessages[0]["status"]; got != "success" {
		t.Fatalf("native response status = %v, want success", got)
	}

	if got := result.NativePostMessages[0]["result"]; got != "mocked code source" {
		t.Fatalf("native response result = %v, want mocked code source", got)
	}

	if got := result.FinalTabStates["42"]; got != "free" {
		t.Fatalf("final tab state for 42 = %q, want free", got)
	}
}

func TestRouting_NoFreeTabsError(t *testing.T) {
	result := runRoutingScenario(t, `
await nativeMessageListener({ cmd: "generate", payload: "test" });
`)

	if len(result.SentTabMessages) != 0 {
		t.Fatalf("chrome.tabs.sendMessage calls = %d, want 0", len(result.SentTabMessages))
	}

	if len(result.NativePostMessages) != 1 {
		t.Fatalf("native postMessage calls = %d, want 1", len(result.NativePostMessages))
	}

	if got := result.NativePostMessages[0]["status"]; got != "error" {
		t.Fatalf("native response status = %v, want error", got)
	}

	if got := result.NativePostMessages[0]["error"]; got != "no_free_tabs" {
		t.Fatalf("native response error = %v, want no_free_tabs", got)
	}
}

func TestRouting_BusyTabError(t *testing.T) {
	result := runRoutingScenario(t, `
listener({ type: "HANDSHAKE", service: "notebooklm" }, { tab: { id: 101 } });
capturedRegistry.get(101).state = "busy";
snapshotState(101);
await nativeMessageListener({ cmd: "generate", payload: "test" });
`)

	if len(result.SentTabMessages) != 0 {
		t.Fatalf("chrome.tabs.sendMessage calls = %d, want 0", len(result.SentTabMessages))
	}

	if len(result.NativePostMessages) != 1 {
		t.Fatalf("native postMessage calls = %d, want 1", len(result.NativePostMessages))
	}

	if got := result.NativePostMessages[0]["status"]; got != "error" {
		t.Fatalf("native response status = %v, want error", got)
	}

	if got := result.NativePostMessages[0]["error"]; got != "no_free_tabs" {
		t.Fatalf("native response error = %v, want no_free_tabs", got)
	}

	if len(result.TabStateTransitions) != 1 || result.TabStateTransitions[0] != "busy" {
		t.Fatalf("tab state transitions = %v, want [busy]", result.TabStateTransitions)
	}
}

func TestRouting_ContentScriptErrorPropagation(t *testing.T) {
	result := runRoutingScenario(t, `
listener({ type: "HANDSHAKE", service: "notebooklm" }, { tab: { id: 101 } });
snapshotState(101);

let rejectSend;
sendMessageImpl = () => new Promise((_, reject) => {
  rejectSend = reject;
});

const nativeMessagePromise = nativeMessageListener({ cmd: "generate", payload: "test" });
await flush();
snapshotState(101);
rejectSend(new Error("tab closed"));
await nativeMessagePromise;
snapshotState(101);
`)

	if len(result.NativePostMessages) != 1 {
		t.Fatalf("native postMessage calls = %d, want 1", len(result.NativePostMessages))
	}

	if got := result.NativePostMessages[0]["status"]; got != "error" {
		t.Fatalf("native response status = %v, want error", got)
	}

	if got := result.NativePostMessages[0]["error"]; got != "tab closed" {
		t.Fatalf("native response error = %v, want tab closed", got)
	}

	wantTransitions := []string{"free", "busy", "free"}
	if len(result.TabStateTransitions) != len(wantTransitions) {
		t.Fatalf("tab state transitions = %v, want %v", result.TabStateTransitions, wantTransitions)
	}

	for i, want := range wantTransitions {
		if got := result.TabStateTransitions[i]; got != want {
			t.Fatalf("tab state transitions[%d] = %q, want %q (all=%v)", i, got, want, result.TabStateTransitions)
		}
	}

	if got := result.FinalTabStates["101"]; got != "free" {
		t.Fatalf("final tab state for 101 = %q, want free", got)
	}
}

func TestRouting_ReactivePurgeDuringTransaction(t *testing.T) {
	result := runRoutingScenario(t, `
listener({ type: "HANDSHAKE", service: "notebooklm" }, { tab: { id: 101 } });
listener({ type: "HANDSHAKE", service: "notebooklm" }, { tab: { id: 202 } });

let resolveFirstSend;
sendMessageImpl = (tabId) => {
  if (tabId === 101) {
    return new Promise((resolve) => {
      resolveFirstSend = resolve;
    });
  }
  return Promise.resolve({ status: "success", result: "fallback route" });
};

const firstMessagePromise = nativeMessageListener({ cmd: "generate", payload: "first" });
await flush();
snapshotState(101);
removedListener(101, { isWindowClosing: false, windowId: 1 });
snapshotState(101);
resolveFirstSend({ status: "success", result: "first complete" });
await firstMessagePromise;
await nativeMessageListener({ cmd: "generate", payload: "second" });
snapshotState(202);
`)

	if len(result.MapDeletes) != 1 || result.MapDeletes[0] != 101 {
		t.Fatalf("purged tabs = %v, want [101]", result.MapDeletes)
	}

	if len(result.SentTabMessages) != 2 {
		t.Fatalf("chrome.tabs.sendMessage calls = %d, want 2", len(result.SentTabMessages))
	}

	if got := result.SentTabMessages[0].TabID; got != 101 {
		t.Fatalf("first routed tab = %d, want 101", got)
	}

	if got := result.SentTabMessages[1].TabID; got != 202 {
		t.Fatalf("second routed tab = %d, want 202", got)
	}

	if got := result.TabStateTransitions[0]; got != "busy" {
		t.Fatalf("first transition = %q, want busy (all=%v)", got, result.TabStateTransitions)
	}

	if got := result.TabStateTransitions[1]; got != "absent" {
		t.Fatalf("second transition = %q, want absent (all=%v)", got, result.TabStateTransitions)
	}

	if got := result.TabStateTransitions[2]; got != "free" {
		t.Fatalf("third transition = %q, want free (all=%v)", got, result.TabStateTransitions)
	}

	if _, ok := result.FinalTabStates["101"]; ok {
		t.Fatalf("final tab states unexpectedly contains purged tab 101: %v", result.FinalTabStates)
	}

	if got := result.FinalTabStates["202"]; got != "free" {
		t.Fatalf("final tab state for 202 = %q, want free", got)
	}

	if containsLog(result.Logs, "[unhandledRejection]") {
		t.Fatalf("unexpected unhandled rejection log: %v", result.Logs)
	}
}

func TestRouting_PurgesClosedTabsReactively(t *testing.T) {
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

func runRoutingScenario(t *testing.T, body string) nodeResult {
	t.Helper()

	var result nodeResult
	runNodeJSON(t, `
const path = require("path");
const logs = [];
const mapSets = [];
const mapDeletes = [];
const nativePostMessages = [];
const sentTabMessages = [];
const tabStateTransitions = [];
let listener = null;
let removedListener = null;
let nativeMessageListener = null;
let connectNativeHost = "";
let capturedRegistry = null;
let sendMessageImpl = () => Promise.resolve({ status: "success", result: "default" });
const NativeMap = global.Map;

global.Map = class ObservedMap extends NativeMap {
  constructor(...args) {
    super(...args);
    capturedRegistry = this;
  }

  set(key, value) {
    mapSets.push({ key, value: { ...value } });
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
  postMessage(message) {
    nativePostMessages.push(message);
  },
};

function snapshotState(tabId) {
  if (!capturedRegistry) {
    tabStateTransitions.push("missing_registry");
    return;
  }

  const entry = capturedRegistry.get(tabId);
  tabStateTransitions.push(entry ? String(entry.state) : "absent");
}

async function flush() {
  await Promise.resolve();
  await new Promise((resolve) => setTimeout(resolve, 0));
}

process.on("unhandledRejection", (error) => {
  logs.push("[unhandledRejection] " + (error && error.message ? error.message : String(error)));
});

global.console = {
  log: (...args) => logs.push(args.map((arg) => typeof arg === "string" ? arg : JSON.stringify(arg)).join(" ")),
  warn: (...args) => logs.push(args.map((arg) => typeof arg === "string" ? arg : JSON.stringify(arg)).join(" ")),
  error: (...args) => logs.push(args.map((arg) => typeof arg === "string" ? arg : JSON.stringify(arg)).join(" ")),
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
      addListener(fn) {
        removedListener = fn;
      },
    },
    sendMessage: (tabId, message) => {
      sentTabMessages.push({ tabId, message });
      return sendMessageImpl(tabId, message);
    },
  },
};

require(path.resolve(process.cwd(), "extension/background.js"));

(async () => {
`+body+`
  await flush();

  const finalTabStates = {};
  if (capturedRegistry) {
    for (const [tabId, entry] of capturedRegistry.entries()) {
      finalTabStates[String(tabId)] = entry ? String(entry.state) : "";
    }
  }

  process.stdout.write(JSON.stringify({
    logs,
    mapSets,
    mapDeletes,
    nativePostMessages,
    sentTabMessages,
    tabStateTransitions,
    finalTabStates,
    connectNativeHost,
    handshakeListenerExists: typeof listener === "function",
    nativeMessageListenerSeen: typeof nativeMessageListener === "function",
    tabRemovedListenerExists: typeof removedListener === "function",
  }));
})().catch((error) => {
  console.error("routing scenario failed", error);
  process.exit(1);
});
`, &result)

	return result
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
