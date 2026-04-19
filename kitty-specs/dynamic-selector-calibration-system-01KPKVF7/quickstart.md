# Operator Quickstart: Dynamic Selector Calibration

**Prerequisites**: daemon running, extension loaded, at least one NotebookLM tab registered.

---

## Inspect active selectors

```bash
./aibbe-cli -cmd "get-active-selectors"
```

Response shows each selector key with its current value and whether it's a calibration override or the factory default.

---

## Calibrate a broken selector

```bash
./aibbe-cli -cmd "calibrate" -payload '{"INPUT": "#new-input-id", "SUBMIT_BUTTON": ".btn-send-v2"}'
```

Takes effect immediately on all open tabs. No reload needed.

---

## Capture a selector visually (Visual Picker)

```bash
./aibbe-cli -cmd "visual-picker-start" -target "My Notebook" -payload '{"key": "INPUT"}'
```

The target tab enters highlight mode. Hover over elements to preview the selector. Click to capture it. The CLI returns the captured selector — apply it with `calibrate`.

To cancel without selecting:
```bash
./aibbe-cli -cmd "visual-picker-cancel" -target "My Notebook"
```

---

## Reset to factory defaults

```bash
./aibbe-cli -cmd "reset-selectors"
```

Clears all stored calibrations. All selectors revert to the values defined in `content.js`. Takes effect immediately.

---

## Notes

- `THINKING_MARKERS` is used internally as a negative signal ("AI is still thinking"). Calibrating it is not recommended and will not improve detection accuracy.
- `RESPONSE_READY_MARKERS` is the primary completion signal. If the extension fails to detect when a response is ready, calibrate this selector first.
- Calibrations survive browser restarts and extension updates.
- Partial calibration: only the keys you specify are updated; others remain unchanged.
