---
work_package_id: WP01
title: Implementar Humanización DOM
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-008
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T003
- T004
- T005
- T006
history:
- date: '2026-04-18'
  event: created
authoritative_surface: extension/content.js
execution_mode: code_change
owned_files:
- extension/content.js
tags: []
---

# WP01 — Implementar Humanización DOM

## Branch Strategy

- **Planning base**: `main`
- **Merge target**: `main`
- **Nota**: Worktree aislado basado en `main`. Al completarse, PR y merge a `main`. WP02 no puede iniciar hasta que este WP esté mergeado.
- **Comando de implementación**: `spec-kitty agent action implement WP01 --agent <name>`

## Objective

Agregar tres funciones nuevas (`sleep`, `randomBetween`, `typeWithJitter`) a `extension/content.js` y modificar `injectAndSubmit` para activar la humanización cuando `window.__AIBBE_HUMAN_TYPING` es `true`.

**Solo se modifica `extension/content.js`. No hay cambios en el test file.**

## Context

Google detecta la extensión como bot porque la inyección de texto ocurre de forma instantánea y sin eventos de teclado. La solución es simular tipeo humano: caracter-por-caracter, con `KeyboardEvent` por cada tecla y delays aleatorios entre caracteres.

Archivo destino: `extension/content.js`. Vanilla JS sin bundler. Todas las adiciones son funciones standalone antes del bloque `chrome.runtime.onMessage.addListener`.

**Banderas de control (en `window`):**

| Flag | Tipo | Default | Descripción |
|------|------|---------|-------------|
| `__AIBBE_HUMAN_TYPING` | boolean | `false` | Activa tipeo humanizado |
| `__AIBBE_JITTER_RANGE` | `[number, number]` | `[40, 120]` | ms de delay entre chars |
| `__AIBBE_SUBMIT_DELAY_RANGE` | `[number, number]` | `[500, 2000]` | ms de delay antes del submit |

---

## Subtask T003 — Agregar `sleep(ms)`

**Purpose**: Helper async que wraps `setTimeout` en una Promise.

**Ubicación**: Agregar en `extension/content.js`, después de la función `waitForNextFrame` (~línea 26).

```javascript
function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
```

**Notas**: 2 líneas, puramente aditivo. No modifica ninguna función existente.

---

## Subtask T004 — Agregar `randomBetween(min, max)`

**Purpose**: Retorna un entero aleatorio en `[min, max]` inclusive.

**Ubicación**: Inmediatamente después de `sleep`.

```javascript
function randomBetween(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}
```

**Notas**: Con `min === max === 0` (modo test), retorna siempre `0` — permite tests determinísticos sin modificar código de producción.

---

## Subtask T005 — Implementar `typeWithJitter(element, text, range)`

**Purpose**: Función async que inserta `text` en `element` caracter-por-caracter con `KeyboardEvent` dispatch y delays aleatorios entre caracteres.

**Ubicación**: Después de `randomBetween`.

**Implementación completa**:
```javascript
async function typeWithJitter(element, text, range) {
  element.focus?.();

  // Clear existing content before typing character by character.
  if (typeof document.execCommand === "function") {
    document.execCommand("selectAll", false, null);
    document.execCommand("delete", false, null);
  } else if (element.getAttribute?.("contenteditable") === "true") {
    element.textContent = "";
  } else {
    const nativeSetter = Object.getOwnPropertyDescriptor(
      window.HTMLTextAreaElement?.prototype,
      "value",
    )?.set;
    if (nativeSetter) {
      nativeSetter.call(element, "");
    }
  }

  for (const char of text) {
    element.dispatchEvent(new KeyboardEvent("keydown", { key: char, bubbles: true }));
    element.dispatchEvent(new KeyboardEvent("keypress", { key: char, bubbles: true }));

    if (typeof document.execCommand === "function") {
      document.execCommand("insertText", false, char);
    } else if (element.getAttribute?.("contenteditable") === "true") {
      element.textContent = (element.textContent ?? "") + char;
      element.dispatchEvent(new Event("input", { bubbles: true }));
    } else {
      const nativeSetter = Object.getOwnPropertyDescriptor(
        window.HTMLTextAreaElement?.prototype,
        "value",
      )?.set;
      if (nativeSetter) {
        nativeSetter.call(element, (element.value ?? "") + char);
        element.dispatchEvent(new Event("input", { bubbles: true }));
      }
    }

    element.dispatchEvent(new KeyboardEvent("keyup", { key: char, bubbles: true }));
    await sleep(randomBetween(range[0], range[1]));
  }
}
```

