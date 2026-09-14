"use strict";

console.log("[aibbe] Content script version: 2026-04-20 (locale-agnostic defaults)");

const SELECTORS = {
  INPUT:                  'textarea.query-box-input',
  SUBMIT_BUTTON:          'button.submit-button',
  RESPONSE_CONTAINER:     'mat-card-content.to-user-message-inner-content',
  RESPONSE_TEXT:          '.message-content',
  RESPONSE_READY_MARKERS: 'mat-card-actions.message-actions',
  THINKING_MARKERS:       'thinking-animation, .loading-spinner',
  CITATION_NOISE:         'button.citation-marker, .xap-inline-dialog',
};

let activeSelectors = { ...SELECTORS };

async function loadSelectors() {
  try {
    if (typeof chrome === "undefined" || !chrome.storage || !chrome.storage.local) {
      activeSelectors = { ...SELECTORS };
      return;
    }
    const stored = await chrome.storage.local.get("aibbe_calibrations");
    const calibrations = stored.aibbe_calibrations ?? {};
    activeSelectors = { ...SELECTORS };
    for (const [key, value] of Object.entries(calibrations)) {
      if (key in SELECTORS) {
        activeSelectors[key] = String(value);
      }
    }
    console.log("[aibbe] Selectors loaded — calibrations:", Object.keys(calibrations));
  } catch (err) {
    console.warn("[aibbe] loadSelectors failed, using defaults:", err.message);
    activeSelectors = { ...SELECTORS };
  }
}

const TITLE_SELECTOR = "div.cover-title";

// The page <title> is "<notebook name> - <brand>". Brand observed as
// "Gemini Notebook" (notebook.google.com) and "NotebookLM" (legacy domain).
// Used as target source when TITLE_SELECTOR no longer matches the DOM.
const DOC_TITLE_BRAND = /\s+-\s+(Gemini Notebook|NotebookLM)$/;

function titleFromDocument() {
  const raw = document.title?.trim();
  if (!raw || !DOC_TITLE_BRAND.test(raw)) return null;
  const name = raw.replace(DOC_TITLE_BRAND, "").trim();
  return name || null;
}

function sendHandshake(target) {
  chrome.runtime.sendMessage({
    type: "HANDSHAKE",
    service: "notebooklm",
    target,
  });
  console.log(`[aibbe] Handshake sent: target=${target ?? "(pending)"}`);
}

function watchLibraryTitle(titleElement) {
  const navObserver = new MutationObserver(() => {
    const text = titleElement.textContent?.trim();
    if (text) {
      sendHandshake(text);
    } else {
      navObserver.disconnect();
      waitForLibraryTitle();
    }
  });

  navObserver.observe(titleElement, {
    childList: true,
    characterData: true,
    subtree: true,
  });
}

function waitForLibraryTitle() {
  // Register the tab immediately with a null target so CLI commands can route
  // to it before the notebook title renders. The target is updated to the real
  // title as soon as the observer sees it.
  sendHandshake(null);

  const existing = document.querySelector(TITLE_SELECTOR);
  const existingText = existing?.textContent?.trim();
  if (existingText) {
    sendHandshake(existingText);
    watchLibraryTitle(existing);
    return;
  }

  // Fallback source: derive the notebook name from the page <title>. Keeps
  // target routing alive when the in-page title element changes class names.
  const docName = titleFromDocument();
  if (docName) {
    sendHandshake(docName);
  }
  const docTitleObserver = watchDocumentTitle(docName);

  const observer = new MutationObserver(() => {
    const el = document.querySelector(TITLE_SELECTOR);
    const text = el?.textContent?.trim();
    if (!text) return;

    observer.disconnect();
    docTitleObserver?.disconnect();
    sendHandshake(text);
    watchLibraryTitle(el);
  });

  observer.observe(document.body, { childList: true, subtree: true });
}

