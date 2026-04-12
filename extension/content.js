"use strict";

// Selector cascade: prefer NotebookLM's textarea, but keep contenteditable as a
// fallback in case the provider swaps editor primitives in a future rollout.
const SELECTORS = {
  INPUT: 'textarea[aria-label="Query box"], textarea[aria-label="Cuadro de consulta"], textarea, div[contenteditable="true"]',
  SUBMIT_BUTTON: 'button[aria-label="Submit"], button[aria-label="Enviar"], button[aria-label*="send"], button[type="submit"], button[data-testid*="send"]',
  RESPONSE_CONTAINER: '.to-user-container, [data-testid*="response"], .response-container, .chat-response, .model-response',
  RESPONSE_TEXT: '.message-text-content',
  THINKING_MARKERS: 'thinking-animation, .thinking-message',
  RESPONSE_READY_MARKERS: '.message-actions, .actions-container, .xap-copy-to-clipboard, .suggestions-container, .follow-up-chip',
  CITATION_NOISE: 'button.citation-marker, button.xap-inline-dialog, [dialoglabel="Detalles de la cita"], .citation-marker, .xap-inline-dialog',
  CODE_BLOCK: 'code, pre',
};

chrome.runtime.sendMessage({
  type: "HANDSHAKE",
  service: "notebooklm",
});

console.log("[aibbe] Handshake sent for notebooklm");

function waitForNextFrame() {
  return new Promise((resolve) => requestAnimationFrame(resolve));
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
  const removable = typeof clone.querySelectorAll === "function" ? clone.querySelectorAll(SELECTORS.CITATION_NOISE) : [];

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
  const responseContainers = document.querySelectorAll(SELECTORS.RESPONSE_CONTAINER);
  if (responseContainers.length === 0) {
    return null;
  }

  const latestResponse = responseContainers[responseContainers.length - 1];
  const contentRoot = latestResponse.querySelector(SELECTORS.RESPONSE_TEXT) || latestResponse;
  const codeBlocks = contentRoot.querySelectorAll(SELECTORS.CODE_BLOCK);
  const result = (codeBlocks.length > 0
    ? codeBlocks[codeBlocks.length - 1].textContent
    : extractCleanTextFromNode(contentRoot)) || "";

  return {
    result,
    hasThinkingMarkers: latestResponse.querySelector(SELECTORS.THINKING_MARKERS) !== null,
    hasReadyMarkers: latestResponse.querySelector(SELECTORS.RESPONSE_READY_MARKERS) !== null,
  };
}

function setInputValue(inputElement, payload) {
  inputElement.focus?.();

  // execCommand("insertText") fires a trusted InputEvent (isTrusted: true).
  // Angular Material's DefaultValueAccessor only updates the FormControl when
  // it receives a trusted event — the native setter + synthetic Event dispatch
  // writes to the DOM but leaves the FormControl invalid, keeping the button disabled.
  if (typeof document.execCommand === "function") {
    if (typeof inputElement.select === "function") {
      inputElement.select();
    } else {
      document.execCommand("selectAll", false, null);
    }
    document.execCommand("insertText", false, payload);
    return;
  }

  // Fallback for non-Angular environments (e.g. test harness, plain textareas).
  const nativeSetter = Object.getOwnPropertyDescriptor(
    window.HTMLTextAreaElement.prototype,
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
  const inputElement = document.querySelector(SELECTORS.INPUT);
  if (!inputElement) {
    return { status: "error", error: "input_not_found" };
  }

  // setInputValue dispatches the input event internally (trusted via execCommand,
  // or synthetic via fallback) — no extra dispatch needed here.
  setInputValue(inputElement, payload);

  await waitForNextFrame();

  const submitButton = document.querySelector(SELECTORS.SUBMIT_BUTTON);
  if (!submitButton) {
    return { status: "error", error: "submit_button_not_found" };
  }

  submitButton.click();
  return { status: "success" };
}

function waitForAIResponse(sendResponse) {
  const submitButton = document.querySelector(SELECTORS.SUBMIT_BUTTON);
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

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
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
