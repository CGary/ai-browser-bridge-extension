# Spec: Humanización DOM con Jitter Anti-Bot

**Mission ID**: `01KPGACXBHV42ATCXGJ9CQDWBG`
**Mission Slug**: `humanizacion-dom-jitter-anti-bot-01KPGACX`
**Mission Type**: software-dev
**Target Branch**: main
**Created**: 2026-04-18

---

## Overview

El content script (`extension/content.js`) inyecta texto en la interfaz de NotebookLM y envía el formulario de forma programática. Google detecta esta actividad como automatizada porque la interacción no produce los patrones de eventos de teclado ni los tiempos de respuesta que caracterizan a un usuario humano.

Este cambio tiene dos objetivos complementarios:

1. **Cobertura de tests existentes**: El path de inyección via `document.execCommand("insertText")` —que es el principal en producción— no está cubierto por ningún test. El path de `contenteditable` tampoco. Los tests actuales usan el fallback de native setter.

2. **Humanización de la inyección**: Implementar tipeo caracter-por-caracter con delays aleatorios (jitter), dispatch de `KeyboardEvent` por caracter y una pausa aleatoria antes del submit, reduciendo la probabilidad de detección de bot por parte de Google/reCAPTCHA Enterprise.

---

## Problem Statement

Google bloqueó el acceso a NotebookLM porque la extensión manipula el DOM sin producir eventos físicos: sin `keydown`, sin variación temporal entre caracteres, con click inmediato post-inserción. El sistema de seguridad marca esta actividad como automatizada independientemente del volumen de peticiones.

La solución de "humanización" fue documentada en `ideas/bloqueo-gemini.md` pero nunca se implementó en el código.

---

## Actors

- **Sistema automatizado (daemon + extensión)**: el agente que inyecta el payload en la UI de NotebookLM.
- **Interfaz de NotebookLM**: receptora de los eventos de teclado y mouse sintéticos.
- **Google reCAPTCHA Enterprise**: el sistema de detección que evalúa patrones de comportamiento.

---

## User Scenarios & Testing

### Scenario 1: Tipeo humanizado activo (happy path)

**Dado** que `window.__AIBBE_HUMAN_TYPING` es `true` y el content script recibe un mensaje `cmd: "generate"` con un payload de texto,
**Cuando** `injectAndSubmit` es invocado,
**Entonces** cada caracter del payload se inserta individualmente con un `KeyboardEvent` (keydown, keypress, keyup) y un delay aleatorio entre caracteres, seguido de un delay aleatorio antes del `.click()` en el submit button.

### Scenario 2: Tipeo estándar (humanización desactivada)

**Dado** que `window.__AIBBE_HUMAN_TYPING` no está definido o es `false`,
**Cuando** `injectAndSubmit` es invocado,
**Entonces** el comportamiento actual se preserva intacto: `execCommand("insertText")` bulk, `waitForNextFrame()`, click inmediato.

### Scenario 3: Path execCommand cubierto por tests

**Dado** un entorno donde `document.execCommand` está disponible,
**Cuando** `setInputValue` es invocado con un payload,
**Entonces** el test verifica que `execCommand("selectAll")` y `execCommand("insertText", false, payload)` fueron llamados en ese orden, y que no se usó el native setter ni el dispatch de Event sintético.

### Scenario 4: Path contenteditable cubierto por tests

**Dado** un input de tipo `div[contenteditable="true"]` sin `HTMLTextAreaElement.prototype.value`,
**Cuando** `setInputValue` es invocado,
**Entonces** el test verifica que `textContent` fue asignado directamente y se despachó un `Event("input", { bubbles: true })`.

### Scenario 5: Jitter dentro del rango configurado

**Dado** que `window.__AIBBE_JITTER_RANGE` está definido como `[min, max]`,
**Cuando** el tipeo humanizado produce delays entre caracteres,
**Entonces** cada delay medido cae dentro del rango `[min, max]` ms.

---

## Functional Requirements

