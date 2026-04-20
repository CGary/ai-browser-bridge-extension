"use strict";

const NM_HOST_NAME = "aibbe";
const LOG_PREFIX = "[aibbe]";
const CALIBRATION_STORAGE_KEY = "aibbe_calibrations";
const VALID_SELECTOR_KEYS = new Set([
  "INPUT",
  "SUBMIT_BUTTON",
  "RESPONSE_CONTAINER",
  "RESPONSE_TEXT",
  "THINKING_MARKERS",
  "RESPONSE_READY_MARKERS",
  "CITATION_NOISE",
  "CODE_BLOCK",
]);
const tabRegistry = new Map();

async function broadcastToAllTabs(message) {
  const sends = [];

  for (const [tabId] of tabRegistry) {
    sends.push(
      chrome.tabs.sendMessage(tabId, message).catch((err) => {
        console.warn(`${LOG_PREFIX} broadcastToAllTabs tab ${tabId} error:`, err.message);
      }),
    );
  }

  await Promise.allSettled(sends);
}

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

    if (message.cmd === "calibrate") {
      try {
        let updates;

        try {
          updates = typeof message.payload === "string"
            ? JSON.parse(message.payload)
            : (message.payload ?? {});
        } catch {
          port.postMessage({ status: "error", error: "invalid_payload" });
          return;
        }

        const stored = await chrome.storage.local.get(CALIBRATION_STORAGE_KEY);
        const existing = stored[CALIBRATION_STORAGE_KEY] ?? {};
        const merged = { ...existing };
        const applied = [];

        for (const [key, value] of Object.entries(updates)) {
          if (VALID_SELECTOR_KEYS.has(key)) {
            merged[key] = String(value);
            applied.push(key);
          } else {
            console.warn(`${LOG_PREFIX} calibrate: unknown key ignored: ${key}`);
          }
        }

        await chrome.storage.local.set({ [CALIBRATION_STORAGE_KEY]: merged });
        await broadcastToAllTabs({ type: "UPDATE_SELECTORS", calibrations: merged });

        port.postMessage({ status: "success", applied });
        console.log(`${LOG_PREFIX} calibrate: applied [${applied.join(", ")}]`);
      } catch (err) {
        port.postMessage({ status: "error", error: err.message });
      }
      return;
    }

    if (message.cmd === "reset-selectors") {
      try {
        await chrome.storage.local.remove(CALIBRATION_STORAGE_KEY);
        await broadcastToAllTabs({ type: "UPDATE_SELECTORS", calibrations: {} });
        port.postMessage({ status: "success" });
        console.log(`${LOG_PREFIX} reset-selectors: all calibrations cleared`);
      } catch (err) {
        port.postMessage({ status: "error", error: err.message });
      }
      return;
    }

    if (message.cmd === "get-active-selectors") {
      const tab = findFreeTab();
      if (!tab) {
        port.postMessage({ status: "error", error: "no_free_tabs" });
        return;
      }

      const { tabId, entry } = tab;
      entry.state = "busy";

      try {
        const response = await chrome.tabs.sendMessage(tabId, {
          cmd: "get-active-selectors",
          payload: "",
        });
        port.postMessage(response);
      } catch (err) {
        port.postMessage({ status: "error", error: err.message });
      } finally {
        if (tabRegistry.has(tabId)) {
          tabRegistry.get(tabId).state = "free";
        }
      }
      return;
    }

    const tab = message.target
      ? findTargetedTab(message.target)
      : findFreeTab();

    if (!tab) {
      const error = message.target ? "target_not_found" : "no_free_tabs";
      port.postMessage({ status: "error", error });
      return;
    }

    const { tabId, entry } = tab;
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
    target: message.target,
  });

  console.log(`${LOG_PREFIX} Tab ${sender.tab.id} registered for ${message.service} target=${message.target}`);
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

function findTargetedTab(target) {
  for (const [tabId, entry] of tabRegistry) {
    if (entry.state === "free" && entry.target === target) {
      return { tabId, entry };
    }
  }
  return null;
}

connectToNativeHost();