**Notas críticas**:
- El **paso de limpieza inicial** es obligatorio: el campo debe estar vacío antes del loop. Sin él, si el campo tenía texto previo, se concatenarían caracteres.
- La limpieza usa la misma lógica de 3 paths que `setInputValue` (execCommand → contenteditable → native setter).
- `document.execCommand("delete")` borra el contenido seleccionado por `selectAll`.
- Orden de eventos por caracter: `keydown` → inserción → `keyup`. `keypress` se incluye entre keydown e inserción (obsoleto en DOM4 pero aún emitido por navegadores reales).
- `KeyboardEvent` necesita `bubbles: true`.

---

## Subtask T006 — Modificar `injectAndSubmit` con flag `__AIBBE_HUMAN_TYPING`

**Purpose**: Bifurcar `injectAndSubmit` para usar `typeWithJitter` cuando `window.__AIBBE_HUMAN_TYPING` es `true`, preservando el comportamiento actual cuando es `false` o no está definido.

**Función actual** (en `content.js` ~línea 127):
```javascript
async function injectAndSubmit(payload) {
  const inputElement = document.querySelector(SELECTORS.INPUT);
  if (!inputElement) {
    return { status: "error", error: "input_not_found" };
  }

  setInputValue(inputElement, payload);
  await waitForNextFrame();

  const submitButton = document.querySelector(SELECTORS.SUBMIT_BUTTON);
  if (!submitButton) {
    return { status: "error", error: "submit_button_not_found" };
  }

  submitButton.click();
  return { status: "success" };
}
```

**Función modificada**:
```javascript
async function injectAndSubmit(payload) {
  const inputElement = document.querySelector(SELECTORS.INPUT);
  if (!inputElement) {
    return { status: "error", error: "input_not_found" };
  }

  if (window.__AIBBE_HUMAN_TYPING) {
    const jitterRange = window.__AIBBE_JITTER_RANGE ?? [40, 120];
    await typeWithJitter(inputElement, payload, jitterRange);
  } else {
    setInputValue(inputElement, payload);
    await waitForNextFrame();
  }

  const submitButton = document.querySelector(SELECTORS.SUBMIT_BUTTON);
  if (!submitButton) {
    return { status: "error", error: "submit_button_not_found" };
  }

  if (window.__AIBBE_HUMAN_TYPING) {
    const submitRange = window.__AIBBE_SUBMIT_DELAY_RANGE ?? [500, 2000];
    await sleep(randomBetween(...submitRange));
  }

  submitButton.click();
  return { status: "success" };
}
```

**Notas**:
- Los tests existentes NO definen `__AIBBE_HUMAN_TYPING`, por lo que toman el `else` branch — comportamiento actual intacto.
- El delay pre-submit ocurre DESPUÉS de verificar que el submit button existe — evita esperar inútilmente si el botón no está.
- `?? [40, 120]` no aplica si el flag es `[0, 0]` (no es null/undefined), garantizando tests determinísticos.

---

## Definition of Done

- [ ] `sleep(ms)` agregado a `extension/content.js`.
- [ ] `randomBetween(min, max)` agregado a `extension/content.js`.
- [ ] `typeWithJitter(element, text, range)` implementado con limpieza inicial + loop de chars + KeyboardEvents + delay.
- [ ] `injectAndSubmit` modificado con `__AIBBE_HUMAN_TYPING` check y delay pre-submit.
- [ ] `go test ./...` pasa al 100% (los tests actuales no definen el flag, siguen usando el path original).
- [ ] No se modificó `extension_handshake_test.go`.

## Reviewer Guidance

- Verificar el paso de **limpieza inicial** en `typeWithJitter` (fácil de omitir, causa concatenación).
- Verificar que `injectAndSubmit` sin `__AIBBE_HUMAN_TYPING` produce idéntico comportamiento al anterior.
- Correr `go test ./...` y confirmar que todos los tests existentes pasan.

## Risks

- `document.execCommand("delete")` podría no estar disponible en todos los contextos. La limpieza usa 3 paths como fallback — si execCommand falla, hay alternativa.
- `window.__AIBBE_JITTER_RANGE ?? [40, 120]`: verificar que `?? [40, 120]` no sobreescribe `[0, 0]` (no debe, porque `[0,0]` no es null/undefined).
