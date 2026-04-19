# Review Cycle 1 — WP01 Rejected

## Outcome
Rejected.

## Summary
The implementation is close, and it stays within the owned file boundary, but the visual picker overlay has a functional positioning defect that breaks the hover/highlight behavior on scrolled pages.

## What Passed
- `extension/content.js` is the only implementation file changed in the lane diff.
- `DEFAULT_SELECTORS`, `activeSelectors`, `loadSelectors()`, `UPDATE_SELECTORS`, `get-active-selectors`, and the visual-picker message handlers are present.
- Syntax check passes (`node --check extension/content.js`).
- No stray `SELECTORS.` references remain.

## Blocking Issue
### 1. Fixed-position overlay uses document scroll offsets
In `activateVisualPicker()`, both the highlight and tooltip are styled with `position:fixed`, but their coordinates are computed using `getBoundingClientRect()` **plus** `window.scrollX/window.scrollY`:

- `extension/content.js:426-427`
- `extension/content.js:432-433`

`getBoundingClientRect()` already returns viewport-relative coordinates. Adding scroll offsets shifts the overlay away from the hovered element whenever the page is scrolled. This breaks the WP acceptance criteria that the picker should highlight the hovered element reliably on a live page.

## Required Fix
Use viewport coordinates directly for fixed-position overlay elements:
- highlight: `top = rect.top`, `left = rect.left`
- tooltip: compute from `rect.bottom` / `rect.left` without adding page scroll offsets

If you want scroll-relative coordinates, then the overlay would need `position:absolute` instead — but the current implementation uses `position:fixed`.

## Re-review Checklist
- Hover highlight remains aligned with the hovered element after vertical and horizontal scrolling.
- Tooltip remains aligned and within viewport bounds on scrolled pages.
- Existing activate/click/Escape flows still work.
