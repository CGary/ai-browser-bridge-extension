---
work_package_id: WP02
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
branch_strategy: WP02 se ejecuta en un worktree aislado basado en main. Al completarse, PR y merge a main. WP03 depende de que este WP esté mergeado.
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

# WP02 — Implementar Humanización DOM

## Branch Strategy

- **Planning base**: `main`
- **Merge target**: `main`
- **Nota**: Este WP se ejecuta en un worktree aislado en base a `main`. Al completarse, hacer PR y merge a `main`. WP03 depende de que este WP esté mergeado.
- **Comando de implementación**: `spec-kitty agent action implement WP02 --agent <name>`

## Objective

Implementar las tres funciones nuevas (`sleep`, `randomBetween`, `typeWithJitter`) en `extension/content.js` y modificar `injectAndSubmit` para activar la humanización cuando `window.__AIBBE_HUMAN_TYPING` es `true`.

**Solo se modifica `extension/content.js`. No hay cambios en el test file.**

## Context

El problema que se resuelve: Google detecta la extensión como bot porque la inyección de texto ocurre de forma instantánea y sin eventos de teclado. La solución es simular tipeo humano: caracter-por-caracter, con `KeyboardEvent` por cada tecla y delays aleatorios entre caracteres.

Archivo destino: `extension/content.js`. Es vanilla JS sin bundler. Todas las adiciones son funciones standalone antes del bloque `chrome.runtime.onMessage.addListener`.

**Banderas de control (en `window`):**

| Flag | Tipo | Default | Descripción |
|------|------|---------|-------------|
| `__AIBBE_HUMAN_TYPING` | boolean | `false` | Activa tipeo humanizado |
| `__AIBBE_JITTER_RANGE` | `[number, number]` | `[40, 120]` | ms de delay entre chars |
| `__AIBBE_SUBMIT_DELAY_RANGE` | `[number, number]` | `[500, 2000]` | ms de delay antes del submit |

---

## Subtask T003 — Agregar `sleep(ms)`

**Purpose**: Helper async que wraps `setTimeout` en una Promise. Se usa dentro de `typeWithJitter` y en el delay pre-submit.

**Ubicación**: Agregar en `extension/content.js`, después de `waitForNextFrame` (línea ~26).

**Implementación**:
```javascript
function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
```

**Notas**:
- 2 líneas de código.
- No modifica ninguna función existente.
- `setTimeout` ya está disponible en el contexto del content script del navegador y en el harness de Node.js.

---

## Subtask T004 — Agregar `randomBetween(min, max)`

**Purpose**: Retorna un entero aleatorio en el rango `[min, max]` inclusive. Se usa en `typeWithJitter` para el delay entre chars y en el delay pre-submit.

**Ubicación**: Agregar inmediatamente después de `sleep`.

**Implementación**:
```javascript
function randomBetween(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}
```

**Notas**:
- Cuando `min === max === 0` (modo test), retorna siempre `0`. Esto es intencional y permite tests determinísticos.
- No modifica ninguna función existente.

---

## Subtask T005 — Implementar `typeWithJitter(element, text, range)`

**Purpose**: Función async principal de la humanización. Inserta `text` en `element` caracter-por-caracter, despachando `KeyboardEvent` por cada tecla y esperando un delay aleatorio entre caracteres.

**Ubicación**: Agregar después de `randomBetween`.

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
- El paso de **limpieza inicial** es fundamental: antes de tipar char-por-char, el campo debe estar vacío. Sin esto, si el campo tenía texto previo, se concatenarían.
- La limpieza usa la misma lógica de paths que `setInputValue` para ser consistente.
- `document.execCommand("delete")` después de `selectAll` borra el contenido seleccionado. Es el equivalente programático de presionar la tecla Delete.
- El orden de eventos por caracter es: `keydown` → inserción → `keyup`. Se incluye `keypress` entre keydown e inserción para completitud (aunque keypress está técnicamente obsoleto, los navegadores aún lo emiten).
- `KeyboardEvent` requiere `bubbles: true` para que los listeners en ancestros lo reciban.

---

## Subtask T006 — Modificar `injectAndSubmit` con flag `__AIBBE_HUMAN_TYPING`

**Purpose**: Hacer que `injectAndSubmit` use `typeWithJitter` cuando `window.__AIBBE_HUMAN_TYPING` es `true`, y preserve el comportamiento actual cuando es `false` o no está definido.

**Función actual** (simplificada):
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
- El check `window.__AIBBE_HUMAN_TYPING` es evaluado en runtime. Los tests existentes no definen este flag, por lo que toman el path `else` (comportamiento actual sin cambios).
- El delay pre-submit solo ocurre en el path humanizado, DESPUÉS de verificar que el submit button existe. Esto evita esperar si el botón no está disponible.
- `window.__AIBBE_JITTER_RANGE ?? [40, 120]` — el operador `??` usa el default solo si el flag es `null` o `undefined`.

---

## Definition of Done

- [ ] `sleep(ms)` agregado a `extension/content.js`.
- [ ] `randomBetween(min, max)` agregado a `extension/content.js`.
- [ ] `typeWithJitter(element, text, range)` implementado con limpieza inicial, loop de chars con KeyboardEvents, y delay entre chars.
- [ ] `injectAndSubmit` modificado con el check `__AIBBE_HUMAN_TYPING` y delay pre-submit.
- [ ] `go test ./...` pasa al 100% (todos los tests existentes siguen verdes — los tests actuales no definen `__AIBBE_HUMAN_TYPING`, por lo que usan el path existente).
- [ ] No se modificó `extension_handshake_test.go`.

## Reviewer Guidance

- Verificar el paso de **limpieza inicial** en `typeWithJitter` — es fácil olvidar y causaría concatenación si el campo no estaba vacío.
- Verificar que `injectAndSubmit` con `__AIBBE_HUMAN_TYPING: false` (o sin definir) produce exactamente el mismo comportamiento que antes de este WP.
- Correr `go test ./...` y confirmar que todos los tests existentes pasan — especialmente `TestExtensionContent_ProcessesGenerateCommand`.

## Risks

- **Riesgo**: `document.execCommand("delete")` podría no estar disponible en algunos contextos (aunque en Chrome extension content scripts sí está). Si no está disponible, la limpieza podría fallar silenciosamente. Mitigación: el paso de limpieza usa el mismo pattern de 3 paths que `setInputValue`.
- **Riesgo**: `window.__AIBBE_JITTER_RANGE ?? [40, 120]` — si el flag está definido como `[0, 0]` en tests, el `??` no lo pisa (correcto). Verificar que `?? [40, 120]` no sobreescribe `[0, 0]` (no debe porque `[0,0]` no es null/undefined).
