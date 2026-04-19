# Dynamic Selector Calibration System

## Overview

**Mission ID**: 01KPKVF761KT27SJ1GZFNEMFYW
**Mission Type**: software-dev
**Target Branch**: main
**Status**: In Progress

## Feature Summary

Enable operators to update and persist Chrome extension DOM selectors at runtime — without reloading the extension or browser tabs. Calibrations take effect immediately across all registered tabs and survive browser restarts. A Visual Picker mode lets operators identify and capture optimal selectors by clicking directly on page elements.

## Problem Statement

When a target web application (e.g., NotebookLM) updates its UI, dynamically generated class names change and the extension stops working. The current fix cycle — inspect, edit source, reload extension, refresh tab — is slow and impossible in production/Docker environments without access to source files.

## Actors

| Actor | Description |
|-------|-------------|
| Operator | Developer or engineer who manages and debugs the extension via CLI |

## User Scenarios & Testing

### Scenario 1: Calibrate a broken selector

**Given** the extension is running and an element selector no longer matches the page
**When** the operator sends a `calibrate` command with the updated selector value
**Then** the extension immediately uses the new selector on all open registered tabs without requiring a reload
**And** the calibration persists across browser restarts

### Scenario 2: Visual Picker — select by clicking

**Given** the operator wants to find the correct selector for a page element
**When** the operator activates the Visual Picker via CLI targeting a specific selector key
**Then** the tab enters highlight mode where hovering over elements shows selector candidates
**And** clicking an element sends the auto-generated selector back to the CLI as a response
**And** the operator can confirm or cancel the selection

### Scenario 3: Inspect active selectors

**Given** the operator wants to audit which selectors are currently active
**When** the operator runs the `get-active-selectors` command
**Then** the CLI receives a complete map of all selector names and their current values
**And** each entry indicates whether it came from a saved calibration or the code default

### Scenario 4: Reset to factory defaults

**Given** saved calibrations are no longer needed or are causing issues
**When** the operator runs `reset-selectors`
**Then** all stored calibrations are cleared
**And** the extension immediately reverts to source-code defaults on all registered tabs
**And** no reload is required

### Scenario 5: Stable response detection (RESPONSE_READY_MARKERS)

**Given** the operator needs to detect when a NotebookLM response is complete
**When** selecting or calibrating a response-completion selector
**Then** the system targets RESPONSE_READY_MARKERS — elements that are stable and only present when a full response is rendered
**And** THINKING_MARKERS are explicitly excluded as detection targets because they are transient and only briefly visible during AI processing

### Scenario 6: Partial calibration

**Given** the operator sends a `calibrate` command with only a subset of selector keys
**When** the command is processed
**Then** only the specified selectors are overridden
**And** all other selectors remain at their previous values (calibrated or default)

## Functional Requirements

| ID | Requirement | Status |
|----|-------------|--------|
| FR-001 | The operator can send a `calibrate` command with a JSON payload mapping selector names to new values | Proposed |
| FR-002 | Calibrated selectors take effect on all registered tabs within 5 seconds of the command, without requiring any reload | Proposed |
| FR-003 | Calibrations persist across browser restarts | Proposed |
| FR-004 | Calibrated values take precedence over code-defined defaults at runtime | Proposed |
| FR-005 | The operator can retrieve the full active selector map, with each entry annotated as "calibration" or "default", via `get-active-selectors` | Proposed |
| FR-006 | The operator can erase all stored calibrations and revert to factory defaults via `reset-selectors`, with immediate effect across all registered tabs | Proposed |
| FR-007 | The Visual Picker can be activated from the CLI, placing the target tab into a highlight mode | Proposed |
| FR-008 | In highlight mode, hovering over DOM elements displays one or more selector candidates for that element | Proposed |
| FR-009 | Clicking a highlighted element in Visual Picker mode returns the auto-generated selector to the CLI | Proposed |
| FR-010 | The Visual Picker can be cancelled without selecting any element, leaving current selectors unchanged | Proposed |
| FR-011 | A `calibrate` command that includes only a subset of selector keys must not affect selectors not included in the payload | Proposed |
| FR-012 | Response-completion detection must use RESPONSE_READY_MARKERS as the primary signal; THINKING_MARKERS must not be used as a stable selector target | Proposed |

## Non-Functional Requirements

| ID | Requirement | Threshold | Status |
|----|-------------|-----------|--------|
| NFR-001 | Propagation latency | Calibrated selectors reach all registered tabs in ≤ 5 seconds under normal conditions | Proposed |
| NFR-002 | Command response time | CLI receives a response to any calibration command in ≤ 5 seconds | Proposed |
| NFR-003 | Storage footprint | Total calibration data in persistent storage must not exceed 50 KB | Proposed |
| NFR-004 | Offline-tab resilience | Calibrations set while a tab is not loaded must be applied when that tab is next accessed | Proposed |

## Constraints

| ID | Constraint | Status |
|----|------------|--------|
| C-001 | Calibration data must be stored locally on the user's machine; no data may be transmitted to external servers | Proposed |
| C-002 | The feature must not require changes to the native host manifest or the extension's static ID | Proposed |
| C-003 | The Visual Picker must be scoped to the single tab receiving the activation command and must not affect other tabs | Proposed |
| C-004 | THINKING_MARKERS must not be used as a stable detection signal due to their transient, briefly-visible nature | Proposed |

## Key Entities

| Entity | Description |
|--------|-------------|
| Selector Map | Named key-value map where keys are selector identifiers (e.g., `INPUT`, `SUBMIT_BUTTON`, `RESPONSE_READY_MARKERS`) and values are CSS selectors |
| Calibration | An operator-provided override of one or more entries in the Selector Map, stored persistently |
| Active Selectors | The runtime-resolved Selector Map: calibrations take precedence over code defaults |
| Visual Picker Session | A temporary, single-tab highlight mode that translates operator click events into selector strings |

## Success Criteria

| # | Criterion |
|---|-----------|
| 1 | An operator can update a broken selector and see it take effect on an open tab within 5 seconds, without touching source files or reloading anything |
| 2 | Calibrations survive a browser restart and remain active until explicitly reset |
| 3 | The Visual Picker allows an operator to capture the correct selector for a page element by clicking it, with no file edits required |
| 4 | Running `reset-selectors` returns the extension to full factory-default state within 5 seconds |
| 5 | The active selector set is always inspectable via CLI, showing which entries are calibration overrides and which are defaults |

## Assumptions

| # | Assumption |
|---|------------|
| 1 | The operator has the daemon running before issuing calibration commands |
| 2 | Only one Visual Picker session is active at a time across all tabs |
| 3 | Selector key names are case-sensitive and must match existing keys in the Selector Map exactly |
| 4 | Calibrations from a previous installation are preserved on reinstall unless explicitly reset |

## Out of Scope

- Automatic detection of broken selectors
- Sharing or syncing calibrations between machines or users
- Calibration history, versioning, or granular rollback (beyond full factory reset)
- Any graphical UI beyond the in-page Visual Picker overlay and CLI interface
