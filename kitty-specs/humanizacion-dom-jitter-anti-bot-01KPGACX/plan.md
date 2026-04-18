# Implementation Plan: Humanización DOM con Jitter Anti-Bot

**Mission**: `humanizacion-dom-jitter-anti-bot-01KPGACX`
**Mission ID**: `01KPGACXBHV42ATCXGJ9CQDWBG`
**Target Branch**: `main`
**Date**: 2026-04-18
**Spec**: [spec.md](spec.md)

---

## Summary

Cubrir con tests los paths de `setInputValue` sin cobertura (`execCommand` y `contenteditable`) e implementar humanización de inyección DOM: función `typeWithJitter` que inserta texto caracter-por-caracter con `KeyboardEvent` y delays aleatorios, activada por `window.__AIBBE_HUMAN_TYPING`. Dos Lanes independientes sin conflicto de merge.

---

## Branch Contract

| Field | Value |
|-------|-------|
| Current branch | `main` |
| Planning base | `main` |
| Merge target | `main` |
| `branch_matches_target` | `true` |

---

## Technical Context

**Language/Version**: Vanilla JS (ES2020, no bundler), Go 1.21 para tests
**Primary Dependencies**: `document.execCommand`, `MutationObserver`, `KeyboardEvent`, `setTimeout`
**Storage**: N/A
**Testing**: Go test harness con scripts Node.js inline (`runNodeJSON`). `go test ./...` desde raíz.
**Target Platform**: Chromium Extension (Manifest V3), NotebookLM
**Project Type**: Browser extension + Go daemon
**Performance Goals**: Jitter default 40–120 ms/char no afecta UX (es headless)
**Constraints**: Sin dependencias externas en `content.js`. `KeyboardEvent` sintéticos tienen `isTrusted: false`.

---

## Charter Check

Governance: `software-dev-default`, DIR-001, DIR-002. Sin conflictos detectados. Cambios aditivos, no modifican contratos IPC ni el protocolo Native Messaging.

---

## Project Structure

### Documentation

```
kitty-specs/humanizacion-dom-jitter-anti-bot-01KPGACX/
├── spec.md
├── plan.md              ← este archivo
├── research.md          ← generado
├── data-model.md        ← generado (contratos de funciones)
├── checklists/
│   └── requirements.md
└── tasks/               ← generado por /spec-kitty.tasks
```

### Source Code

```
extension/
└── content.js           ← Lane B: agrega sleep, randomBetween, typeWithJitter; modifica injectAndSubmit

extension_handshake_test.go  ← Lane A: tests execCommand + contenteditable
                              ← Lane B: tests humanización
```

---

## Lane Breakdown

### Lane A — Test Coverage de Paths Existentes

**Objetivo**: Cubrir con tests los paths de `setInputValue` sin cobertura.
**Archivos**: `extension_handshake_test.go` únicamente.
**Dependencia**: Ninguna. Lane A es completamente independiente.

| WP | Título | Descripción |
|----|--------|-------------|
| WP-A1 | Tests path execCommand | Test `TestExtensionContent_SetInputValue_ExecCommandPath`: harness con `document.execCommand` espía y sin native setter. Verifica: `selectAll` + `insertText(payload)` en orden, native setter no invocado. |
| WP-A2 | Tests path contenteditable | Test `TestExtensionContent_SetInputValue_ContenteditablePath`: harness con elemento `contenteditable="true"`, sin `value`, sin `execCommand`. Verifica: `textContent = payload` y dispatch `Event("input", { bubbles: true })`. |

---

### Lane B — Implementación de Humanización

**Objetivo**: Implementar tipeo caracter-por-caracter con jitter + KeyboardEvents + delay pre-submit.
**Archivos**: `extension/content.js` y `extension_handshake_test.go`.
**Dependencia interna**: WP-B2 depende de WP-B1.
**Dependencia cross-lane**: Ninguna de Lane A.

| WP | Título | Descripción |
|----|--------|-------------|
| WP-B1 | Implementar `typeWithJitter` | Agregar `sleep(ms)`, `randomBetween(min, max)`, `typeWithJitter(element, text, range)` a `content.js`. Modificar `injectAndSubmit`: cuando `window.__AIBBE_HUMAN_TYPING` es `true` usar `typeWithJitter` + delay pre-submit; path existente intacto si es `false` o no definido. |
| WP-B2 | Tests de humanización | Tests: (1) chars insertados uno-a-uno con `__AIBBE_JITTER_RANGE: [0,0]`; (2) KeyboardEvent keydown+keypress+keyup por char; (3) delay pre-submit con `__AIBBE_SUBMIT_DELAY_RANGE: [0,0]`; (4) flag `false` preserva comportamiento actual. |

---

## Orden de Ejecución

```
Lane A ──── WP-A1 → WP-A2
                             ──── merge a main
Lane B ──── WP-B1 → WP-B2
```

Lane A y Lane B corren en paralelo. Dentro de Lane B, WP-B2 depende de WP-B1.

---

## Criterios de Completitud

1. `go test ./...` pasa al 100%.
2. Los 3 paths de `setInputValue` tienen cobertura: execCommand (WP-A1), native setter (ya existente), contenteditable (WP-A2).
3. Con `__AIBBE_HUMAN_TYPING: true` y jitter `[0,0]`: inserción char-por-char + KeyboardEvents verificados (WP-B2).
4. Con `__AIBBE_HUMAN_TYPING: false`: comportamiento existente sin regresiones.
