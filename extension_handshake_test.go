package aibbe

import (
	"encoding/json"
	"fmt"
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

type execCall struct {
	Cmd string `json:"cmd"`
	Val any    `json:"val"`
}

type evtRecord struct {
	Type    string `json:"type"`
	Bubbles bool   `json:"bubbles"`
}

type keyEvent struct {
	Type string `json:"type"`
	Key  string `json:"key"`
}

// nodeHandshakeResult extends nodeResult with fields needed for humanization tests.
type nodeHandshakeResult struct {
	nodeResult
	ExecCalls           []execCall  `json:"execCalls"`
	SetterCalled        bool        `json:"setterCalled"`
	TextContentAssigned string      `json:"textContentAssigned"`
	Events              []evtRecord `json:"events"`
	InsertedChars       []string    `json:"insertedChars"`
	KeyEvents           []keyEvent  `json:"keyEvents"`
	EventLog            []string    `json:"eventLog"`
	SubmitClicks        int         `json:"submitClicks"`
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
    onMessage: {
      addListener() {},
    },
  },
};

global.MutationObserver = class {
  constructor(cb) {}
  observe() { }
  disconnect() {}
};

global.document = {
  body: {},
  querySelector: (selector) => {
    if (selector === "div.cover-title") {
      return { textContent: "SIAT" };
    }
    return null;
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

	if !containsLog(result.Logs, "[aibbe] Handshake sent: target=SIAT") {
		t.Fatalf("expected content.js runtime logs to include handshake confirmation, got %v", result.Logs)
	}
}

func TestExtensionContent_ProcessesGenerateCommand(t *testing.T) {
	t.Run("injects payload, observes mutations, and returns extracted code block on success", func(t *testing.T) {
		var result nodeResult
		runNodeJSON(t, `
const path = require("path");
const logs = [];
const sent = [];
const contentResponses = [];
const querySelectorCalls = [];
let onMessageListener = null;
let inputEvents = 0;
let buttonClicks = 0;
let observerCallback = null;
let observerDisconnected = false;
let observerConfig = null;

class FakeTextArea {
  constructor() {
    this._value = "";
  }

  dispatchEvent(event) {
    if (event.type === "input" && event.bubbles === true) {
      inputEvents += 1;
    }
    return true;
  }
}

Object.defineProperty(FakeTextArea.prototype, "value", {
  get() {
    return this._value;
  },
  set(next) {
    this._value = next;
  },
});

const input = new FakeTextArea();
const submitButton = {
  disabled: true,
  click() {
    buttonClicks += 1;
  },
};
const codeBlocks = [{ textContent: "print('hello')" }];
const responseText = {
  textContent: "fallback response",
  querySelectorAll(selector) {
    if (selector === "code, pre") {
      return codeBlocks;
    }
    return [];
  },
};
const responseContainer = {
  querySelector(selector) {
    if (selector === ".message-text-content") {
      return responseText;
    }
    if (selector === "thinking-animation, .thinking-message") {
      return null;
    }
    if (selector === ".message-actions, .actions-container, .xap-copy-to-clipboard, .suggestions-container, .follow-up-chip") {
      return {};
    }
    return null;
  },
};

global.Event = class FakeEvent {
  constructor(type, options = {}) {
    this.type = type;
    this.bubbles = options.bubbles === true;
  }
};

global.window = {
  HTMLTextAreaElement: FakeTextArea,
  requestAnimationFrame: (callback) => callback(),
  __AIBBE_SETTLE_MS: 0,
};

global.requestAnimationFrame = global.window.requestAnimationFrame;

global.MutationObserver = class FakeMutationObserver {
  constructor(callback) {
    observerCallback = callback;
  }

  observe(target, config) {
    observerConfig = config;
  }

  disconnect() {
    observerDisconnected = true;
  }
};

global.document = {
  querySelector(selector) {
    querySelectorCalls.push(selector);
    if (selector === 'textarea[aria-label="Query box"], textarea[aria-label="Cuadro de consulta"], textarea, div[contenteditable="true"]') {
      return input;
    }
    if (selector === 'button[aria-label="Submit"], button[aria-label="Enviar"], button[aria-label*="send"], button[type="submit"], button[data-testid*="send"]') {
      return submitButton;
    }
    if (selector === '.to-user-container, [data-testid*="response"], .response-container, .chat-response, .model-response') {
      return responseContainer;
    }
    return null;
  },
  querySelectorAll(selector) {
    querySelectorCalls.push(selector);
    if (selector === '.to-user-container, [data-testid*="response"], .response-container, .chat-response, .model-response') {
      return [responseContainer];
    }
    return [];
  },
};

global.console = {
  log: (...args) => logs.push(args.map((arg) => typeof arg === "string" ? arg : JSON.stringify(arg)).join(" ")),
  warn: () => {},
  error: () => {},
};

global.chrome = {
  runtime: {
    sendMessage: (message) => sent.push(message),
    onMessage: {
      addListener(fn) {
        onMessageListener = fn;
      },
    },
  },
};

require(path.resolve(process.cwd(), "extension/content.js"));

const sendResponse = (response) => contentResponses.push(response);
const returnValue = onMessageListener({ cmd: "generate", payload: "data" }, {}, sendResponse);

Promise.resolve(returnValue).then((resolved) => {
  setTimeout(() => {
    submitButton.disabled = false;
    observerCallback([{ type: "attributes", attributeName: "disabled" }]);
    setTimeout(() => {
      process.stdout.write(JSON.stringify({
        logs,
        sent,
        contentResponses,
        inputValue: input.value,
        inputEvents,
        buttonClicks,
        observerDisconnected,
        observerConfig,
        querySelectorCalls,
        listenerReturnedTrue: resolved === true,
      }));
    }, 0);
  }, 0);
});
`, &result)

		if len(result.ContentResponses) != 1 {
			t.Fatalf("content script sendResponse calls = %d, want 1 on successful response extraction", len(result.ContentResponses))
		}

		if got := result.ContentResponses[0]["status"]; got != "success" {
			t.Fatalf("content response status = %v, want success", got)
		}

		if got := result.ContentResponses[0]["result"]; got != "print('hello')" {
			t.Fatalf("content response result = %v, want print('hello')", got)
		}

		if got := result.InputValue; got != "data" {
			t.Fatalf("input value = %q, want data", got)
		}

		if result.InputEvents != 1 {
			t.Fatalf("input events = %d, want 1", result.InputEvents)
		}

		if result.ButtonClicks != 1 {
			t.Fatalf("button clicks = %d, want 1", result.ButtonClicks)
		}

		if !result.ObserverDisconnected {
			t.Fatal("expected MutationObserver to disconnect after extracting the response")
		}

		if got := result.ObserverConfig["childList"]; got != true {
			t.Fatalf("observer childList = %v, want true", got)
		}

		if got := result.ObserverConfig["subtree"]; got != true {
			t.Fatalf("observer subtree = %v, want true", got)
		}

		if got := result.ObserverConfig["attributes"]; got != true {
			t.Fatalf("observer attributes = %v, want true", got)
		}

		filter, ok := result.ObserverConfig["attributeFilter"].([]any)
		if !ok || len(filter) != 1 || filter[0] != "disabled" {
			t.Fatalf("observer attributeFilter = %v, want [disabled]", result.ObserverConfig["attributeFilter"])
		}

		if !contains(result.QuerySelectorCalls, `textarea[aria-label="Query box"], textarea[aria-label="Cuadro de consulta"], textarea, div[contenteditable="true"]`) {
			t.Fatalf("expected input selector query, got %v", result.QuerySelectorCalls)
		}

		if !contains(result.QuerySelectorCalls, `button[aria-label="Submit"], button[aria-label="Enviar"], button[aria-label*="send"], button[type="submit"], button[data-testid*="send"]`) {
			t.Fatalf("expected submit selector query, got %v", result.QuerySelectorCalls)
		}

		if !contains(result.QuerySelectorCalls, `.to-user-container, [data-testid*="response"], .response-container, .chat-response, .model-response`) {
			t.Fatalf("expected response container selector query, got %v", result.QuerySelectorCalls)
		}

		if !result.ListenerReturnedTrue {
			t.Fatal("content script onMessage listener must return true for async response support")
		}
	})

	t.Run("falls back to response container text when no code block exists", func(t *testing.T) {
		var result nodeResult
		runNodeJSON(t, `
const path = require("path");
const contentResponses = [];
const querySelectorCalls = [];
let onMessageListener = null;
let observerCallback = null;
let observerDisconnected = false;

class FakeTextArea {
  constructor() {
    this._value = "";
  }

  dispatchEvent() {
    return true;
  }
}

Object.defineProperty(FakeTextArea.prototype, "value", {
  get() {
    return this._value;
  },
  set(next) {
    this._value = next;
  },
});

const input = new FakeTextArea();
const submitButton = {
  disabled: true,
  click() {},
};
const responseText = {
  textContent: "plain text response",
  querySelectorAll(selector) {
    querySelectorCalls.push(selector);
    return [];
  },
};
const responseContainer = {
  querySelector(selector) {
    if (selector === ".message-text-content") {
      return responseText;
    }
    if (selector === "thinking-animation, .thinking-message") {
      return null;
    }
    if (selector === ".message-actions, .actions-container, .xap-copy-to-clipboard, .suggestions-container, .follow-up-chip") {
      return {};
    }
    return null;
  },
};

global.Event = class FakeEvent {
  constructor(type, options = {}) {
    this.type = type;
    this.bubbles = options.bubbles === true;
  }
};

global.window = {
  HTMLTextAreaElement: FakeTextArea,
  requestAnimationFrame: (callback) => callback(),
  __AIBBE_SETTLE_MS: 0,
};

global.requestAnimationFrame = global.window.requestAnimationFrame;

global.MutationObserver = class FakeMutationObserver {
  constructor(callback) {
    observerCallback = callback;
  }

  observe() {}

  disconnect() {
    observerDisconnected = true;
  }
};

global.document = {
  querySelector(selector) {
    if (selector === 'textarea[aria-label="Query box"], textarea[aria-label="Cuadro de consulta"], textarea, div[contenteditable="true"]') {
      return input;
    }
    if (selector === 'button[aria-label="Submit"], button[aria-label="Enviar"], button[aria-label*="send"], button[type="submit"], button[data-testid*="send"]') {
      return submitButton;
    }
    if (selector === '.to-user-container, [data-testid*="response"], .response-container, .chat-response, .model-response') {
      return responseContainer;
    }
    return null;
  },
  querySelectorAll(selector) {
    querySelectorCalls.push(selector);
    if (selector === '.to-user-container, [data-testid*="response"], .response-container, .chat-response, .model-response') {
      return [responseContainer];
    }
    return [];
  },
};

global.console = { log() {}, warn() {}, error() {} };
global.chrome = {
  runtime: {
    sendMessage() {},
    onMessage: {
      addListener(fn) {
        onMessageListener = fn;
      },
    },
  },
};

require(path.resolve(process.cwd(), "extension/content.js"));
const sendResponse = (response) => contentResponses.push(response);
const returnValue = onMessageListener({ cmd: "generate", payload: "data" }, {}, sendResponse);

Promise.resolve(returnValue).then((resolved) => {
  setTimeout(() => {
    submitButton.disabled = false;
    observerCallback([{ type: "attributes", attributeName: "disabled" }]);
    setTimeout(() => {
      process.stdout.write(JSON.stringify({
        contentResponses,
        observerDisconnected,
        listenerReturnedTrue: resolved === true,
      }));
    }, 0);
  }, 0);
});
`, &result)

		if len(result.ContentResponses) != 1 {
			t.Fatalf("content script sendResponse calls = %d, want 1", len(result.ContentResponses))
		}

		if got := result.ContentResponses[0]["result"]; got != "plain text response" {
			t.Fatalf("content response result = %v, want plain text response", got)
		}

		if !result.ObserverDisconnected {
			t.Fatal("expected MutationObserver to disconnect after fallback extraction")
		}

		if !result.ListenerReturnedTrue {
			t.Fatal("content script onMessage listener must return true when awaiting fallback extraction")
		}
	})

	t.Run("removes inline citation noise from final response text", func(t *testing.T) {
		var result nodeResult
		runNodeJSON(t, `
const path = require("path");
const contentResponses = [];
let onMessageListener = null;
let observerCallback = null;
let observerDisconnected = false;

class FakeTextArea {
  constructor() {
    this._value = "";
  }

  dispatchEvent() {
    return true;
  }
}

Object.defineProperty(FakeTextArea.prototype, "value", {
  get() {
    return this._value;
  },
  set(next) {
    this._value = next;
  },
});

const input = new FakeTextArea();
const submitButton = { disabled: true, click() {} };
const responseText = {
  textContent: "El SIAT es 1 more_horiz una plataforma.",
  innerText: "El SIAT es una plataforma.",
  cloneNode() {
    return {
      innerText: "El SIAT es una plataforma.",
      textContent: "El SIAT es una plataforma.",
      querySelectorAll(selector) {
        if (selector === 'button.citation-marker, button.xap-inline-dialog, [dialoglabel=\"Detalles de la cita\"], .citation-marker, .xap-inline-dialog') {
          return [{ remove() {} }];
        }
        if (selector === "mat-icon") {
          return [{
            textContent: "more_horiz",
            getAttribute() { return ""; },
            remove() {},
          }];
        }
        return [];
      },
    };
  },
  querySelectorAll() {
    return [];
  },
};
const responseContainer = {
  querySelector(selector) {
    if (selector === ".message-text-content") return responseText;
    if (selector === "thinking-animation, .thinking-message") return null;
    if (selector === ".message-actions, .actions-container, .xap-copy-to-clipboard, .suggestions-container, .follow-up-chip") return {};
    return null;
  },
};

global.Event = class FakeEvent {
  constructor(type, options = {}) {
    this.type = type;
    this.bubbles = options.bubbles === true;
  }
};

global.window = {
  HTMLTextAreaElement: FakeTextArea,
  requestAnimationFrame: (callback) => callback(),
  __AIBBE_SETTLE_MS: 0,
};
global.requestAnimationFrame = global.window.requestAnimationFrame;

global.MutationObserver = class FakeMutationObserver {
  constructor(callback) { observerCallback = callback; }
  observe() {}
  disconnect() { observerDisconnected = true; }
};

global.document = {
  querySelector(selector) {
    if (selector === 'textarea[aria-label="Query box"], textarea[aria-label="Cuadro de consulta"], textarea, div[contenteditable="true"]') return input;
    if (selector === 'button[aria-label="Submit"], button[aria-label="Enviar"], button[aria-label*="send"], button[type="submit"], button[data-testid*="send"]') return submitButton;
    if (selector === '.to-user-container, [data-testid*="response"], .response-container, .chat-response, .model-response') return responseContainer;
    return null;
  },
  querySelectorAll(selector) {
    if (selector === '.to-user-container, [data-testid*="response"], .response-container, .chat-response, .model-response') return [responseContainer];
    return [];
  },
};

global.console = { log() {}, warn() {}, error() {} };
global.chrome = {
  runtime: {
    sendMessage() {},
    onMessage: { addListener(fn) { onMessageListener = fn; } },
  },
};

require(path.resolve(process.cwd(), "extension/content.js"));
const sendResponse = (response) => contentResponses.push(response);
const returnValue = onMessageListener({ cmd: "generate", payload: "data" }, {}, sendResponse);

Promise.resolve(returnValue).then((resolved) => {
  setTimeout(() => {
    submitButton.disabled = false;
    observerCallback([{ type: "childList" }]);
    setTimeout(() => {
      process.stdout.write(JSON.stringify({
        contentResponses,
        observerDisconnected,
        listenerReturnedTrue: resolved === true,
      }));
    }, 0);
  }, 0);
});
`, &result)

		if len(result.ContentResponses) != 1 {
			t.Fatalf("content script sendResponse calls = %d, want 1", len(result.ContentResponses))
		}

		if got := result.ContentResponses[0]["result"]; got != "El SIAT es una plataforma." {
			t.Fatalf("content response result = %v, want cleaned text without citation noise", got)
		}

		if !result.ObserverDisconnected {
			t.Fatal("expected MutationObserver to disconnect after cleaned extraction")
		}

		if !result.ListenerReturnedTrue {
			t.Fatal("content script onMessage listener must return true when cleaning final response text")
		}
	})

	t.Run("waits for thinking markers to disappear and final actions to appear", func(t *testing.T) {
		var result nodeResult
		runNodeJSON(t, `
const path = require("path");
const contentResponses = [];
let onMessageListener = null;
let observerCallback = null;

class FakeTextArea {
  constructor() {
    this._value = "";
  }

  dispatchEvent() {
    return true;
  }
}

Object.defineProperty(FakeTextArea.prototype, "value", {
  get() {
    return this._value;
  },
  set(next) {
    this._value = next;
  },
});

const input = new FakeTextArea();
const submitButton = {
  disabled: true,
  click() {},
};
let thinkingVisible = true;
let readyVisible = false;
const responseText = {
  textContent: "Checking the scope...",
  querySelectorAll() {
    return [];
  },
};
const responseContainer = {
  querySelector(selector) {
    if (selector === ".message-text-content") {
      return responseText;
    }
    if (selector === "thinking-animation, .thinking-message") {
      return thinkingVisible ? {} : null;
    }
    if (selector === ".message-actions, .actions-container, .xap-copy-to-clipboard, .suggestions-container, .follow-up-chip") {
      return readyVisible ? {} : null;
    }
    return null;
  },
};

global.Event = class FakeEvent {
  constructor(type, options = {}) {
    this.type = type;
    this.bubbles = options.bubbles === true;
  }
};

global.window = {
  HTMLTextAreaElement: FakeTextArea,
  requestAnimationFrame: (callback) => callback(),
  __AIBBE_SETTLE_MS: 0,
};

global.requestAnimationFrame = global.window.requestAnimationFrame;

global.MutationObserver = class FakeMutationObserver {
  constructor(callback) {
    observerCallback = callback;
  }

  observe() {}
  disconnect() {}
};

global.document = {
  querySelector(selector) {
    if (selector === 'textarea[aria-label="Query box"], textarea[aria-label="Cuadro de consulta"], textarea, div[contenteditable="true"]') {
      return input;
    }
    if (selector === 'button[aria-label="Submit"], button[aria-label="Enviar"], button[aria-label*="send"], button[type="submit"], button[data-testid*="send"]') {
      return submitButton;
    }
    if (selector === '.to-user-container, [data-testid*="response"], .response-container, .chat-response, .model-response') {
      return responseContainer;
    }
    return null;
  },
  querySelectorAll(selector) {
    if (selector === '.to-user-container, [data-testid*="response"], .response-container, .chat-response, .model-response') {
      return [responseContainer];
    }
    return [];
  },
};

global.console = { log() {}, warn() {}, error() {} };
global.chrome = {
  runtime: {
    sendMessage() {},
    onMessage: {
      addListener(fn) {
        onMessageListener = fn;
      },
    },
  },
};

require(path.resolve(process.cwd(), "extension/content.js"));
const sendResponse = (response) => contentResponses.push(response);
const returnValue = onMessageListener({ cmd: "generate", payload: "data" }, {}, sendResponse);

Promise.resolve(returnValue).then((resolved) => {
  setTimeout(() => {
    observerCallback([{ type: "childList" }]);
    responseText.textContent = "Retrieving details...";
    observerCallback([{ type: "childList" }]);
    thinkingVisible = false;
    readyVisible = true;
    responseText.textContent = "final answer";
    observerCallback([{ type: "childList" }]);
    setTimeout(() => {
      process.stdout.write(JSON.stringify({
        contentResponses,
        listenerReturnedTrue: resolved === true,
      }));
    }, 0);
  }, 0);
});
`, &result)

		if len(result.ContentResponses) != 1 {
			t.Fatalf("content script sendResponse calls = %d, want 1", len(result.ContentResponses))
		}

		if got := result.ContentResponses[0]["result"]; got != "final answer" {
			t.Fatalf("content response result = %v, want final answer", got)
		}

		if !result.ListenerReturnedTrue {
			t.Fatal("content script onMessage listener must return true while waiting for final response markers")
		}
	})

	t.Run("returns input_not_found when notebook input is unavailable", func(t *testing.T) {
		var result nodeResult
		runNodeJSON(t, `
const path = require("path");
const logs = [];
const sent = [];
const contentResponses = [];
let onMessageListener = null;

global.Event = class FakeEvent {
  constructor(type, options = {}) {
    this.type = type;
    this.bubbles = options.bubbles === true;
  }
};

global.window = {
  HTMLTextAreaElement: function FakeTextArea() {},
  requestAnimationFrame: (callback) => callback(),
};

global.requestAnimationFrame = global.window.requestAnimationFrame;

global.document = {
  querySelector() {
    return null;
  },
  querySelectorAll(selector) {
    querySelectorCalls.push(selector);
    return [];
  },
};

global.console = {
  log: (...args) => logs.push(args.map((arg) => typeof arg === "string" ? arg : JSON.stringify(arg)).join(" ")),
  warn: () => {},
  error: () => {},
};

global.chrome = {
  runtime: {
    sendMessage: (message) => sent.push(message),
    onMessage: {
      addListener(fn) {
        onMessageListener = fn;
      },
    },
  },
};

require(path.resolve(process.cwd(), "extension/content.js"));

const sendResponse = (response) => contentResponses.push(response);
const returnValue = onMessageListener({ cmd: "generate", payload: "data" }, {}, sendResponse);

Promise.resolve(returnValue).then((resolved) => {
  setTimeout(() => {
    process.stdout.write(JSON.stringify({
      contentResponses,
      listenerReturnedTrue: resolved === true,
    }));
  }, 0);
});
`, &result)

		if len(result.ContentResponses) != 1 {
			t.Fatalf("content script sendResponse calls = %d, want 1", len(result.ContentResponses))
		}

		if got := result.ContentResponses[0]["status"]; got != "error" {
			t.Fatalf("content response status = %v, want error", got)
		}

		if got := result.ContentResponses[0]["error"]; got != "input_not_found" {
			t.Fatalf("content response error = %v, want input_not_found", got)
		}

		if !result.ListenerReturnedTrue {
			t.Fatal("content script onMessage listener must return true when input lookup fails")
		}
	})

	t.Run("returns input_not_found when notebook textarea is unavailable", func(t *testing.T) {
		var result nodeResult
		runNodeJSON(t, `
	const path = require("path");
	const contentResponses = [];
	const querySelectorCalls = [];
	let onMessageListener = null;
	let queryCount = 0;

class FakeTextArea {
  constructor() {
    this._value = "";
  }

  dispatchEvent() {
    return true;
  }
}

Object.defineProperty(FakeTextArea.prototype, "value", {
  get() {
    return this._value;
  },
  set(next) {
    this._value = next;
  },
});

const input = new FakeTextArea();

global.Event = class FakeEvent {
  constructor(type, options = {}) {
    this.type = type;
    this.bubbles = options.bubbles === true;
  }
};

global.window = {
  HTMLTextAreaElement: FakeTextArea,
  requestAnimationFrame: (callback) => callback(),
};

global.requestAnimationFrame = global.window.requestAnimationFrame;

global.document = {
  querySelector(selector) {
    if (selector === "div.cover-title") return null;
    queryCount += 1;
    if (queryCount === 1) {
      return input;
    }
    return null;
  },
  querySelectorAll(selector) {
    querySelectorCalls.push(selector);
    return [];
  },
};

global.console = {
  log() {},
  warn() {},
  error() {},
};

global.chrome = {
  runtime: {
    sendMessage() {},
    onMessage: {
      addListener(fn) {
        onMessageListener = fn;
      },
    },
  },
};

require(path.resolve(process.cwd(), "extension/content.js"));

const sendResponse = (response) => contentResponses.push(response);
const returnValue = onMessageListener({ cmd: "generate", payload: "data" }, {}, sendResponse);

Promise.resolve(returnValue).then((resolved) => {
  setTimeout(() => {
    process.stdout.write(JSON.stringify({
      contentResponses,
      listenerReturnedTrue: resolved === true,
    }));
  }, 0);
});
`, &result)

		if len(result.ContentResponses) != 1 {
			t.Fatalf("content script sendResponse calls = %d, want 1", len(result.ContentResponses))
		}

		if got := result.ContentResponses[0]["status"]; got != "error" {
			t.Fatalf("content response status = %v, want error", got)
		}

		if got := result.ContentResponses[0]["error"]; got != "submit_button_not_found" {
			t.Fatalf("content response error = %v, want submit_button_not_found", got)
		}

		if !result.ListenerReturnedTrue {
			t.Fatal("content script onMessage listener must return true when submit button lookup fails")
		}
	})

	t.Run("returns response_timeout when notebook response container never appears", func(t *testing.T) {
		var result nodeResult
		runNodeJSON(t, `
const path = require("path");
const contentResponses = [];
const querySelectorCalls = [];
let onMessageListener = null;

class FakeTextArea {
  constructor() {
    this._value = "";
  }

  dispatchEvent() {
    return true;
  }
}

Object.defineProperty(FakeTextArea.prototype, "value", {
  get() {
    return this._value;
  },
  set(next) {
    this._value = next;
  },
});

const input = new FakeTextArea();
const submitButton = {
  click() {},
};

global.Event = class FakeEvent {
  constructor(type, options = {}) {
    this.type = type;
    this.bubbles = options.bubbles === true;
  }
};

global.window = {
  HTMLTextAreaElement: FakeTextArea,
  requestAnimationFrame: (callback) => callback(),
  __AIBBE_TIMEOUT: 10,
};

global.requestAnimationFrame = global.window.requestAnimationFrame;

global.MutationObserver = class FakeMutationObserver {
  observe() {}
  disconnect() {}
};

global.document = {
  querySelector(selector) {
    if (selector === 'textarea[aria-label="Query box"], textarea[aria-label="Cuadro de consulta"], textarea, div[contenteditable="true"]') {
      return input;
    }
    if (selector === 'button[aria-label="Submit"], button[aria-label="Enviar"], button[aria-label*="send"], button[type="submit"], button[data-testid*="send"]') {
      return submitButton;
    }
    return null;
  },
  querySelectorAll(selector) {
    return [];
  },
};

global.console = { log() {}, warn() {}, error() {} };
global.chrome = {
  runtime: {
    sendMessage() {},
    onMessage: {
      addListener(fn) {
        onMessageListener = fn;
      },
    },
  },
};

require(path.resolve(process.cwd(), "extension/content.js"));
const sendResponse = (response) => {
  contentResponses.push(response);
  process.stdout.write(JSON.stringify({
    contentResponses,
    listenerReturnedTrue: true,
  }));
  process.exit(0);
};
onMessageListener({ cmd: "generate", payload: "data" }, {}, sendResponse);
`, &result)

		if len(result.ContentResponses) != 1 {
			t.Fatalf("content script sendResponse calls = %d, want 1", len(result.ContentResponses))
		}

		if got := result.ContentResponses[0]["status"]; got != "error" {
			t.Fatalf("content response status = %v, want error", got)
		}

		if got := result.ContentResponses[0]["error"]; got != "response_timeout" {
			t.Fatalf("content response error = %v, want response_timeout", got)
		}
	})

	t.Run("returns submit_button_not_found when observer setup cannot find submit button", func(t *testing.T) {
		var result nodeResult
		runNodeJSON(t, `
const path = require("path");
const contentResponses = [];
const querySelectorCalls = [];
let onMessageListener = null;
let queryCount = 0;

class FakeTextArea {
  constructor() {
    this._value = "";
  }

  dispatchEvent() {
    return true;
  }
}

Object.defineProperty(FakeTextArea.prototype, "value", {
  get() {
    return this._value;
  },
  set(next) {
    this._value = next;
  },
});

const input = new FakeTextArea();
const submitButton = {
  click() {},
};

global.Event = class FakeEvent {
  constructor(type, options = {}) {
    this.type = type;
    this.bubbles = options.bubbles === true;
  }
};

global.window = {
  HTMLTextAreaElement: FakeTextArea,
  requestAnimationFrame: (callback) => callback(),
  __AIBBE_TIMEOUT: 20,
};

global.requestAnimationFrame = global.window.requestAnimationFrame;
const realSetTimeout = global.setTimeout;

global.setTimeout = (callback, delay) => {
  if (delay !== 0) {
    pendingTimer = { callback, delay };
    return pendingTimer;
  }
  return realSetTimeout(callback, delay);
};

global.clearTimeout = (timer) => {
  if (pendingTimer === timer) {
    pendingTimer = null;
  }
};

global.MutationObserver = class FakeMutationObserver {
  observe() {}
  disconnect() {}
};

global.document = {
  querySelector(selector) {
    if (selector === "div.cover-title") return null;
    queryCount += 1;
    if (queryCount === 1 && selector === 'textarea[aria-label="Query box"], textarea[aria-label="Cuadro de consulta"], textarea, div[contenteditable="true"]') {
      return input;
    }
    if (queryCount === 2 && selector === 'button[aria-label="Submit"], button[aria-label="Enviar"], button[aria-label*="send"], button[type="submit"], button[data-testid*="send"]') {
      return submitButton;
    }
    if (queryCount === 3 && selector === 'button[aria-label="Submit"], button[aria-label="Enviar"], button[aria-label*="send"], button[type="submit"], button[data-testid*="send"]') {
      return null;
    }
    return null;
  },
  querySelectorAll(selector) {
    querySelectorCalls.push(selector);
    return [];
  },
};

global.console = { log() {}, warn() {}, error() {} };
global.chrome = {
  runtime: {
    sendMessage() {},
    onMessage: {
      addListener(fn) {
        onMessageListener = fn;
      },
    },
  },
};

require(path.resolve(process.cwd(), "extension/content.js"));
const sendResponse = (response) => contentResponses.push(response);
const returnValue = onMessageListener({ cmd: "generate", payload: "data" }, {}, sendResponse);

Promise.resolve(returnValue).then((resolved) => {
  setTimeout(() => {
    process.stdout.write(JSON.stringify({
      contentResponses,
      listenerReturnedTrue: resolved === true,
    }));
  }, 0);
});
`, &result)

		if len(result.ContentResponses) != 1 {
			t.Fatalf("content script sendResponse calls = %d, want 1", len(result.ContentResponses))
		}

		if got := result.ContentResponses[0]["status"]; got != "error" {
			t.Fatalf("content response status = %v, want error", got)
		}

		if got := result.ContentResponses[0]["error"]; got != "submit_button_not_found" {
			t.Fatalf("content response error = %v, want submit_button_not_found", got)
		}

		if !result.ListenerReturnedTrue {
			t.Fatal("content script onMessage listener must return true when observer setup cannot find submit button")
		}
	})

	t.Run("keeps waiting while final response action markers are absent", func(t *testing.T) {
		var result nodeResult
		runNodeJSON(t, `
const path = require("path");
const contentResponses = [];
const querySelectorCalls = [];
let onMessageListener = null;
let observerCallback = null;
let observerDisconnected = false;
let pendingTimer = null;

class FakeTextArea {
  constructor() {
    this._value = "";
  }

  dispatchEvent() {
    return true;
  }
}

Object.defineProperty(FakeTextArea.prototype, "value", {
  get() {
    return this._value;
  },
  set(next) {
    this._value = next;
  },
});

const input = new FakeTextArea();
const submitButton = {
  disabled: true,
  click() {},
};
const responseText = {
  textContent: "streaming",
  querySelectorAll() {
    return [{ textContent: "partial" }];
  },
};
const responseContainer = {
  querySelector(selector) {
    if (selector === ".message-text-content") {
      return responseText;
    }
    if (selector === "thinking-animation, .thinking-message") {
      return {};
    }
    if (selector === ".message-actions, .actions-container, .xap-copy-to-clipboard, .suggestions-container, .follow-up-chip") {
      return null;
    }
    return null;
  },
};

global.Event = class FakeEvent {
  constructor(type, options = {}) {
    this.type = type;
    this.bubbles = options.bubbles === true;
  }
};

global.window = {
  HTMLTextAreaElement: FakeTextArea,
  requestAnimationFrame: (callback) => callback(),
  __AIBBE_TIMEOUT: 20,
};

global.requestAnimationFrame = global.window.requestAnimationFrame;
const realSetTimeout = global.setTimeout;

global.setTimeout = (callback, delay) => {
  if (delay !== 0) {
    pendingTimer = { callback, delay };
    return pendingTimer;
  }
  return realSetTimeout(callback, delay);
};

global.clearTimeout = (timer) => {
  if (pendingTimer === timer) {
    pendingTimer = null;
  }
};

global.MutationObserver = class FakeMutationObserver {
  constructor(callback) {
    observerCallback = callback;
  }

  observe() {}

  disconnect() {
    observerDisconnected = true;
  }
};

global.document = {
  querySelector(selector) {
    if (selector === 'textarea[aria-label="Query box"], textarea[aria-label="Cuadro de consulta"], textarea, div[contenteditable="true"]') {
      return input;
    }
    if (selector === 'button[aria-label="Submit"], button[aria-label="Enviar"], button[aria-label*="send"], button[type="submit"], button[data-testid*="send"]') {
      return submitButton;
    }
    if (selector === '.to-user-container, [data-testid*="response"], .response-container, .chat-response, .model-response') {
      return responseContainer;
    }
    return null;
  },
  querySelectorAll(selector) {
    querySelectorCalls.push(selector);
    if (selector === '.to-user-container, [data-testid*="response"], .response-container, .chat-response, .model-response') {
      return [responseContainer];
    }
    return [];
  },
};

global.console = { log() {}, warn() {}, error() {} };
global.chrome = {
  runtime: {
    sendMessage() {},
    onMessage: {
      addListener(fn) {
        onMessageListener = fn;
      },
    },
  },
};

require(path.resolve(process.cwd(), "extension/content.js"));
const sendResponse = (response) => contentResponses.push(response);
const returnValue = onMessageListener({ cmd: "generate", payload: "data" }, {}, sendResponse);

Promise.resolve(returnValue).then((resolved) => {
  setTimeout(() => {
    observerCallback([{ type: "childList" }]);
    process.stdout.write(JSON.stringify({
      contentResponses,
      observerDisconnected,
      settleTimerPending: pendingTimer !== null,
      listenerReturnedTrue: resolved === true,
    }));
  }, 0);
});
`, &result)

		if len(result.ContentResponses) != 0 {
			t.Fatalf("content script sendResponse calls = %d, want 0 while final action markers are absent", len(result.ContentResponses))
		}

		if result.ObserverDisconnected {
			t.Fatal("observer must remain connected while final action markers are absent")
		}

		if !result.SettleTimerPending {
			t.Fatal("expected timeout watcher to remain pending while final action markers are absent")
		}

		if !result.ListenerReturnedTrue {
			t.Fatal("content script onMessage listener must return true while observer is still waiting")
		}
	})
}

func TestExtensionContent_SetInputValue_ExecCommandPath(t *testing.T) {
	var result nodeHandshakeResult
	runNodeJSON(t, `
const path = require("path");
const execCalls = [];
let setterCalled = false;

class FakeInput {
  constructor() {}
  focus() {}
  dispatchEvent() { return true; }
  // Without select() — force sub-path execCommand("selectAll")
}

global.window = {
  requestAnimationFrame: (cb) => cb(),
  __AIBBE_SETTLE_MS: 0,
};
global.requestAnimationFrame = (cb) => cb();

const input = new FakeInput();
const responseText = {
  textContent: "ok",
  innerText: "ok",
  cloneNode() { return this; },
  querySelectorAll() { return []; },
};
const responseContainer = {
  querySelector(sel) {
    if (sel.includes('message-text-content')) return responseText;
    if (sel.includes('thinking')) return null;
    if (sel.includes('message-actions')) return {};
    return null;
  },
};

let observerCallback = null;
global.MutationObserver = class {
  constructor(cb) { observerCallback = cb; }
  observe() {}
  disconnect() {}
};

global.Event = class { constructor(type, opts={}) { this.type=type; this.bubbles=opts.bubbles??false; } };
global.console = { log(){}, warn(){}, error(){} };

global.document = {
  execCommand(cmd, _, val) {
    execCalls.push({ cmd, val: val ?? null });
    return true;
  },
  querySelector(selector) {
    if (selector.includes('textarea') || selector.includes('contenteditable')) return input;
    if (selector.includes('button')) return { click() {} };
    return null;
  },
  querySelectorAll(selector) {
    if (selector.includes('to-user-container')) return [responseContainer];
    return [];
  },
};

let onMessageListener = null;
const contentResponses = [];
global.chrome = {
  runtime: {
    sendMessage() {},
    onMessage: { addListener(fn) { onMessageListener = fn; } },
  },
};

require(path.resolve(process.cwd(), "extension/content.js"));
const sendResponse = (r) => {
  contentResponses.push(r);
  process.stdout.write(JSON.stringify({
    execCalls,
    setterCalled,
    contentResponses,
  }));
  process.exit(0);
};
onMessageListener({ cmd: "generate", payload: "hello" }, {}, sendResponse);

// Trigger observer repeatedly until it resolves (or timeout)
const interval = setInterval(() => {
  if (observerCallback) {
    observerCallback([{ type: "childList" }]);
    clearInterval(interval);
  }
}, 10);
`, &result)

	// 1. execCommand("insertText", "hello") was called
	foundInsert := false
	for _, c := range result.ExecCalls {
		if c.Cmd == "insertText" && fmt.Sprint(c.Val) == "hello" {
			foundInsert = true
			break
		}
	}
	if !foundInsert {
		t.Fatalf("expected execCommand insertText with payload 'hello', got %v", result.ExecCalls)
	}

	// 2. execCommand("selectAll") was called (because FakeInput has no select())
	foundSelectAll := false
	for _, c := range result.ExecCalls {
		if c.Cmd == "selectAll" {
			foundSelectAll = true
			break
		}
	}
	if !foundSelectAll {
		t.Fatalf("expected execCommand selectAll, got %v", result.ExecCalls)
	}

	// 3. native setter was NOT invoked
	if result.SetterCalled {
		t.Fatal("expected native setter NOT called when execCommand is available")
	}
}

func TestExtensionContent_SetInputValue_ContenteditablePath(t *testing.T) {
	var result nodeHandshakeResult
	runNodeJSON(t, `
const path = require("path");
const events = [];
let textContentValue = "";

const fakeDiv = {
  getAttribute(name) { return name === "contenteditable" ? "true" : null; },
  focus() {},
  dispatchEvent(e) { events.push({ type: e.type, bubbles: e.bubbles }); return true; },
  get textContent() { return textContentValue; },
  set textContent(v) { textContentValue = v; },
};

global.window = {
  requestAnimationFrame: (cb) => cb(),
  __AIBBE_SETTLE_MS: 0,
  HTMLTextAreaElement: class {},
};
global.requestAnimationFrame = (cb) => cb();

const responseText = {
  textContent: "ok",
  innerText: "ok",
  cloneNode() { return this; },
  querySelectorAll() { return []; },
};
const responseContainer = {
  querySelector(sel) {
    if (sel.includes('message-text-content')) return responseText;
    if (sel.includes('thinking')) return null;
    if (sel.includes('message-actions')) return {};
    return null;
  },
};

let observerCallback = null;
global.MutationObserver = class {
  constructor(cb) { observerCallback = cb; }
  observe() {}
  disconnect() {}
};

global.Event = class { constructor(type, opts={}) { this.type=type; this.bubbles=opts.bubbles??false; } };
global.console = { log(){}, warn(){}, error(){} };

global.document = {
  querySelector(selector) {
    if (selector.includes('textarea') || selector.includes('contenteditable')) return fakeDiv;
    if (selector.includes('button')) return { click() {} };
    return null;
  },
  querySelectorAll(selector) {
    if (selector.includes('to-user-container')) return [responseContainer];
    return [];
  },
};

let onMessageListener = null;
const contentResponses = [];
global.chrome = {
  runtime: {
    sendMessage() {},
    onMessage: { addListener(fn) { onMessageListener = fn; } },
  },
};

require(path.resolve(process.cwd(), "extension/content.js"));
const sendResponse = (r) => {
  contentResponses.push(r);
  process.stdout.write(JSON.stringify({
    textContentAssigned: textContentValue,
    events,
    contentResponses,
  }));
  process.exit(0);
};
onMessageListener({ cmd: "generate", payload: "hello" }, {}, sendResponse);

// Trigger observer repeatedly until it resolves (or timeout)
const interval = setInterval(() => {
  if (observerCallback) {
    observerCallback([{ type: "childList" }]);
    clearInterval(interval);
  }
}, 10);
`, &result)

	// 1. textContent was assigned with the payload
	if result.TextContentAssigned != "hello" {
		t.Fatalf("textContent = %q, want hello", result.TextContentAssigned)
	}

	// 2. Event("input", { bubbles: true }) was dispatched
	found := false
	for _, ev := range result.Events {
		if ev.Type == "input" && ev.Bubbles {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected bubbling input event dispatched after textContent assignment")
	}
}

func TestExtensionContent_HumanTyping_InsertsCharsOneByOne(t *testing.T) {
	var result nodeHandshakeResult
	runNodeJSON(t, `
const path = require("path");
const insertedChars = [];
let submitClicks = 0;

global.KeyboardEvent = class {
  constructor(type, init = {}) {
    this.type = type; this.key = init.key; this.bubbles = init.bubbles ?? false;
  }
};

const fakeInput = { focus() {}, dispatchEvent() { return true; } };
const responseText = {
  textContent: "ok",
  innerText: "ok",
  cloneNode() { return this; },
  querySelectorAll() { return []; },
};
const responseContainer = {
  querySelector(sel) {
    if (sel.includes('message-text-content')) return responseText;
    if (sel.includes('thinking')) return null;
    if (sel.includes('message-actions')) return {};
    return null;
  },
};

let observerCallback = null;
global.MutationObserver = class {
  constructor(cb) { observerCallback = cb; }
  observe() {}
  disconnect() {}
};

global.Event = class { constructor(type, opts={}) { this.type=type; this.bubbles=opts.bubbles??false; } };
global.console = { log(){}, warn(){}, error(){} };

global.document = {
  execCommand(cmd, _, val) {
    if (cmd === "insertText") insertedChars.push(val);
    return true;
  },
  querySelector(selector) {
    if (selector.includes('textarea') || selector.includes('contenteditable')) return fakeInput;
    if (selector.includes('button')) return { click() { submitClicks++; } };
    return null;
  },
  querySelectorAll(selector) {
    if (selector.includes('to-user-container')) return [responseContainer];
    return [];
  },
};

global.window = {
  requestAnimationFrame: (cb) => cb(),
  __AIBBE_SETTLE_MS: 0,
  __AIBBE_HUMAN_TYPING: true,
  __AIBBE_JITTER_RANGE: [0, 0],
  __AIBBE_SUBMIT_DELAY_RANGE: [0, 0],
};
global.requestAnimationFrame = (cb) => cb();

let onMessageListener = null;
const contentResponses = [];
global.chrome = {
  runtime: {
    sendMessage() {},
    onMessage: { addListener(fn) { onMessageListener = fn; } },
  },
};

require(path.resolve(process.cwd(), "extension/content.js"));
const sendResponse = (r) => {
  contentResponses.push(r);
  process.stdout.write(JSON.stringify({
    insertedChars,
    submitClicks,
    contentResponses,
  }));
  process.exit(0);
};
onMessageListener({ cmd: "generate", payload: "hi" }, {}, sendResponse);

// Trigger observer repeatedly until it resolves (or timeout)
const interval = setInterval(() => {
  if (observerCallback) {
    observerCallback([{ type: "childList" }]);
    clearInterval(interval);
  }
}, 10);
`, &result)

	wantChars := []string{"h", "i"}
	if len(result.InsertedChars) != len(wantChars) {
		t.Fatalf("insertedChars = %v, want %v", result.InsertedChars, wantChars)
	}
	for i, want := range wantChars {
		if result.InsertedChars[i] != want {
			t.Fatalf("insertedChars[%d] = %q, want %q", i, result.InsertedChars[i], want)
		}
	}
	if result.SubmitClicks != 1 {
		t.Fatalf("submitClicks = %d, want 1", result.SubmitClicks)
	}
	if got := result.ContentResponses[0]["status"]; got != "success" {
		t.Fatalf("response status = %v, want success", got)
	}
}

func TestExtensionContent_HumanTyping_DispatchesKeyboardEventsPerChar(t *testing.T) {
	var result nodeHandshakeResult
	runNodeJSON(t, `
const path = require("path");
const keyEvents = [];

global.KeyboardEvent = class {
  constructor(type, init = {}) {
    keyEvents.push({ type, key: init.key });
    this.type = type; this.key = init.key; this.bubbles = init.bubbles ?? false;
  }
};

const fakeInput = { focus() {}, dispatchEvent() { return true; } };
const responseText = {
  textContent: "ok",
  innerText: "ok",
  cloneNode() { return this; },
  querySelectorAll() { return []; },
};
const responseContainer = {
  querySelector(sel) {
    if (sel.includes('message-text-content')) return responseText;
    if (sel.includes('thinking')) return null;
    if (sel.includes('message-actions')) return {};
    return null;
  },
};

let observerCallback = null;
global.MutationObserver = class {
  constructor(cb) { observerCallback = cb; }
  observe() {}
  disconnect() {}
};

global.Event = class { constructor(type, opts={}) { this.type=type; this.bubbles=opts.bubbles??false; } };
global.console = { log(){}, warn(){}, error(){} };

global.document = {
  execCommand() { return true; },
  querySelector(selector) {
    if (selector.includes('textarea') || selector.includes('contenteditable')) return fakeInput;
    if (selector.includes('button')) return { click() {} };
    return null;
  },
  querySelectorAll(selector) {
    if (selector.includes('to-user-container')) return [responseContainer];
    return [];
  },
};

global.window = {
  requestAnimationFrame: (cb) => cb(),
  __AIBBE_SETTLE_MS: 0,
  __AIBBE_HUMAN_TYPING: true,
  __AIBBE_JITTER_RANGE: [0, 0],
  __AIBBE_SUBMIT_DELAY_RANGE: [0, 0],
};
global.requestAnimationFrame = (cb) => cb();

let onMessageListener = null;
const contentResponses = [];
global.chrome = {
  runtime: {
    sendMessage() {},
    onMessage: { addListener(fn) { onMessageListener = fn; } },
  },
};

require(path.resolve(process.cwd(), "extension/content.js"));
const sendResponse = (r) => {
  contentResponses.push(r);
  process.stdout.write(JSON.stringify({
    keyEvents,
    contentResponses,
  }));
  process.exit(0);
};
onMessageListener({ cmd: "generate", payload: "ab" }, {}, sendResponse);

// Trigger observer repeatedly until it resolves (or timeout)
const interval = setInterval(() => {
  if (observerCallback) {
    observerCallback([{ type: "childList" }]);
    clearInterval(interval);
  }
}, 10);
`, &result)

	wantEvents := []struct{ Type, Key string }{
		{"keydown", "a"}, {"keypress", "a"}, {"keyup", "a"},
		{"keydown", "b"}, {"keypress", "b"}, {"keyup", "b"},
	}
	if len(result.KeyEvents) != len(wantEvents) {
		t.Fatalf("keyEvents count = %d, want %d\ngot: %v", len(result.KeyEvents), len(wantEvents), result.KeyEvents)
	}
	for i, want := range wantEvents {
		got := result.KeyEvents[i]
		if got.Type != want.Type || got.Key != want.Key {
			t.Fatalf("keyEvents[%d] = {%s %s}, want {%s %s}", i, got.Type, got.Key, want.Type, want.Key)
		}
	}
}

func TestExtensionContent_HumanTyping_SleepsBeforeSubmit(t *testing.T) {
	var result nodeHandshakeResult
	runNodeJSON(t, `
const path = require("path");
const eventLog = [];

global.KeyboardEvent = class {
  constructor(type, init = {}) {
    this.type = type; this.key = init.key; this.bubbles = init.bubbles ?? false;
  }
};

const fakeInput = { focus() {}, dispatchEvent() { return true; } };
const responseText = {
  textContent: "ok",
  innerText: "ok",
  cloneNode() { return this; },
  querySelectorAll() { return []; },
};
const responseContainer = {
  querySelector(sel) {
    if (sel.includes('message-text-content')) return responseText;
    if (sel.includes('thinking')) return null;
    if (sel.includes('message-actions')) return {};
    return null;
  },
};

let observerCallback = null;
global.MutationObserver = class {
  constructor(cb) { observerCallback = cb; }
  observe() {}
  disconnect() {}
};

global.Event = class { constructor(type, opts={}) { this.type=type; this.bubbles=opts.bubbles??false; } };
global.console = { log(){}, warn(){}, error(){} };

global.document = {
  execCommand() { return true; },
  querySelector(selector) {
    if (selector.includes('textarea') || selector.includes('contenteditable')) return fakeInput;
    if (selector.includes('button')) return { click() { eventLog.push("submit_click"); } };
    return null;
  },
  querySelectorAll(selector) {
    if (selector.includes('to-user-container')) return [responseContainer];
    return [];
  },
};

global.window = {
  requestAnimationFrame: (cb) => cb(),
  __AIBBE_SETTLE_MS: 0,
  __AIBBE_HUMAN_TYPING: true,
  __AIBBE_JITTER_RANGE: [0, 0],
  __AIBBE_SUBMIT_DELAY_RANGE: [50, 50],
};
global.requestAnimationFrame = (cb) => cb();

const realSetTimeout = global.setTimeout;
global.setTimeout = (cb, ms) => {
  if (ms > 0) {
    eventLog.push("sleep:" + ms);
    return realSetTimeout(cb, 0); // execute immediately for test speed
  }
  return realSetTimeout(cb, ms);
};

let onMessageListener = null;
const contentResponses = [];
global.chrome = {
  runtime: {
    sendMessage() {},
    onMessage: { addListener(fn) { onMessageListener = fn; } },
  },
};

require(path.resolve(process.cwd(), "extension/content.js"));
const sendResponse = (r) => {
  contentResponses.push(r);
  process.stdout.write(JSON.stringify({
    eventLog,
    contentResponses,
  }));
  process.exit(0);
};
onMessageListener({ cmd: "generate", payload: "hi" }, {}, sendResponse);

// Trigger observer repeatedly until it resolves (or timeout)
const interval = setInterval(() => {
  if (observerCallback) {
    observerCallback([{ type: "childList" }]);
    clearInterval(interval);
  }
}, 10);
`, &result)

	sleepIdx, clickIdx := -1, -1
	for i, ev := range result.EventLog {
		if strings.HasPrefix(ev, "sleep:") && sleepIdx == -1 {
			sleepIdx = i
		}
		if ev == "submit_click" && clickIdx == -1 {
			clickIdx = i
		}
	}
	if sleepIdx == -1 {
		t.Fatalf("expected sleep before submit in event log: %v", result.EventLog)
	}
	if clickIdx == -1 {
		t.Fatalf("expected submit_click in event log: %v", result.EventLog)
	}
	if sleepIdx >= clickIdx {
		t.Fatalf("sleep must occur BEFORE submit_click: sleepIdx=%d, clickIdx=%d, log=%v", sleepIdx, clickIdx, result.EventLog)
	}
}

func TestExtensionContent_HumanTyping_DisabledPreservesExistingBehavior(t *testing.T) {
	var result nodeHandshakeResult
	runNodeJSON(t, `
const path = require("path");
const keyEvents = [];
let buttonClicks = 0;

global.KeyboardEvent = class {
  constructor(type, init = {}) { keyEvents.push({ type, key: init.key }); }
};

class FakeTextArea {
  constructor() { this._value = ""; }
  dispatchEvent() { return true; }
  focus() {}
}
Object.defineProperty(FakeTextArea.prototype, "value", {
  get() { return this._value; },
  set(next) { this._value = next; },
});

const input = new FakeTextArea();
const submitButton = {
  disabled: false,
  click() { buttonClicks++; },
};

const responseText = {
  textContent: "ok",
  innerText: "ok",
  cloneNode() { return this; },
  querySelectorAll() { return []; },
};
const responseContainer = {
  querySelector(sel) {
    if (sel.includes('message-text-content')) return responseText;
    if (sel.includes('thinking')) return null;
    if (sel.includes('message-actions')) return {};
    return null;
  },
};

let observerCallback = null;
global.MutationObserver = class {
  constructor(cb) { observerCallback = cb; }
  observe() {}
  disconnect() {}
};

global.Event = class { constructor(type, opts={}) { this.type=type; this.bubbles=opts.bubbles??false; } };
global.window = {
  HTMLTextAreaElement: FakeTextArea,
  requestAnimationFrame: (cb) => cb(),
  __AIBBE_SETTLE_MS: 0,
  // __AIBBE_HUMAN_TYPING not defined
};
global.requestAnimationFrame = (cb) => cb();

global.document = {
  querySelector(selector) {
    if (selector.includes('textarea') || selector.includes('contenteditable')) return input;
    if (selector.includes('button')) return submitButton;
    return null;
  },
  querySelectorAll(selector) {
    if (selector.includes('to-user-container')) return [responseContainer];
    return [];
  },
};

global.console = { log(){}, warn(){}, error(){} };

let onMessageListener = null;
const contentResponses = [];
global.chrome = {
  runtime: {
    sendMessage() {},
    onMessage: { addListener(fn) { onMessageListener = fn; } },
  },
};

require(path.resolve(process.cwd(), "extension/content.js"));
const sendResponse = (r) => {
  contentResponses.push(r);
  process.stdout.write(JSON.stringify({
    keyEvents,
    inputValue: input.value,
    buttonClicks,
    contentResponses,
  }));
  process.exit(0);
};
onMessageListener({ cmd: "generate", payload: "hello" }, {}, sendResponse);

// Trigger observer repeatedly until it resolves (or timeout)
const interval = setInterval(() => {
  if (observerCallback) {
    observerCallback([{ type: "childList" }]);
    clearInterval(interval);
  }
}, 10);
`, &result)

	// No KeyboardEvents
	if len(result.KeyEvents) != 0 {
		t.Fatalf("expected no KeyboardEvents when __AIBBE_HUMAN_TYPING is false, got %v", result.KeyEvents)
	}
	// Bulk insertion worked
	if result.InputValue != "hello" {
		t.Fatalf("inputValue = %q, want hello", result.InputValue)
	}
	// Button clicked
	if result.ButtonClicks != 1 {
		t.Fatalf("buttonClicks = %d, want 1", result.ButtonClicks)
	}
	// Successful response
	if got := result.ContentResponses[0]["status"]; got != "success" {
		t.Fatalf("status = %v, want success", got)
	}
}

func TestExtensionContent_DefinesSelectorCascadeCommentsAndConstants(t *testing.T) {
	source := readExtensionFile(t, "content.js")

	if !strings.Contains(source, "const SELECTORS = {") {
		t.Fatal("expected content.js to define SELECTORS constants")
	}

	if !strings.Contains(source, `INPUT: 'textarea[aria-label="Query box"], textarea[aria-label="Cuadro de consulta"], textarea, div[contenteditable="true"]'`) {
		t.Fatal("expected content.js to define SELECTORS.INPUT for textarea/contenteditable fallback")
	}

	if !strings.Contains(source, `SUBMIT_BUTTON: 'button[aria-label="Submit"], button[aria-label="Enviar"], button[aria-label*="send"], button[type="submit"], button[data-testid*="send"]'`) {
		t.Fatal("expected content.js to define SELECTORS.SUBMIT_BUTTON with resilient send button selectors")
	}

	if !strings.Contains(source, `RESPONSE_CONTAINER: '.to-user-container, [data-testid*="response"], .response-container, .chat-response, .model-response'`) {
		t.Fatal("expected content.js to define SELECTORS.RESPONSE_CONTAINER with resilient response container selectors")
	}

	if !strings.Contains(source, `RESPONSE_TEXT: '.message-text-content'`) {
		t.Fatal("expected content.js to define SELECTORS.RESPONSE_TEXT for scoped message extraction")
	}

	if !strings.Contains(source, `THINKING_MARKERS: 'thinking-animation, .thinking-message'`) {
		t.Fatal("expected content.js to define SELECTORS.THINKING_MARKERS for in-progress NotebookLM states")
	}

	if !strings.Contains(source, `RESPONSE_READY_MARKERS: '.message-actions, .actions-container, .xap-copy-to-clipboard, .suggestions-container, .follow-up-chip'`) {
		t.Fatal("expected content.js to define SELECTORS.RESPONSE_READY_MARKERS for final-response UI detection")
	}

	if !strings.Contains(source, `CITATION_NOISE: 'button.citation-marker, button.xap-inline-dialog, [dialoglabel="Detalles de la cita"], .citation-marker, .xap-inline-dialog'`) {
		t.Fatal("expected content.js to define SELECTORS.CITATION_NOISE for inline citation cleanup")
	}

	if !strings.Contains(source, `CODE_BLOCK: 'code, pre'`) {
		t.Fatal("expected content.js to define SELECTORS.CODE_BLOCK for code extraction")
	}

	if !strings.Contains(source, "Selector cascade: prefer NotebookLM's textarea") {
		t.Fatal("expected content.js to document the selector cascade for future maintainers")
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
