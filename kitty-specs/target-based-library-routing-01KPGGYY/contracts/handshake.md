# Contract: HANDSHAKE Message (Content Script → Background Script)

## Transport

`chrome.runtime.sendMessage` — no response expected.

## Message Shape

```json
{
  "type": "HANDSHAKE",
  "service": "notebooklm",
  "target": "<library title from div.cover-title>"
}
```

## Invariants

- `type` is always `"HANDSHAKE"`.
- `service` is always `"notebooklm"` for this extension.
- `target` is always a non-empty, trimmed string — the Content Script MUST NOT send HANDSHAKE before `div.cover-title` contains text.

## Sending Conditions

| Trigger | Condition | Action |
|---------|-----------|--------|
| Page load | `div.cover-title` becomes non-empty (MutationObserver, Phase 1) | Send HANDSHAKE with `target` |
| SPA navigation | `div.cover-title` text content changes (MutationObserver, Phase 2) | Re-send HANDSHAKE with new `target` |
| Timeout | 5 seconds without `div.cover-title` appearing | Log warning, do not send |

## DOM Selector

```
div.cover-title
```

Confirmed from live NotebookLM DOM (`docs/chat-panel.html`). Text content must be `.trim()`-ed before use as `target`.

## Effect on Background Script

On receipt:
```js
tabRegistry.set(sender.tab.id, {
  state: "free",
  service: message.service,
  lastSeen: Date.now(),
  target: message.target,        // NEW
});
```
