"use strict";

chrome.runtime.sendMessage({
  type: "HANDSHAKE",
  service: "notebooklm",
});

console.log("[aibbe] Handshake sent for notebooklm");

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message.cmd === "generate") {
    sendResponse({ status: "success", result: "mocked code source" });
    return true;
  }
});
