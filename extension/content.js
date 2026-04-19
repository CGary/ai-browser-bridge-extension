"use strict";

console.log("[aibbe] Content script version: 2026-04-19 18:30 (Jerarquías Absolutas)");

// Selectors based on live DOM inspection from docs/selectores
const DEFAULT_SELECTORS = {
  // Primary: exact hierarchy from user, Secondary: semantic classes/attributes
  INPUT: 'body > labs-tailwind-root > div > notebook > div > section.chat-panel.ng-tns-c1370865089-0 > chat-panel > omnibar > div > div > div > query-box > div > div > form > div > textarea',

  SUBMIT_BUTTON: 'body > labs-tailwind-root > div > notebook > div > section.chat-panel.ng-tns-c1370865089-0 > chat-panel > omnibar > div > div > div > query-box > div > div > form > div > div > button',

  RESPONSE_CONTAINER: 'div.chat-panel-content > div > chat-message:nth-child(2) mat-card-content, mat-card-content.message-content, .to-user-container',

  RESPONSE_TEXT: 'div.message-text-content, mat-card-content > div > div, .message-content',

  THINKING_MARKERS: '.thinking-message, thinking-animation',

  RESPONSE_READY_MARKERS: '.message-actions, .xap-copy-to-clipboard, [aria-label*="copy"]',

  CITATION_NOISE: 'button.citation-marker, .citation-marker, button.xap-inline-dialog',

  CODE_BLOCK: 'code, pre',
};

let activeSelectors = { ...DEFAULT_SELECTORS };

async function loadSelectors() {
  try {
    const stored = await chrome.storage.local.get("aibbe_calibrations");
    const calibrations = stored.aibbe_calibrations ?? {};
    activeSelectors = { ...DEFAULT_SELECTORS };
    for (const [key, value] of Object.entries(calibrations)) {
      if (key in DEFAULT_SELECTORS) {
        activeSelectors[key] = String(value);
      }
    }
    console.log("[aibbe] Selectors loaded — calibrations:", Object.keys(calibrations));
  } catch (err) {
    console.warn("[aibbe] loadSelectors failed, using defaults:", err.message);
    activeSelectors = { ...DEFAULT_SELECTORS };
  }
}

const TITLE_SELECTOR = "div.cover-title";
const HANDSHAKE_TIMEOUT_MS = 5000;
let _pickerSession = null;

