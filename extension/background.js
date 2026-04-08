"use strict";

const NM_HOST_NAME = "aibbe";
const LOG_PREFIX = "[aibbe]";

function connectToNativeHost() {
  console.log(`${LOG_PREFIX} Connecting to native host:`, NM_HOST_NAME);

  let port;

  try {
    port = chrome.runtime.connectNative(NM_HOST_NAME);
  } catch (error) {
    console.error(`${LOG_PREFIX} Failed to connect to native host:`, error);
    return null;
  }

  port.onMessage.addListener((message) => {
    console.log(`${LOG_PREFIX} Message from native host:`, JSON.stringify(message));
    port.postMessage(message);
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

connectToNativeHost();
