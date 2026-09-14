package aibbe

import (
	"regexp"
	"strings"
	"testing"
)

type generateWaitResult struct {
	Response        map[string]any `json:"response"`
	ActiveIntervals int            `json:"activeIntervals"`
	ActiveTimeouts  int            `json:"activeTimeouts"`
	PollInvocations int            `json:"pollInvocations"`
	ResponseCount   int            `json:"responseCount"`
}

const generateWaitPreamble = `
const path = require("path");

let activeIntervals = 0;
let activeTimeouts = 0;
let pollInvocations = 0;
const _setInterval = global.setInterval;
const _clearInterval = global.clearInterval;
const _setTimeout = global.setTimeout;
const _clearTimeout = global.clearTimeout;
const _pendingTimeouts = new Set();

global.setInterval = function(fn, ms) { activeIntervals++; return _setInterval(function() { pollInvocations++; fn(); }, ms); };
global.clearInterval = function(id) { activeIntervals--; return _clearInterval(id); };
// A timeout is counted exactly once: on fire (deleted from _pendingTimeouts before
// decrementing) or on clearTimeout — never both, so cleanup after a natural fire
// cannot double-decrement.
global.setTimeout = function(fn, ms) {
  activeTimeouts++;
  const id = _setTimeout(function() { if (_pendingTimeouts.delete(id)) activeTimeouts--; fn(); }, ms);
  _pendingTimeouts.add(id);
  return id;
};
global.clearTimeout = function(id) { if (_pendingTimeouts.delete(id)) activeTimeouts--; return _clearTimeout(id); };

function makeTextNode(text) {
  return {
    tagName: "SPAN",
    textContent: text,
    innerText: text,
    cloneNode() { return makeTextNode(text); },
    querySelectorAll() { return []; },
    querySelector() { return null; },
    remove() {},
    getAttribute() { return null; },
  };
}

function classMatcher(cfg) {
  return function(sel) {
    const parts = sel.split(",").map(function(s) { return s.trim(); });
    for (let i = 0; i < parts.length; i++) {
      const tokens = parts[i].split(".");
      const cls = tokens[tokens.length - 1];
      if (cls && cfg.classes.indexOf(cls) >= 0) return { _class: cls };
    }
    return null;
  };
}

function makeScope(cfg) {
  return {
    tagName: "CHAT-MESSAGE",
    querySelector: classMatcher(cfg),
    querySelectorAll() { return []; },
  };
}

function makeContainer(text, scope) {
  return {
    tagName: "MAT-CARD-CONTENT",
    querySelector(sel) {
      if (sel.indexOf("message-content") >= 0 || sel.indexOf("text") >= 0) return makeTextNode(text);
      return null;
    },
    querySelectorAll() { return []; },
    closest(sel) { return sel === "chat-message" ? scope : null; },
    parentElement: scope,
  };
}

global.Event = class { constructor(t) { this.type = t; } };
global.window = {
  requestAnimationFrame: (cb) => cb(),
  __AIBBE_POLL_MS: 5,
  __AIBBE_STABLE_TICKS: 2,
  __AIBBE_HARD_STABLE_TICKS: 6,
  __AIBBE_TIMEOUT: 300,
  HTMLTextAreaElement: { prototype: { value: "" } }
};
global.requestAnimationFrame = global.window.requestAnimationFrame;
global.console = { log() {}, warn() {}, error() {} };
global.chrome = {
  runtime: { sendMessage: () => {}, onMessage: { addListener(fn) { global.__onMessage = fn; } } },
  storage: { local: { get: () => Promise.resolve({}) } }
};
`

func runGenerateWait(t *testing.T, script string, result *generateWaitResult) {
	t.Helper()
	runNodeJSON(t, generateWaitPreamble+script, result)
}