| ID | Requirement | Status |
|----|-------------|--------|
| FR-001 | El content script DEBE exponer una función `typeWithJitter(element, text, range)` que inserte el texto caracter por caracter con delays aleatorios dentro de `range`. | Proposed |
| FR-002 | Por cada caracter, `typeWithJitter` DEBE despachar `KeyboardEvent` con tipo `keydown`, `keypress` y `keyup` antes/después de insertar el caracter. | Proposed |
| FR-003 | `injectAndSubmit` DEBE usar `typeWithJitter` cuando `window.__AIBBE_HUMAN_TYPING` sea `true`, y preservar el path de `execCommand` cuando sea `false` o no esté definido. | Proposed |
| FR-004 | El delay antes del submit DEBE ser aleatorio dentro del rango `window.__AIBBE_SUBMIT_DELAY_RANGE` (default: `[500, 2000]` ms), activado solo cuando `__AIBBE_HUMAN_TYPING` es `true`. | Proposed |
| FR-005 | Los tests DEBEN cubrir el path de `document.execCommand("insertText")` verificando que `execCommand` fue invocado con los argumentos correctos y que el native setter NO fue usado. | Proposed |
| FR-006 | Los tests DEBEN cubrir el path de `contenteditable` verificando asignación de `textContent` y dispatch de `Event("input")`. | Proposed |
| FR-007 | Los tests de humanización DEBEN usar `window.__AIBBE_JITTER_RANGE: [0, 0]` para eliminar la aleatoriedad y hacer los tests determinísticos. | Proposed |
| FR-008 | `typeWithJitter` DEBE ser una función async que retorna una Promise que resuelve cuando todos los caracteres han sido insertados. | Proposed |

---

## Non-Functional Requirements

| ID | Requirement | Threshold | Status |
|----|-------------|-----------|--------|
| NFR-001 | El overhead de humanización NO debe afectar el tiempo de respuesta cuando `__AIBBE_HUMAN_TYPING` es `false`. | 0 ms adicionales en modo desactivado. | Proposed |
| NFR-002 | Los delays por defecto entre caracteres deben simular velocidad de tipeo humana rápida. | Rango default: 40–120 ms por caracter. | Proposed |
| NFR-003 | El delay default antes del submit debe simular "leer y hacer click". | Rango default: 500–2000 ms. | Proposed |
| NFR-004 | Todos los tests nuevos y existentes deben pasar en el harness Go (`go test ./...`). | 100% pass rate. | Proposed |

---

## Constraints

| ID | Constraint | Status |
|----|------------|--------|
| C-001 | No se puede usar la API oficial de NotebookLM (no existe). La implementación debe ser vía DOM. | Active |
| C-002 | Los `KeyboardEvent` emitidos serán sintéticos (`isTrusted: false`). No bypasean reCAPTCHA Enterprise directamente, pero mejoran los patrones de timing detectables. | Active |
| C-003 | El content script no puede tener dependencias externas (no npm, no bundler). Código vanilla JS. | Active |
| C-004 | La humanización debe ser opt-in via flags `window.__AIBBE_*` para no romper el flujo E2E existente ni los tests actuales. | Active |
| C-005 | No se deben modificar los selectores CSS existentes en `SELECTORS`. | Active |

---

## Key Entities

| Entity | Description |
|--------|-------------|
| `typeWithJitter(element, text, range)` | Función async nueva en `content.js`. Inserta `text` caracter-por-caracter con delays aleatorios en `range: [min, max]` ms y dispatch de KeyboardEvent por caracter. |
| `window.__AIBBE_HUMAN_TYPING` | Flag de activación (boolean). `true` activa humanización. Default: `false`. |
| `window.__AIBBE_JITTER_RANGE` | Par `[min, max]` ms de delay entre caracteres. Default: `[40, 120]`. |
| `window.__AIBBE_SUBMIT_DELAY_RANGE` | Par `[min, max]` ms de delay antes del submit. Default: `[500, 2000]`. |
| `setInputValue(element, payload)` | Función existente. No se modifica para humanización; solo se agregan tests de cobertura para sus paths. |

---

## Success Criteria

1. Los tests del path `execCommand` pasan y verifican que `execCommand("insertText")` fue invocado con el payload correcto.
2. Los tests del path `contenteditable` pasan y verifican asignación de `textContent` y dispatch de `Event("input")`.
3. Con `__AIBBE_HUMAN_TYPING: true` y `__AIBBE_JITTER_RANGE: [0, 0]`, el test verifica que cada caracter del payload fue insertado individualmente con sus `KeyboardEvent` correspondientes.
4. Con `__AIBBE_HUMAN_TYPING: false` (o sin definir), el comportamiento existente no cambia y todos los tests actuales siguen pasando.
5. `go test ./...` pasa al 100% sin modificar ningún test existente.

---

## Assumptions

- Los `KeyboardEvent` sintéticos mejoran los patrones de timing pero no son suficientes por sí solos para engañar a reCAPTCHA Enterprise. La humanización es una capa de mitigación, no una garantía.
- El aislamiento de sesión (perfil de Chrome separado, stack dockerizado) complementa esta solución y ya está cubierto por la misión `dockerized-aibbe-stack-isolated-chrome`.
- El rango de delays es configurable precisamente para poder ajustarlo empíricamente si Google actualiza sus heurísticas.

---

## Out of Scope

- Implementar movimiento de mouse sintético o scroll.
- Configurar la API key de AI Studio como alternativa a NotebookLM.
- Modificar el daemon o el CLI para controlar la humanización desde afuera (el flag vive en `window.__AIBBE_*` del browser).
