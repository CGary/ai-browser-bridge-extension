# Tasks: Humanización DOM con Jitter Anti-Bot

**Mission**: `humanizacion-dom-jitter-anti-bot-01KPGACX`
**Mission ID**: `01KPGACXBHV42ATCXGJ9CQDWBG`
**Target Branch**: `main`
**Date**: 2026-04-18

---

## Subtask Index

| ID | Description | WP | Parallel |
|----|-------------|-----|---------|
| T003 | Agregar `sleep(ms)` helper a `content.js` | WP01 | [P] |
| T004 | Agregar `randomBetween(min, max)` helper a `content.js` | WP01 | [P] |
| T005 | Implementar `typeWithJitter(element, text, range)` async function | WP01 | — |
| T006 | Modificar `injectAndSubmit`: check `__AIBBE_HUMAN_TYPING`, branch humanizado | WP01 | — |
| T001 | Test `setInputValue` path execCommand — spy, verifica selectAll/insertText | WP02 | — |
| T002 | Test `setInputValue` path contenteditable — sin execCommand, verifica textContent | WP02 | — |
| T007 | Test char-by-char: `__AIBBE_JITTER_RANGE:[0,0]` → insertedChars == payload chars | WP02 | — |
| T008 | Test KeyboardEvent: keydown+keypress+keyup por caracter en orden | WP02 | — |
| T009 | Test delay pre-submit: sleep ocurre ANTES del submit_click (via eventLog) | WP02 | — |
| T010 | Regression: `__AIBBE_HUMAN_TYPING` no definido → comportamiento existente intacto | WP02 | — |

---

## Work Packages

### WP01 — Implementar Humanización DOM

**Goal**: Agregar `sleep`, `randomBetween`, `typeWithJitter` a `content.js` y modificar `injectAndSubmit` con flag `__AIBBE_HUMAN_TYPING`.
**Priority**: High
**Lane**: A (independiente)
**Estimated prompt size**: ~300 lines
**File**: `tasks/WP01-implementar-humanizacion-dom.md`

- [ ] T003 Agregar `sleep(ms)` helper a `content.js` (WP01)
- [ ] T004 Agregar `randomBetween(min, max)` helper a `content.js` (WP01)
- [ ] T005 Implementar `typeWithJitter(element, text, range)` async function (WP01)
- [ ] T006 Modificar `injectAndSubmit` con `__AIBBE_HUMAN_TYPING` check (WP01)

**Dependencies**: ninguna
**Risks**: El paso de limpieza inicial en `typeWithJitter` es crítico — sin él, el texto se concatenaría al contenido previo del campo.

---

### WP02 — Tests: Cobertura y Humanización

**Goal**: 6 tests en `extension_handshake_test.go`: cobertura de paths existentes (execCommand, contenteditable) y verificación de la humanización.
**Priority**: High
**Lane**: A (secuencial después de WP01)
**Estimated prompt size**: ~400 lines
**File**: `tasks/WP02-tests-cobertura-y-humanizacion.md`

- [ ] T001 Test path execCommand de `setInputValue` (WP02)
- [ ] T002 Test path contenteditable de `setInputValue` (WP02)
- [ ] T007 Test char-by-char insertion con jitter `[0,0]` (WP02)
- [ ] T008 Test KeyboardEvent dispatch por caracter (WP02)
- [ ] T009 Test delay pre-submit — sleep antes de click (WP02)
- [ ] T010 Regression: humanización desactivada preserva comportamiento (WP02)

**Dependencies**: WP01
**Risks**: El override de `global.setTimeout` en T009 puede interferir con los settle timers. Mantener `__AIBBE_SETTLE_MS: 0` para que el settle use 0ms y no sea capturado por el spy.

---

## Execution Order

```
WP01 (implementation) ──► WP02 (tests)
```

Secuencial: WP02 depende de WP01 porque los tests de humanización cargan `content.js` via `require()` y necesitan que `typeWithJitter` exista.

**Nota**: T001 y T002 (cobertura de paths existentes) podrían ejecutarse en paralelo con WP01 desde una perspectiva lógica, pero se consolidaron en WP02 para evitar conflictos de ownership sobre `extension_handshake_test.go`.

---

## Success Gate

- `go test ./...` pasa al 100% con las 2 WPs integradas.
- `setInputValue` tiene cobertura en los 3 paths: execCommand ✓, native setter ✓ (ya existía), contenteditable ✓.
- Con `__AIBBE_HUMAN_TYPING: true`: chars insertados uno-a-uno con KeyboardEvents ✓.
- Con `__AIBBE_HUMAN_TYPING: false`/no definido: comportamiento existente sin regresiones ✓.