func TestGenerateWait_ResolvesUnderContinuousMutations(t *testing.T) {
	var result generateWaitResult
	runGenerateWait(t, `
const scope = makeScope({ classes: ["message-actions"] });
const container = makeContainer("hello world", scope);

const input = { value: "", focus() {}, getAttribute() { return null; }, dispatchEvent() { return true; } };
const submitButton = { disabled: true, click() {
  global.document.querySelectorAll = (s) => (s.indexOf("content") >= 0 ? [container] : []);
} };

global.MutationObserver = class { constructor(cb) { global.__obsCb = cb; } observe() {} disconnect() {} };
global.document = {
  execCommand() { return true; },
  querySelector(s) {
    if (s.indexOf("textarea") >= 0) return input;
    if (s.indexOf("button") >= 0) return submitButton;
    return null;
  },
  querySelectorAll() { return []; },
  body: { appendChild: () => {} }
};

require(path.resolve(process.cwd(), "extension/content.js"));

setTimeout(() => {
  const storm = setInterval(() => { if (global.__obsCb) global.__obsCb([{ type: "attributes" }]); }, 3);
  global.__onMessage({ cmd: "generate", payload: "test" }, {}, (r) => {
    clearInterval(storm);
    process.stdout.write(JSON.stringify({ response: r, activeIntervals, activeTimeouts }));
    process.exit(0);
  });
}, 20);
`, &result)

	if result.Response == nil {
		t.Fatalf("no response captured")
	}
	if result.Response["status"] != "success" {
		t.Fatalf("expected success, got %v", result.Response)
	}
	if result.Response["result"] != "hello world" {
		t.Fatalf("expected 'hello world', got %v", result.Response["result"])
	}
	if result.ActiveIntervals != 0 {
		t.Fatalf("interval leak from content.js: activeIntervals=%d", result.ActiveIntervals)
	}
	if result.ActiveTimeouts != 0 {
		t.Fatalf("timeout leak from content.js: activeTimeouts=%d", result.ActiveTimeouts)
	}
}

func TestGenerateWait_FalsePositiveThinkingNodesDoNotBlock(t *testing.T) {
	var result generateWaitResult
	runGenerateWait(t, `
const scope = makeScope({ classes: ["message-actions", "rethinking-panel"] });
const container = makeContainer("response text", scope);

const input = { value: "", focus() {}, getAttribute() { return null; }, dispatchEvent() { return true; } };
const submitButton = { disabled: true, click() {
  global.document.querySelectorAll = (s) => (s.indexOf("content") >= 0 ? [container] : []);
} };

global.MutationObserver = class { constructor() {} observe() {} disconnect() {} };
global.document = {
  execCommand() { return true; },
  querySelector(s) {
    if (s.indexOf("textarea") >= 0) return input;
    if (s.indexOf("button") >= 0) return submitButton;
    return null;
  },
  querySelectorAll() { return []; },
  body: { appendChild: () => {} }
};

require(path.resolve(process.cwd(), "extension/content.js"));

setTimeout(() => {
  global.__onMessage({ cmd: "generate", payload: "test" }, {}, (r) => {
    process.stdout.write(JSON.stringify({ response: r, activeIntervals, activeTimeouts }));
    process.exit(0);
  });
}, 20);
`, &result)

	if result.Response == nil {
		t.Fatalf("no response captured")
	}
	if result.Response["status"] != "success" {
		t.Fatalf("expected success (rethinking-panel must not match new THINKING_MARKERS), got %v", result.Response)
	}
	if result.Response["result"] != "response text" {
		t.Fatalf("expected 'response text', got %v", result.Response["result"])
	}
	if result.ActiveTimeouts != 0 {
		t.Fatalf("timeout leak from content.js: activeTimeouts=%d", result.ActiveTimeouts)
	}
}

func TestGenerateWait_IgnoresPreviousResponseBeforeBaseline(t *testing.T) {
	var result generateWaitResult
	runGenerateWait(t, `
const staleScope = makeScope({ classes: ["message-actions"] });
const staleContainer = makeContainer("stale answer", staleScope);
const newScope = makeScope({ classes: ["message-actions"] });
const newContainer = makeContainer("fresh answer", newScope);

// The new container appears asynchronously (well after click()), several poll
// ticks after baselineCount is captured, so the baseline gate is actually
// exercised: if it were missing, early ticks would resolve on the stale
// container instead of waiting for the fresh one.
let containers = [staleContainer];
const input = { value: "", focus() {}, getAttribute() { return null; }, dispatchEvent() { return true; } };
const submitButton = { disabled: true, click() {
  setTimeout(() => { containers = [staleContainer, newContainer]; }, 30);
} };

global.MutationObserver = class { constructor() {} observe() {} disconnect() {} };
global.document = {
  execCommand() { return true; },
  querySelector(s) {
    if (s.indexOf("textarea") >= 0) return input;
    if (s.indexOf("button") >= 0) return submitButton;
    return null;
  },
  querySelectorAll(s) {
    if (s.indexOf("content") >= 0) return containers;
    return [];
  },
  body: { appendChild: () => {} }
};

require(path.resolve(process.cwd(), "extension/content.js"));

setTimeout(() => {
  global.__onMessage({ cmd: "generate", payload: "test" }, {}, (r) => {
    process.stdout.write(JSON.stringify({ response: r, activeIntervals, activeTimeouts }));
    process.exit(0);
  });
}, 20);
`, &result)

	if result.Response == nil {
		t.Fatalf("no response captured")
	}
	if result.Response["status"] != "success" {
		t.Fatalf("expected success, got %v", result.Response)
	}
	if result.Response["result"] != "fresh answer" {
		t.Fatalf("expected 'fresh answer' (not stale), got %v", result.Response["result"])
	}
	if result.ActiveIntervals != 0 {
		t.Fatalf("interval leak: activeIntervals=%d", result.ActiveIntervals)
	}
	if result.ActiveTimeouts != 0 {
		t.Fatalf("timeout leak from content.js: activeTimeouts=%d", result.ActiveTimeouts)
	}
}

