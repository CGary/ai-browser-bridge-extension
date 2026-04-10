"use strict";

// Selector cascade: prefer NotebookLM's textarea, but keep contenteditable as a
// fallback in case the provider swaps editor primitives in a future rollout.
const SELECTORS = {
  INPUT: 'textarea, div[contenteditable="true"]',
  SUBMIT_BUTTON: 'button[aria-label*="send"], button[type="submit"], button[data-testid*="send"]',
};

chrome.runtime.sendMessage({
  type: "HANDSHAKE",
  service: "notebooklm",
});

console.log("[aibbe] Handshake sent for notebooklm");

function waitForNextFrame() {
  return new Promise((resolve) => requestAnimationFrame(resolve));
}

function setInputValue(inputElement, payload) {
  const nativeSetter = Object.getOwnPropertyDescriptor(
    window.HTMLTextAreaElement.prototype,
    "value",
  )?.set;

  if (nativeSetter && "value" in inputElement) {
    nativeSetter.call(inputElement, payload);
    return;
  }

  if (inputElement.getAttribute?.("contenteditable") === "true") {
    inputElement.textContent = payload;
    return;
  }

  throw new Error("unsupported_input_element");
}

async function injectAndSubmit(payload) {
  const inputElement = document.querySelector(SELECTORS.INPUT);
  if (!inputElement) {
    return { status: "error", error: "input_not_found" };
  }

  setInputValue(inputElement, payload);
  inputElement.dispatchEvent(new Event("input", { bubbles: true }));

  await waitForNextFrame();

  const submitButton = document.querySelector(SELECTORS.SUBMIT_BUTTON);
  if (!submitButton) {
    return { status: "error", error: "submit_button_not_found" };
  }

  submitButton.click();
  return { status: "success" };
}

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message.cmd === "generate") {
    injectAndSubmit(message.payload).then((result) => {
      if (result.status === "error") {
        sendResponse(result);
      }
    }).catch((error) => {
      sendResponse({ status: "error", error: error.message || "injection_failed" });
    });

    return true;
  }
});
