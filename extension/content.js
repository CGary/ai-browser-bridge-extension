"use strict";

chrome.runtime.sendMessage({
  type: "HANDSHAKE",
  service: "notebooklm",
});

console.log("[aibbe] Handshake sent for notebooklm");
