"use strict";

const NM_HOST_NAME = "aibbe";
const LOG_PREFIX = "[aibbe]";
const tabRegistry = new Map();

function connectToNativeHost() {
  console.log(`${LOG_PREFIX} Connecting to native host:`, NM_HOST_NAME);

  let port;

  try {
    port = chrome.runtime.connectNative(NM_HOST_NAME);
  } catch (error) {
    console.error(`${LOG_PREFIX} Failed to connect to native host:`, error);
    return null;
  }

  port.onMessage.addListener(async (message) => {
    console.log(`${LOG_PREFIX} Message from native host:`, JSON.stringify(message));

    const freeTab = findFreeTab();

    if (!freeTab) {
      port.postMessage({ status: "error", error: "no_free_tabs" });
      return;
    }

    const { tabId, entry } = freeTab;
    entry.state = "busy";

    try {
      const response = await chrome.tabs.sendMessage(tabId, {
        cmd: message.cmd,
        payload: message.payload,
      });
      port.postMessage(response);
    } catch (err) {
      port.postMessage({ status: "error", error: err.message });
    } finally {
      if (tabRegistry.has(tabId)) {
        tabRegistry.get(tabId).state = "free";
      }
    }
  });

  port.onDisconnect.addListener(() => {
    const error = chrome.runtime.lastError;

    if (error) {
      console.warn(`${LOG_PREFIX} Native host disconnected with error:`, error.message);
      return;
    }

    console.warn(`${LOG_PREFIX} Native host disconnected`);
  });

  return port;
}

chrome.runtime.onMessage.addListener((message, sender) => {
  if (message.type !== "HANDSHAKE") {
    return;
  }

  if (!sender.tab || typeof sender.tab.id !== "number") {
    return;
  }

  tabRegistry.set(sender.tab.id, {
    state: "free",
    service: message.service,
    lastSeen: Date.now(),
  });

  console.log(`${LOG_PREFIX} Tab ${sender.tab.id} registered for ${message.service}`);
});

chrome.tabs.onRemoved.addListener((tabId) => {
  if (!tabRegistry.has(tabId)) {
    return;
  }

  tabRegistry.delete(tabId);
  console.log(`${LOG_PREFIX} Tab ${tabId} purged from registry`);
});

function findFreeTab() {
  for (const [tabId, entry] of tabRegistry) {
    if (entry.state === "free") return { tabId, entry };
  }
  return null;
}

connectToNativeHost();
