# Contracts & Function Signatures

**Mission**: `humanizacion-dom-jitter-anti-bot-01KPGACX`

---

## Funciones nuevas en `extension/content.js`

### `sleep(ms: number): Promise<void>`
Espera `ms` milisegundos. Wraps `setTimeout`.

### `randomBetween(min: number, max: number): number`
Retorna un entero aleatorio en `[min, max]` inclusive. Usa `Math.random()`.

### `typeWithJitter(element: HTMLElement, text: string, range: [number, number]): Promise<void>`
Inserta `text` en `element` caracter-por-caracter con delays y KeyboardEvents.

**Contrato**:
- Por cada char en `text`:
  1. `element.dispatchEvent(new KeyboardEvent("keydown", { key: char, bubbles: true }))`
  2. `element.dispatchEvent(new KeyboardEvent("keypress", { key: char, bubbles: true }))`
  3. Insertar char: `document.execCommand("insertText", false, char)` si disponible, sino `element.value += char` + dispatch `Event("input")`
  4. `element.dispatchEvent(new KeyboardEvent("keyup", { key: char, bubbles: true }))`
  5. `await sleep(randomBetween(range[0], range[1]))`
- Resuelve cuando todos los caracteres han sido insertados.

---

## Modificación a `injectAndSubmit` en `extension/content.js`

```
// Antes (simplificado):
setInputValue(inputElement, payload)
await waitForNextFrame()
submitButton.click()

// Después (con humanización):
if (window.__AIBBE_HUMAN_TYPING) {
  const jitterRange = window.__AIBBE_JITTER_RANGE ?? [40, 120]
  await typeWithJitter(inputElement, payload, jitterRange)
  const submitRange = window.__AIBBE_SUBMIT_DELAY_RANGE ?? [500, 2000]
  await sleep(randomBetween(...submitRange))
} else {
  setInputValue(inputElement, payload)
  await waitForNextFrame()
}
submitButton.click()
```

---

## Flags de configuración (en `window`)

| Flag | Tipo | Default | Descripción |
|------|------|---------|-------------|
| `__AIBBE_HUMAN_TYPING` | `boolean` | `false` | Activa tipeo humanizado |
| `__AIBBE_JITTER_RANGE` | `[number, number]` | `[40, 120]` | Delay en ms entre caracteres |
| `__AIBBE_SUBMIT_DELAY_RANGE` | `[number, number]` | `[500, 2000]` | Delay en ms antes del submit |
| `__AIBBE_TIMEOUT` | `number` | `150000` | Ya existente. Sin cambios. |
| `__AIBBE_SETTLE_MS` | `number` | `750` | Ya existente. Sin cambios. |

---

## Tests: nuevas funciones helper en `extension_handshake_test.go`

No se agregan helpers Go. Los tests reutilizan el patrón `runNodeJSON` existente con harness JS inline.

**Nuevo harness para execCommand path**:
```js
const execCalls = [];
global.document = {
  execCommand(cmd, _, val) { execCalls.push({ cmd, val }); return true; },
  querySelector(...) { ... }
};
// window sin HTMLTextAreaElement prototype value → fuerza execCommand branch
```

**Nuevo harness para humanización**:
```js
const insertedChars = [];
const keyEvents = [];
global.KeyboardEvent = class {
  constructor(type, init) { keyEvents.push({ type, key: init?.key }); }
};
global.document.execCommand = (cmd, _, val) => {
  if (cmd === "insertText") insertedChars.push(val);
  return true;
};
global.window.__AIBBE_HUMAN_TYPING = true;
global.window.__AIBBE_JITTER_RANGE = [0, 0];
global.window.__AIBBE_SUBMIT_DELAY_RANGE = [0, 0];
```