function sendHandshake(target) {
  chrome.runtime.sendMessage({
    type: "HANDSHAKE",
    service: "notebooklm",
    target,
  });
  console.log(`[aibbe] Handshake sent: target=${target}`);
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
  const existing = document.querySelector(TITLE_SELECTOR);
  const existingText = existing?.textContent?.trim();
  if (existingText) {
    sendHandshake(existingText);
    watchLibraryTitle(existing);
    return;
  }

  const timeoutId = setTimeout(() => {
    observer.disconnect();
    console.warn("[aibbe] Timed out waiting for library title — HANDSHAKE not sent");
  }, HANDSHAKE_TIMEOUT_MS);

  const observer = new MutationObserver(() => {
    const el = document.querySelector(TITLE_SELECTOR);
    const text = el?.textContent?.trim();
    if (!text) return;

    clearTimeout(timeoutId);
    observer.disconnect();
    sendHandshake(text);
    watchLibraryTitle(el);
  });

  observer.observe(document.body, { childList: true, subtree: true });
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

function inspectLatestResponse() {
  const responseContainers = document.querySelectorAll(activeSelectors.RESPONSE_CONTAINER);
  if (responseContainers.length === 0) {
    return null;
  }

  const latestResponse = responseContainers[responseContainers.length - 1];
  const contentRoot = latestResponse.querySelector(activeSelectors.RESPONSE_TEXT) || latestResponse;
  const codeBlocks = contentRoot.querySelectorAll(activeSelectors.CODE_BLOCK);
  const result = (codeBlocks.length > 0
    ? codeBlocks[codeBlocks.length - 1].textContent
    : extractCleanTextFromNode(contentRoot)) || "";

  return {
    result,
    hasThinkingMarkers: latestResponse.querySelector(activeSelectors.THINKING_MARKERS) !== null,
    hasReadyMarkers: latestResponse.querySelector(activeSelectors.RESPONSE_READY_MARKERS) !== null,
  };
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

  submitButton.click();
  return { status: "success" };
}

function waitForAIResponse(sendResponse) {
  const submitButton = document.querySelector(activeSelectors.SUBMIT_BUTTON);
  if (!submitButton) {
    sendResponse({ status: "error", error: "submit_button_not_found" });
    return;
  }

  const timeoutMs = window.__AIBBE_TIMEOUT ?? 150000;
  const settleMs = window.__AIBBE_SETTLE_MS ?? 750;
  let settleTimer = null;
  let pendingSnapshot = "";
  const timeout = setTimeout(() => {
    if (settleTimer) {
      clearTimeout(settleTimer);
    }
    observer.disconnect();
    sendResponse({ status: "error", error: "response_timeout" });
  }, timeoutMs);

  const flushResponse = () => {
    const state = inspectLatestResponse();
    if (!state) {
      return;
    }

    if (!state.hasThinkingMarkers && state.hasReadyMarkers && state.result.trim() && state.result === pendingSnapshot) {
      clearTimeout(timeout);
      if (settleTimer) {
        clearTimeout(settleTimer);
      }
      observer.disconnect();
      sendResponse({ status: "success", result: state.result });
    }
  };

  const observer = new MutationObserver(() => {
    const state = inspectLatestResponse();
    if (!state) {
      return;
    }

    if (state.hasThinkingMarkers || !state.hasReadyMarkers || !state.result.trim()) {
      if (settleTimer) {
        clearTimeout(settleTimer);
        settleTimer = null;
      }
      return;
    }

    pendingSnapshot = state.result;
    if (settleTimer) {
      clearTimeout(settleTimer);
    }

    settleTimer = setTimeout(flushResponse, settleMs);
  });

  observer.observe(document.body, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: ["disabled"],
  });
}

function generateSelector(element) {
  if (element?.id && !/^ng-|^cdk-|^mat-|^auto-|\d{5,}/.test(element.id)) {
    return `#${CSS.escape(element.id)}`;
  }

  for (const attr of ["data-testid", "data-test-id", "aria-label", "name", "type"]) {
    const val = element?.getAttribute?.(attr);
    if (val && val.length < 60) {
      const sel = `${element.tagName.toLowerCase()}[${attr}="${CSS.escape(val)}"]`;
      if (document.querySelectorAll(sel).length === 1) {
        return sel;
      }
    }
  }

  const stableClasses = [...(element?.classList ?? [])].filter(
    (c) => c.length > 2 && !/^ng-|^cdk-|^mat-|^_ng|^ng\d|tns-c/.test(c),
  );
  if (stableClasses.length > 0) {
    const sel = `.${stableClasses.slice(0, 3).map(CSS.escape).join(".")}`;
    if (document.querySelectorAll(sel).length === 1) {
      return sel;
    }

    const tagSel = `${element.tagName.toLowerCase()}.${stableClasses.slice(0, 2).map(CSS.escape).join(".")}`;
    if (document.querySelectorAll(tagSel).length === 1) {
      return tagSel;
    }
  }

  const parent = element?.parentElement;
  if (parent && element?.tagName) {
    const index = [...parent.children].indexOf(element) + 1;
    return `${element.tagName.toLowerCase()}:nth-child(${index})`;
  }

  return element?.tagName?.toLowerCase?.() ?? "unknown";
}

function deactivateVisualPicker(response) {
  if (!_pickerSession) return;
  const { highlight, tooltip, onMouseOver, onClick, onKeyDown, sendResponse } = _pickerSession;
  document.removeEventListener("mouseover", onMouseOver, true);
  if (onClick) document.removeEventListener("click", onClick, true);
  if (onKeyDown) document.removeEventListener("keydown", onKeyDown, true);
  highlight.remove();
  tooltip.remove();
  _pickerSession = null;
  if (response && sendResponse) sendResponse(response);
}