function watchDocumentTitle(initialName) {
  const titleElement = document.querySelector("title");
  if (!titleElement) return null;

  let lastName = initialName;
  const observer = new MutationObserver(() => {
    const name = titleFromDocument();
    if (name && name !== lastName) {
      lastName = name;
      sendHandshake(name);
    }
  });

  observer.observe(titleElement, {
    childList: true,
    characterData: true,
    subtree: true,
  });

  return observer;
}

if (typeof document !== "undefined" && typeof MutationObserver !== "undefined") {
  loadSelectors().catch(err => console.warn("[aibbe] Initial selector load failed:", err.message));
  waitForLibraryTitle();
}

function waitForNextFrame() {
  return new Promise((resolve) => requestAnimationFrame(resolve));
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function randomBetween(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

async function typeWithJitter(element, text, range) {
  element.focus?.();

  if (typeof document.execCommand === "function") {
    document.execCommand("selectAll", false, null);
    document.execCommand("delete", false, null);
  } else if (element.getAttribute?.("contenteditable") === "true") {
    element.textContent = "";
  } else {
    const nativeSetter = Object.getOwnPropertyDescriptor(
      window.HTMLTextAreaElement?.prototype,
      "value",
    )?.set;
    if (nativeSetter) {
      nativeSetter.call(element, "");
    }
  }

  for (const char of text) {
    element.dispatchEvent(new KeyboardEvent("keydown", { key: char, bubbles: true }));
    element.dispatchEvent(new KeyboardEvent("keypress", { key: char, bubbles: true }));

    if (typeof document.execCommand === "function") {
      document.execCommand("insertText", false, char);
    } else if (element.getAttribute?.("contenteditable") === "true") {
      element.textContent = (element.textContent ?? "") + char;
      element.dispatchEvent(new Event("input", { bubbles: true }));
    } else {
      const nativeSetter = Object.getOwnPropertyDescriptor(
        window.HTMLTextAreaElement?.prototype,
        "value",
      )?.set;
      if (nativeSetter) {
        nativeSetter.call(element, (element.value ?? "") + char);
        element.dispatchEvent(new Event("input", { bubbles: true }));
      }
    }

    element.dispatchEvent(new KeyboardEvent("keyup", { key: char, bubbles: true }));
    await sleep(randomBetween(range[0], range[1]));
  }
}

function normalizeWhitespace(value) {
  return value
    .replace(/[ \t]+\n/g, "\n")
    .replace(/\n[ \t]+/g, "\n")
    .replace(/\n{3,}/g, "\n\n")
    .replace(/[ \t]{2,}/g, " ")
    .trim();
}

function extractCleanTextFromNode(root) {
  if (!root) {
    return "";
  }

  const clone = typeof root.cloneNode === "function" ? root.cloneNode(true) : root;
  const removable = typeof clone.querySelectorAll === "function" ? clone.querySelectorAll(activeSelectors.CITATION_NOISE) : [];

  for (const node of removable) {
    if (typeof node.remove === "function") {
      node.remove();
    }
  }

  if (typeof clone.querySelectorAll === "function") {
    for (const icon of clone.querySelectorAll("mat-icon")) {
      const text = (icon.textContent || "").trim().toLowerCase();
      const label = (icon.getAttribute?.("aria-label") || "").trim().toLowerCase();
      if (text === "more_horiz" || label.includes("citas adicionales")) {
        if (typeof icon.remove === "function") {
          icon.remove();
        }
      }
    }
  }

  if (typeof clone.innerText === "string" && clone.innerText.trim()) {
    return normalizeWhitespace(clone.innerText);
  }

  return normalizeWhitespace(clone.textContent || "");
}

function evaluateResponseState(doc, selectors, baselineCount) {
  const responseContainers = doc.querySelectorAll(selectors.RESPONSE_CONTAINER);
  const containerCount = responseContainers.length;

  if (containerCount <= baselineCount) {
    return { eligible: false, containerCount };
  }

  const latestResponse = responseContainers[responseContainers.length - 1];
  const contentRoot = latestResponse.querySelector(selectors.RESPONSE_TEXT) || latestResponse;
  const result = extractCleanTextFromNode(contentRoot) || "";

  const messageScope = latestResponse.closest("chat-message") || latestResponse.parentElement || latestResponse;
  const scopeTag = messageScope ? String(messageScope.tagName || "").toUpperCase() : "";

  return {
    eligible: true,
    result,
    hasThinkingMarkers: messageScope.querySelector(selectors.THINKING_MARKERS) !== null,
    hasReadyMarkers: messageScope.querySelector(selectors.RESPONSE_READY_MARKERS) !== null,
    scopeTag,
    containerCount,
  };
}

function inspectLatestResponse(doc) {
  return evaluateResponseState(doc || document, activeSelectors, 0);
}

function setInputValue(inputElement, payload) {
  inputElement.focus?.();

  if (typeof document.execCommand === "function") {
    if (typeof inputElement.select === "function") {
      inputElement.select();
    } else {
      document.execCommand("selectAll", false, null);
    }
    document.execCommand("insertText", false, payload);
    inputElement.dispatchEvent(new Event("input", { bubbles: true }));
    return;
  }

  const nativeSetter = Object.getOwnPropertyDescriptor(
    window.HTMLTextAreaElement?.prototype,
    "value",
  )?.set;

  if (nativeSetter && "value" in inputElement) {
    nativeSetter.call(inputElement, payload);
    inputElement.dispatchEvent(new Event("input", { bubbles: true }));
    return;
  }

  if (inputElement.getAttribute?.("contenteditable") === "true") {
    inputElement.textContent = payload;
    inputElement.dispatchEvent(new Event("input", { bubbles: true }));
    return;
  }

  throw new Error("unsupported_input_element");
}

async function injectAndSubmit(payload) {
  const inputElement = document.querySelector(activeSelectors.INPUT);
  if (!inputElement) {
    return { status: "error", error: "input_not_found" };
  }

  if (window.__AIBBE_HUMAN_TYPING) {
    const jitterRange = window.__AIBBE_JITTER_RANGE ?? [40, 120];
    await typeWithJitter(inputElement, payload, jitterRange);
  } else {
    setInputValue(inputElement, payload);
    await waitForNextFrame();
  }

  const submitButton = document.querySelector(activeSelectors.SUBMIT_BUTTON);
  if (!submitButton) {
    return { status: "error", error: "submit_button_not_found" };
  }

  if (window.__AIBBE_HUMAN_TYPING) {
    const submitRange = window.__AIBBE_SUBMIT_DELAY_RANGE ?? [500, 2000];
    await sleep(randomBetween(...submitRange));
  }

  const baselineCount = document.querySelectorAll(activeSelectors.RESPONSE_CONTAINER).length;
  submitButton.click();
  return { status: "success", baselineCount };
}

function waitForAIResponse(sendResponse, baselineCount) {
  const timeoutMs = window.__AIBBE_TIMEOUT ?? 150000;
  const pollMs = window.__AIBBE_POLL_MS ?? 500;
  const stableTicksNeeded = window.__AIBBE_STABLE_TICKS ?? 2;
  const hardStableTicksNeeded = window.__AIBBE_HARD_STABLE_TICKS ?? 6;

  let settled = false;
  let stableTicks = 0;
  let observerFires = 0;
  let pollTicks = 0;
  let lastText = "";
  let intervalHandle = null;
  let timeoutHandle = null;
  let observer = null;

  function cleanup() {
    if (intervalHandle !== null) { clearInterval(intervalHandle); intervalHandle = null; }
    if (timeoutHandle !== null) { clearTimeout(timeoutHandle); timeoutHandle = null; }
    if (observer !== null) { observer.disconnect(); observer = null; }
  }

  function finish(payload) {
    if (settled) return;
    settled = true;
    cleanup();
    sendResponse(payload);
  }

  const submitButton = document.querySelector(activeSelectors.SUBMIT_BUTTON);
  if (!submitButton) {
    finish({ status: "error", error: "submit_button_not_found" });
    return;
  }

  function tick(source) {
    if (settled) return;

    if (source === "poll") pollTicks++;

    let state;
    try {
      state = evaluateResponseState(document, activeSelectors, baselineCount);
    } catch (err) {
      finish({ status: "error", error: err.message || "evaluation_failed" });
      return;
    }

    if (!state.eligible) return;

    if (source === "observer") {
      observerFires++;
      if (state.result !== lastText) {
        lastText = state.result;
        stableTicks = 0;
      }
      return;
    }

    if (state.result !== lastText) {
      lastText = state.result;
      stableTicks = 0;
      return;
    }

    stableTicks++;

    const ready = state.hasReadyMarkers && state.result.trim().length > 0;

    if (ready && stableTicks >= stableTicksNeeded && !state.hasThinkingMarkers) {
      finish({ status: "success", result: state.result });
      return;
    }

    if (ready && stableTicks >= hardStableTicksNeeded) {
      finish({ status: "success", result: state.result });
    }
  }

  timeoutHandle = setTimeout(() => {
    let state;
    try {
      state = evaluateResponseState(document, activeSelectors, baselineCount);
    } catch (err) {
      finish({ status: "error", error: err.message || "evaluation_failed" });
      return;
    }
    finish({
      status: "error",
      error: "response_timeout",
      detail: {
        hasThinkingMarkers: state.eligible ? state.hasThinkingMarkers : false,
        hasReadyMarkers: state.eligible ? state.hasReadyMarkers : false,
        resultLength: state.result ? state.result.length : 0,
        scopeTag: state.eligible ? state.scopeTag : "",
        observerFires,
        pollTicks,
        containerCount: state.containerCount,
        baselineCount,
      },
    });
  }, timeoutMs);

  observer = new MutationObserver(() => tick("observer"));
  observer.observe(document.body, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: ["disabled"],
  });

  intervalHandle = setInterval(() => tick("poll"), pollMs);
}

function probeSelectors() {
  const report = {};
  for (const [key, selector] of Object.entries(activeSelectors)) {
    let count = 0;
    let error = null;
    try {
      count = document.querySelectorAll(selector).length;
    } catch (err) {
      error = err.message;
    }
    report[key] = {
      selector,
      matches: count,
      status: error ? "invalid" : count === 0 ? "missing" : count === 1 ? "unique" : "multiple",
      ...(error ? { error } : {}),
    };
  }
  return report;
}

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message.type === "UPDATE_SELECTORS") {
    loadSelectors()
      .then(() => console.log("[aibbe] Selectors reloaded"))
      .catch(err => console.warn("[aibbe] Reload failed:", err.message));
    return;
  }

  if (message.cmd === "get-active-selectors") {
    (async () => {
      try {
        await loadSelectors();
        const stored = await chrome.storage.local.get("aibbe_calibrations");
        const calibrations = stored.aibbe_calibrations ?? {};
        const result = {};
        for (const key of Object.keys(SELECTORS)) {
          result[key] = {
            value: activeSelectors[key],
            source: key in calibrations ? "calibration" : "default",
          };
        }
        sendResponse({ status: "success", selectors: result });
      } catch (err) {
        sendResponse({ status: "error", error: err.message });
      }
    })();
    return true;
  }

  if (message.cmd === "probe-selectors") {
    sendResponse({ status: "success", report: probeSelectors() });
    return true;
  }

  if (message.cmd === "generate") {
    injectAndSubmit(message.payload).then((result) => {
      if (result.status === "error") {
        sendResponse(result);
        return;
      }
      waitForAIResponse(sendResponse, result.baselineCount ?? 0);
    }).catch((error) => {
      sendResponse({ status: "error", error: error.message || "injection_failed" });
    });
    return true;
  }
});
