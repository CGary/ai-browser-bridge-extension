package aibbe

import (
	"encoding/json"
	"os"
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

func readExtensionManifest(t *testing.T) extensionManifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("extension", "manifest.json"))
	if err != nil { t.Fatalf("read manifest: %v", err) }
	var manifest extensionManifest
	if err := json.Unmarshal(data, &manifest); err != nil { t.Fatalf("unmarshal manifest: %v", err) }
	return manifest
}

func readExtensionFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("extension", name))
	if err != nil { t.Fatalf("read extension/%s: %v", name, err) }
	return string(data)
}

func TestExtensionManifest_RegistersNotebookLMContentScript(t *testing.T) {
	manifest := readExtensionManifest(t)
	if !contains(manifest.Permissions, "storage") { t.Fatal("expected storage permission") }
}

func TestExtensionContent_SendsNotebookLMHandshakeOnLoad(t *testing.T) {
	var result nodeResult
	runNodeJSON(t, `
const path = require("path");
global.chrome = { runtime: { sendMessage: (m) => process.stdout.write(JSON.stringify({ sent: [m] })), onMessage: { addListener() {} } } };
global.MutationObserver = class { constructor() {} observe() {} disconnect() {} };
global.document = { body: {}, querySelector: (s) => s === "div.cover-title" ? { textContent: "SIAT" } : null };
global.console = { log() {}, warn() {}, error() {} };
require(path.resolve(process.cwd(), "extension/content.js"));
`, &result)
	if len(result.Sent) != 1 || result.Sent[0]["type"] != "HANDSHAKE" { t.Fatalf("got %v", result.Sent) }
}

func TestExtensionContent_ProcessesGenerateCommand(t *testing.T) {
	var result nodeResult
	runNodeJSON(t, `
const path = require("path");
let onMessageListener = null, observerCallback = null;

const input = { 
  value: "", 
  focus() {}, 
  getAttribute() { return null; },
  dispatchEvent() { return true; } 
};
const submitButton = { disabled: true, click() {} };
const responseContainer = {
  querySelector(s) { return s.includes("text") ? { textContent: "ok", querySelectorAll() { return []; } } : (s.includes("actions") ? {} : null); },
  querySelectorAll() { return []; }
};

global.Event = class { constructor(t) { this.type = t; } };
global.window = { 
  requestAnimationFrame: (cb) => cb(), 
  __AIBBE_SETTLE_MS: 0,
  HTMLTextAreaElement: { prototype: { value: "" } }
};
global.requestAnimationFrame = global.window.requestAnimationFrame;
global.MutationObserver = class { constructor(cb) { observerCallback = cb; } observe() {} disconnect() {} };
global.document = {
  execCommand() { return true; },
  querySelector(s) {
    if (s.includes("textarea")) return input;
    if (s.includes("button")) return submitButton;
    if (s.includes("chat-message") || s.includes("content")) return responseContainer;
    return null;
  },
  querySelectorAll(s) { return (s.includes("chat-message") || s.includes("content")) ? [responseContainer] : []; },
  body: { appendChild: () => {} }
};
global.console = { log() {}, warn() {}, error() {} };
global.chrome = {
  runtime: { sendMessage: () => {}, onMessage: { addListener(fn) { onMessageListener = fn; } } },
  storage: { local: { get: () => Promise.resolve({}) } }
};

require(path.resolve(process.cwd(), "extension/content.js"));

setTimeout(() => {
  onMessageListener({ cmd: "generate", payload: "data" }, {}, (r) => {
    process.stdout.write(JSON.stringify({ contentResponses: [r] }));
    process.exit(0);
  });
  setTimeout(() => {
    if (observerCallback) observerCallback([{ type: "attributes" }]);
  }, 10);
}, 50);
`, &result)

	if len(result.ContentResponses) == 0 || result.ContentResponses[0]["status"] != "success" {
		t.Fatalf("failed: %v", result.ContentResponses)
	}
}

func TestExtensionContent_DefinesSelectorCascade(t *testing.T) {
	source := readExtensionFile(t, "content.js")
	if !strings.Contains(source, "const SELECTORS = {") { t.Fatal("SELECTORS missing") }
}

func TestExtensionContent_LoadSelectors_HandlesMissingStorage(t *testing.T) {
	var result nodeResult
	runNodeJSON(t, `
const path = require("path");
const logs = [];
global.chrome = { runtime: { onMessage: { addListener() {} } } };
global.console = { 
  log: (m) => { logs.push(m); }, 
  warn: () => {} 
};
global.window = { requestAnimationFrame: () => {} };
global.document = { querySelector: () => null, body: { appendChild: () => {} } };
require(path.resolve(process.cwd(), "extension/content.js"));
setTimeout(() => {
  process.stdout.write(JSON.stringify({ logs }));
  process.exit(0);
}, 100);
`, &result)

	if len(result.Logs) == 0 { t.Errorf("log missing: %v", result.Logs) }
}

func contains(values []string, target string) bool {
	for _, v := range values { if v == target { return true } }
	return false
}
func hasContentScript(scripts []extensionContentSpec, match, js, runAt string) bool {
	for _, s := range scripts {
		if contains(s.Matches, match) && contains(s.JS, js) && s.RunAt == runAt { return true }
	}
	return false
}
