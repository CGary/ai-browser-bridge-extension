# AGENTS.md: OpenCode Chromium Extension Agent Architecture

This document defines the logical entities (agents) responsible for executing the browser extension's operations within the OpenCode architecture. It strictly delineates their roles, responsibilities, and operational constraints, ensuring full compatibility with the **Gentleman Ecosystem**, including Spec-Driven Development (SDD) workflows and Engram-based persistence.

## 1. Routing Agent: Background Script (Service Worker)

**Role:** Logical router and lifecycle manager for web interfaces.

**Core Responsibilities:**
* **Communication Management:** Listen for events originating from the host daemon and manage bidirectional message routing via the Native Messaging API. This acts as the bridge for **Agent Teams Lite (ATL)** commands.
* **State Maintenance:** Register tab identifiers (`tabId`) of available web interfaces in volatile memory, categorizing them by service and availability state (`free` or `busy`).
* **Handshake Initialization:** Receive the availability event emitted by the Content Script upon loading an interface and store the associated `tabId` with a `free` state.
* **Garbage Collection:** Listen to the native `chrome.tabs.onRemoved` event to immediately purge any closed tab from the memory registry, preventing invalid routing.
* **Deterministic Orchestration:** Route payload requirements validated by the daemon exclusively to a compatible `tabId` currently in a `free` state, modifying its state to `busy` before payload transfer.
* **State Recovery:** Restore a `tabId` state to `free` after receiving a Timeout exception from the Content Script.

## 2. Interaction Agent: Content Script

**Role:** Document Object Model (DOM) manipulator operating in *virtual headless* mode, responsible for executing injection and extraction flows.

**Core Responsibilities:**
* **Context Injection (Local RAG):** Execute selector mapping and insert the payload into the provider's DOM through native manipulation of input nodes.
* **Transparent Activation:** Trigger the provider's native submit event without embedding artificial nodes or altering the original visual structure.
* **Reactive Observation:** Deploy a `MutationObserver` instance to detect the rendering of generated code, strictly conditioned by a predefined Timeout limit.
* **Data Extraction:** Capture the plain text of the rendered source code, terminate the mutation observer, and transfer the response back to the Background Script.
* **Fail-Fast Latency Interruption:** Abort observation if the timeout limit is reached without detecting the expected mutation, emitting a typed JSON "Timeout" exception to the Background Script.

## 3. Transport Agent: Native Messaging Interface

**Role:** Standardized data bridge between the Chromium environment and the host operating system.

**Core Responsibilities:**
* **Secure Exchange:** Utilize the Chromium Native Messaging API to exchange UTF-8 encoded JSON objects with the Daemon process (which may be interfacing with **Engram**'s MCP tools).
* **Protocol Formatting:** Process and emit transmissions that comply with the binary protocol specification, where each message is preceded by a 32-bit integer (4 bytes) denoting its length.

## 4. Global Operational Constraints

* **Memory Isolation & Persistence Contracts:** Agents must operate exclusively in RAM (volatile storage). Implementing browser persistence APIs (e.g., `chrome.storage.local`) for state or data management is **strictly prohibited**. 
  * *Gentleman Ecosystem Compliance:* Any requirement for persistent context must strictly adhere to the defined persistence contract (`engram`, `openspec`, or `hybrid`). State must be delegated to the host daemon and persisted externally via **Engram** (utilizing its SQLite database).
* **Technical Deployment:** The extension will not be packaged for public distribution; agents are designed to be initialized via manual loading (Sideloading) by enabling Developer Mode in the browser.