func TestGenerateWait_TimeoutCarriesDiagnostics(t *testing.T) {
	var result generateWaitResult
	runGenerateWait(t, `
const scope = makeScope({ classes: [] });
const container = makeContainer("partial text", scope);

const input = { value: "", focus() {}, getAttribute() { return null; }, dispatchEvent() { return true; } };
const submitButton = { disabled: true, click() {
  global.document.querySelectorAll = (s) => (s.indexOf("content") >= 0 ? [container] : []);
} };

global.MutationObserver = class { constructor() {} observe() {} disconnect() {} };
global.document = {
  execCommand() { return true; },
  querySelector(s) {
    if (s.indexOf("textarea") >= 0) return input;
    if (s.indexOf("button") >= 0) return submitButton;
    return null;
  },
  querySelectorAll() { return []; },
  body: { appendChild: () => {} }
};

require(path.resolve(process.cwd(), "extension/content.js"));

setTimeout(() => {
  global.__onMessage({ cmd: "generate", payload: "test" }, {}, (r) => {
    process.stdout.write(JSON.stringify({ response: r, activeIntervals, activeTimeouts }));
    process.exit(0);
  });
}, 20);
`, &result)

	if result.Response == nil {
		t.Fatalf("no response captured")
	}
	if result.Response["status"] != "error" || result.Response["error"] != "response_timeout" {
		t.Fatalf("expected response_timeout, got %v", result.Response)
	}
	detail, ok := result.Response["detail"].(map[string]any)
	if !ok {
		t.Fatalf("expected detail object, got %v", result.Response["detail"])
	}
	if detail["hasReadyMarkers"] != false {
		t.Fatalf("expected hasReadyMarkers=false, got %v", detail["hasReadyMarkers"])
	}
	if detail["hasThinkingMarkers"] != false {
		t.Fatalf("expected hasThinkingMarkers=false, got %v", detail["hasThinkingMarkers"])
	}
	if detail["resultLength"] == nil || detail["resultLength"].(float64) == 0 {
		t.Fatalf("expected non-zero resultLength, got %v", detail["resultLength"])
	}
	if detail["scopeTag"] == nil {
		t.Fatalf("expected scopeTag in detail, got %v", detail)
	}
	if detail["observerFires"] == nil {
		t.Fatalf("expected observerFires in detail, got %v", detail)
	}
	if detail["pollTicks"] == nil {
		t.Fatalf("expected pollTicks in detail, got %v", detail)
	}
	if result.ActiveIntervals != 0 {
		t.Fatalf("interval leak after timeout: activeIntervals=%d", result.ActiveIntervals)
	}
}

func TestGenerateWait_ThinkingMarkerDelaysThenResolves(t *testing.T) {
	var result generateWaitResult
	runGenerateWait(t, `
const scope = makeScope({ classes: ["message-actions", "thinking-animation"] });
const container = makeContainer("final answer", scope);

const input = { value: "", focus() {}, getAttribute() { return null; }, dispatchEvent() { return true; } };
const submitButton = { disabled: true, click() {
  global.document.querySelectorAll = (s) => (s.indexOf("content") >= 0 ? [container] : []);
} };

global.MutationObserver = class { constructor() {} observe() {} disconnect() {} };
global.document = {
  execCommand() { return true; },
  querySelector(s) {
    if (s.indexOf("textarea") >= 0) return input;
    if (s.indexOf("button") >= 0) return submitButton;
    return null;
  },
  querySelectorAll() { return []; },
  body: { appendChild: () => {} }
};

require(path.resolve(process.cwd(), "extension/content.js"));

setTimeout(() => {
  global.__onMessage({ cmd: "generate", payload: "test" }, {}, (r) => {
    process.stdout.write(JSON.stringify({ response: r, activeIntervals, activeTimeouts, pollInvocations }));
    process.exit(0);
  });
}, 20);
`, &result)

	if result.Response == nil {
		t.Fatalf("no response captured")
	}
	if result.Response["status"] != "success" {
		t.Fatalf("expected success (bounded veto), got %v", result.Response)
	}
	if result.Response["result"] != "final answer" {
		t.Fatalf("expected 'final answer', got %v", result.Response["result"])
	}
	// The permanent thinking marker blocks the fast path, so resolution can only
	// come from the hard veto: it must not fire before hardStableTicksNeeded (6)
	// poll ticks have elapsed. A prior version of this test only checked the
	// final result and would also pass if the veto were skipped entirely.
	const hardStableTicksNeeded = 6
	if result.PollInvocations < hardStableTicksNeeded {
		t.Fatalf("resolved too early: pollInvocations=%d, want >= %d (bounded veto not exercised)", result.PollInvocations, hardStableTicksNeeded)
	}
	if result.ActiveIntervals != 0 {
		t.Fatalf("interval leak: activeIntervals=%d", result.ActiveIntervals)
	}
	if result.ActiveTimeouts != 0 {
		t.Fatalf("timeout leak from content.js: activeTimeouts=%d", result.ActiveTimeouts)
	}
}

