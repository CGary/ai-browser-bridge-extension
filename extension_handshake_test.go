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
  querySelector() {
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