function activateVisualPicker(key, sendResponse) {
  if (_pickerSession) deactivateVisualPicker(null);

  const highlight = document.createElement("div");
  highlight.id = "aibbe-picker-highlight";
  highlight.style.cssText = [
    "position:fixed", "pointer-events:none", "z-index:2147483646",
    "border:2px solid #4285f4", "background:rgba(66,133,244,0.12)",
    "border-radius:2px", "transition:all 80ms ease", "box-sizing:border-box",
  ].join(";");

  const tooltip = document.createElement("div");
  tooltip.id = "aibbe-picker-tooltip";
  tooltip.style.cssText = [
    "position:fixed", "z-index:2147483647", "background:#1a1a2e", "color:#e0e0ff",
    "padding:5px 10px", "font-size:11px", "font-family:monospace", "border-radius:4px",
    "pointer-events:none", "max-width:480px", "word-break:break-all",
    "box-shadow:0 2px 8px rgba(0,0,0,0.4)",
  ].join(";");

  document.body.appendChild(highlight);
  document.body.appendChild(tooltip);

  const onMouseOver = (e) => {
    const el = e.target;
    if (!(el instanceof Element) || el === highlight || el === tooltip) return;
    const rect = el.getBoundingClientRect();
    Object.assign(highlight.style, {
      top: `${rect.top + window.scrollY}px`,
      left: `${rect.left + window.scrollX}px`,
      width: `${rect.width}px`,
      height: `${rect.height}px`,
    });
    tooltip.textContent = `[${key}] ${generateSelector(el)}`;
    tooltip.style.top = `${Math.min(rect.bottom + window.scrollY + 6, window.innerHeight - 40)}px`;
    tooltip.style.left = `${Math.min(rect.left + window.scrollX, window.innerWidth - 500)}px`;
  };

  document.addEventListener("mouseover", onMouseOver, true);
  _pickerSession = { highlight, tooltip, onMouseOver, onClick: null, onKeyDown: null, key, sendResponse };

  const onClick = (e) => {
    e.preventDefault();
    e.stopPropagation();
    const el = e.target;
    if (!(el instanceof Element) || !_pickerSession) {
      return;
    }
    const selector = generateSelector(el);
    const { key: activeKey } = _pickerSession;
    deactivateVisualPicker({ status: "success", key: activeKey, selector });
  };

  const onKeyDown = (e) => {
    if (e.key === "Escape" && _pickerSession) {
      const { key: activeKey } = _pickerSession;
      deactivateVisualPicker({ status: "cancelled", key: activeKey });
    }
  };

  document.addEventListener("click", onClick, true);
  document.addEventListener("keydown", onKeyDown, true);
  _pickerSession.onClick = onClick;
  _pickerSession.onKeyDown = onKeyDown;
}

loadSelectors().catch((err) => console.warn("[aibbe] Initial selector load failed:", err.message));

if (typeof document !== "undefined" && typeof MutationObserver !== "undefined") {
  waitForLibraryTitle();
}

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message.type === "UPDATE_SELECTORS") {
    loadSelectors()
      .then(() => console.log("[aibbe] Selectors reloaded from UPDATE_SELECTORS"))
      .catch((err) => console.warn("[aibbe] Selector reload failed:", err.message));
    return;
  }

  if (message.type === "ACTIVATE_VISUAL_PICKER") {
    const key = message.key || "UNKNOWN";
    activateVisualPicker(key, sendResponse);
    return true;
  }

  if (message.type === "DEACTIVATE_VISUAL_PICKER") {
    if (_pickerSession) {
      const { key } = _pickerSession;
      deactivateVisualPicker({ status: "cancelled", key });
    }
    sendResponse({ status: "success" });
    return true;
  }

  if (message.cmd === "get-active-selectors") {
    (async () => {
      try {
        await loadSelectors();
        const stored = await chrome.storage.local.get("aibbe_calibrations");
        const calibrations = stored.aibbe_calibrations ?? {};
        const result = {};
        for (const key of Object.keys(DEFAULT_SELECTORS)) {
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

  if (message.cmd === "generate") {
    injectAndSubmit(message.payload).then((result) => {
      if (result.status === "error") {
        sendResponse(result);
        return;
      }

      waitForAIResponse(sendResponse);
    }).catch((error) => {
      sendResponse({ status: "error", error: error.message || "injection_failed" });
    });

    return true;
  }
});