func TestGenerateWait_ExceptionDuringEvaluationYieldsSingleErrorNoLeak(t *testing.T) {
	var result generateWaitResult
	runGenerateWait(t, `
// Simulates a calibrated-invalid selector: messageScope.querySelector throws
// instead of returning null/an element, as a bad CSS selector would under a
// real querySelector call. This must fail fast through the finish() funnel,
// not leak the interval/observer/timeout or crash uncaught.
const throwingScope = {
  tagName: "CHAT-MESSAGE",
  querySelector() { throw new Error("boom_invalid_selector"); },
  querySelectorAll() { return []; }
};
const container = makeContainer("partial", throwingScope);

const input = { value: "", focus() {}, getAttribute() { return null; }, dispatchEvent() { return true; } };
const submitButton = { disabled: true, click() {
  global.document.querySelectorAll = (s) => (s.indexOf("content") >= 0 ? [container] : []);
} };

global.MutationObserver = class { constructor() {} observe() {} disconnect() {} };
global.document = {
  execCommand() { return true; },
  querySelector(s) {
    if (s.indexOf("textarea") >= 0) return input;
    if (s.indexOf("button") >= 0) return submitButton;
    return null;
  },
  querySelectorAll() { return []; },
  body: { appendChild: () => {} }
};

require(path.resolve(process.cwd(), "extension/content.js"));

let responseCount = 0;
setTimeout(() => {
  global.__onMessage({ cmd: "generate", payload: "test" }, {}, (r) => {
    responseCount++;
    // Give any (incorrect) second tick a chance to fire before asserting.
    setTimeout(() => {
      process.stdout.write(JSON.stringify({ response: r, activeIntervals, activeTimeouts, responseCount }));
      process.exit(0);
    }, 30);
  });
}, 20);
`, &result)

	if result.Response == nil {
		t.Fatalf("no response captured")
	}
	if result.Response["status"] != "error" {
		t.Fatalf("expected error response on evaluation exception, got %v", result.Response)
	}
	if result.ResponseCount != 1 {
		t.Fatalf("expected exactly one response, got responseCount=%d", result.ResponseCount)
	}
	if result.ActiveIntervals != 0 {
		t.Fatalf("interval leak after evaluation exception: activeIntervals=%d", result.ActiveIntervals)
	}
	if result.ActiveTimeouts != 0 {
		t.Fatalf("timeout leak after evaluation exception: activeTimeouts=%d", result.ActiveTimeouts)
	}
}

// TestContentJS_ThinkingMarkersDefaultHasNoSubstringMatcher is a static regression
// guard for the root cause of FIX-001: a THINKING_MARKERS default using a
// substring/prefix attribute matcher (e.g. [class*=...]) false-positives on
// unrelated classes (like "rethinking-panel") and blocks resolution forever.
// This must fail the build if the substring matcher is ever re-added.
func TestContentJS_ThinkingMarkersDefaultHasNoSubstringMatcher(t *testing.T) {
	source := readExtensionFile(t, "content.js")
	re := regexp.MustCompile(`THINKING_MARKERS:\s*'([^']*)'`)
	m := re.FindStringSubmatch(source)
	if m == nil {
		t.Fatalf("could not locate THINKING_MARKERS default in extension/content.js")
	}
	if strings.Contains(m[1], "[class*=") || strings.Contains(m[1], "[class^=") {
		t.Fatalf("THINKING_MARKERS default must not use a substring/prefix class matcher, got %q", m[1])
	}
}
