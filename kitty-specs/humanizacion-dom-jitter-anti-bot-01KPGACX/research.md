# Research: Humanización DOM con Jitter Anti-Bot

**Mission**: `humanizacion-dom-jitter-anti-bot-01KPGACX`
**Date**: 2026-04-18

---

## 1. Cobertura del path `execCommand`

**Decisión**: Los tests existentes usan el fallback de native setter porque no definen `document.execCommand`. Para cubrir el path principal, el harness de Node.js debe exponer `global.document.execCommand` como una función espía (tracking de llamadas) y NO exponer `global.window.HTMLTextAreaElement` con la propiedad `value` en el prototype — de ese modo `setInputValue` tomará el branch de `execCommand`.

**Rationale**: `setInputValue` en `content.js` evalúa `typeof document.execCommand === "function"` primero. Si está definido, usa `selectAll + insertText` y retorna sin llegar al native setter. El test debe verificar: (1) `execCommand("selectAll", false, null)`, (2) `execCommand("insertText", false, payload)`, y (3) que el native setter no fue invocado.

**Implementación en harness**:
```js
global.document = {
  execCommand(cmd, _b, val) {
    execCalls.push({ cmd, val });
    return true;
  },
  // ...querySelector
};
// No exponer HTMLTextAreaElement con native setter — fuerza el path execCommand
global.window = { requestAnimationFrame: cb => cb() };
```

---

## 2. Cobertura del path `contenteditable`

**Decisión**: El harness debe retornar un elemento cuyo `getAttribute("contenteditable")` devuelva `"true"` y que NO tenga `"value"` en sus propiedades, y sin `document.execCommand`. Así `setInputValue` cae al tercer branch.

**Rationale**: El tercer branch en `setInputValue` verifica `inputElement.getAttribute?.("contenteditable") === "true"`. El test debe verificar que `textContent` fue asignado y que se despachó `Event("input", { bubbles: true })`.

---

## 3. Diseño de `typeWithJitter`

**Decisión**: Función async nueva en `content.js`. Inserta el texto caracter-por-caracter: por cada caracter despacha `KeyboardEvent("keydown")`, inserta el caracter vía `document.execCommand("insertText", false, char)` (o asignación directa si execCommand no está disponible), despacha `KeyboardEvent("keyup")`, luego espera `sleep(randomBetween(min, max))`.

**Rationale**: Este patrón reproduce los eventos mínimos que los sistemas anti-bot buscan: variación temporal entre teclas y secuencia keydown→input→keyup. No se necesita `keypress` moderno (obsoleto en la spec de DOM4 pero aún emitido por navegadores — lo incluimos para completitud).

**Alternativas consideradas**:
- Usar `InputEvent` con `inputType: "insertText"`: más fidedigno, pero no mejora la detección de timing.
- Usar `document.execCommand` por caracter vs. modificar `.value` directamente: `execCommand` es preferible porque dispara los mismos listeners que en el path bulk.

**Función `sleep` auxiliar**:
```js
function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}
```
Ya existe un patrón similar en el harness de tests (settle timer). Consistente con el estilo del codebase.

---

## 4. Integración con `injectAndSubmit`

**Decisión**: `injectAndSubmit` chequea `window.__AIBBE_HUMAN_TYPING`. Si es `true`, llama `typeWithJitter` en lugar de `setInputValue`, y agrega `await sleep(randomBetween(...window.__AIBBE_SUBMIT_DELAY_RANGE))` antes del click. El path existente queda intacto.

**Defaults**:
- `window.__AIBBE_JITTER_RANGE`: `[40, 120]` ms
- `window.__AIBBE_SUBMIT_DELAY_RANGE`: `[500, 2000]` ms

---

## 5. Estrategia de tests para humanización (determinismo)

**Decisión**: Los tests de humanización usan `window.__AIBBE_JITTER_RANGE: [0, 0]` y `window.__AIBBE_SUBMIT_DELAY_RANGE: [0, 0]`. Esto elimina aleatoriedad sin modificar el código de producción.

**Para verificar char-by-char**: El harness trackea cada llamada a `execCommand("insertText", ...)` en un array. Al final verifica que el array contiene exactamente los mismos caracteres del payload en el mismo orden.

**Para verificar KeyboardEvent**: El harness expone `global.KeyboardEvent` como una clase spy que registra cada instancia creada con su tipo y propiedad `key`.

---

## 6. Independencia de Lanes

**Decisión**: Lane A (test coverage de paths existentes) y Lane B (implementación de humanización + tests) son independientes:
- Lane A: solo agrega tests a `extension_handshake_test.go`. No toca `content.js`.
- Lane B: agrega `typeWithJitter` + `sleep` a `content.js`, modifica `injectAndSubmit`, y agrega tests a `extension_handshake_test.go` (secciones separadas).
- No hay conflicto de merge: Lane A y Lane B tocan diferentes funciones en el test file y diferentes secciones de `content.js`.
