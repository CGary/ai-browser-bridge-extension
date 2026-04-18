---
work_package_id: WP01
title: 'Test Coverage: setInputValue Paths Existentes'
dependencies: []
requirement_refs:
- FR-005
- FR-006
planning_base_branch: main
merge_target_branch: main
branch_strategy: WP01 se ejecuta en un worktree aislado basado en main. Al completarse, se hace PR y merge a main antes de que WP03 inicie.
subtasks:
- T001
- T002
history:
- date: '2026-04-18'
  event: created
authoritative_surface: extension_handshake_test.go
execution_mode: code_change
owned_files:
- extension_handshake_test.go
tags: []
---

# WP01 — Test Coverage: setInputValue Paths Existentes

## Branch Strategy

- **Planning base**: `main`
- **Merge target**: `main`
- **Nota**: Este WP se ejecuta en un worktree aislado en base a `main`. Al completarse, hacer PR y merge a `main`. WP03 depende de que este WP esté mergeado.
- **Comando de implementación**: `spec-kitty agent action implement WP01 --agent <name>`

## Objective

Cubrir con tests los paths de `setInputValue` en `extension/content.js` que actualmente no tienen ningún test. Los tests existentes en `extension_handshake_test.go` siempre fuerzan el **native setter path** (segundo branch) porque exponen `window.HTMLTextAreaElement` con un descriptor `value` en el prototype. Los otros dos paths — `execCommand` y `contenteditable` — nunca se ejecutan en el harness actual.

**Solo se modifica `extension_handshake_test.go`. No hay cambios en `content.js`.**

## Context

`setInputValue` tiene 3 paths de inyección (en orden de evaluación):

```
1. execCommand path:  if (typeof document.execCommand === "function")
   → inputElement.select?.() OR execCommand("selectAll")
   → execCommand("insertText", false, payload)
   → return

2. native setter path: if (nativeSetter && "value" in inputElement)
   → nativeSetter.call(inputElement, payload)
   → dispatchEvent(new Event("input", { bubbles: true }))
   → return

3. contenteditable path: if (getAttribute("contenteditable") === "true")
   → textContent = payload
   → dispatchEvent(new Event("input", { bubbles: true }))
   → return
```

El path 2 (native setter) ya está cubierto por los tests existentes que usan `FakeTextArea` con `Object.defineProperty` en su prototype. Los paths 1 y 3 no tienen cobertura.

## Subtask T001 — Test path execCommand

**Purpose**: Verificar que cuando `document.execCommand` está disponible, `setInputValue` lo usa para insertar el payload (vía `selectAll` + `insertText`), y no usa el native setter.

**Nombre del test**: `TestExtensionContent_SetInputValue_ExecCommandPath`

**Lógica del harness**:

El harness Node.js debe:
1. Definir `global.document.execCommand` como una función espía que registra cada llamada.
2. Proveer un elemento input (FakeTextArea) que NO tiene el método `select()` — así se toma el sub-path `execCommand("selectAll")`.
3. **No** definir `window.HTMLTextAreaElement.prototype` con un descriptor `value` — así el native setter es `undefined` y el path 2 nunca se alcanza (pero tampoco importa porque `execCommand` ya retorna en path 1).
4. Tener submit button y observer configurados para que `injectAndSubmit` complete sin error (la verificación principal es sobre setInputValue, no sobre el submit).

```js
const execCalls = [];
let setterCalled = false;

class FakeContentDiv {
  constructor() { this._text = ""; }
  // No select() method
  focus() {}
  dispatchEvent() { return true; }
}

global.window = {
  requestAnimationFrame: (cb) => cb(),
  __AIBBE_SETTLE_MS: 0,
  // NO HTMLTextAreaElement con native setter
};

global.document = {
  execCommand(cmd, _, val) {
    execCalls.push({ cmd, val: val ?? null });
    return true;
  },
  querySelector(selector) {
    if (/* INPUT selector */) return new FakeContentDiv();
    if (/* SUBMIT selector */) return { click() {} };
    return null;
  },
  querySelectorAll(selector) {
    if (/* RESPONSE_CONTAINER */) return [responseContainer];
    return [];
  },
};
```

**Assertions** (Go):
```go
// 1. execCommand("insertText", payload) fue llamado
if !containsExecCall(result.ExecCalls, "insertText", "my payload") {
    t.Fatal("expected execCommand insertText with payload")
}

// 2. Si el elemento no tiene select(), se llama selectAll primero
if !containsExecCall(result.ExecCalls, "selectAll", nil) {
    t.Fatal("expected execCommand selectAll before insertText")
}

// 3. El setter nativo NO fue usado (setterCalled must be false)
if result.SetterCalled {
    t.Fatal("expected native setter NOT to be called when execCommand is available")
}
```

**Struct `nodeResult` adicional** (si no están): agrega `ExecCalls []execCall` y `SetterCalled bool` al struct `nodeResult` en el test file. Definir:
```go
type execCall struct {
    Cmd string `json:"cmd"`
    Val any    `json:"val"`
}
```

**Notas de implementación**:
- El harness también necesita un `MutationObserver` fake y un `responseContainer` para que `waitForAIResponse` resuelva. Usar `__AIBBE_SETTLE_MS: 0` y un `observerCallback` que se invoca manualmente como en los tests existentes.
- La respuesta del observer puede ser cualquier texto válido — el foco del test es la inyección, no la extracción.
- El harness debe incluir `global.requestAnimationFrame` para que `waitForNextFrame()` funcione.

---

## Subtask T002 — Test path contenteditable

**Purpose**: Verificar que cuando `document.execCommand` NO está disponible y el elemento es un `div[contenteditable="true"]` sin la propiedad `value`, `setInputValue` asigna `textContent` y despacha `Event("input", { bubbles: true })`.

**Nombre del test**: `TestExtensionContent_SetInputValue_ContenteditablePath`

**Lógica del harness**:

```js
const events = [];
let textContentAssigned = "";

const fakeDiv = {
  // contenteditable = "true"
  getAttribute(name) {
    if (name === "contenteditable") return "true";
    return null;
  },
  focus() {},
  dispatchEvent(e) {
    events.push({ type: e.type, bubbles: e.bubbles });
    return true;
  },
  // NO propiedad "value" — así el path 2 (native setter) falla
  set textContent(val) { textContentAssigned = val; },
  get textContent() { return textContentAssigned; },
};

global.window = {
  requestAnimationFrame: (cb) => cb(),
  __AIBBE_SETTLE_MS: 0,
  // NO HTMLTextAreaElement — nativeSetter será undefined
};

global.document = {
  // NO execCommand — path 1 no se toma
  querySelector(selector) {
    if (/* INPUT */) return fakeDiv;
    if (/* SUBMIT */) return { click() {} };
    return null;
  },
  querySelectorAll(selector) { ... }
};
```

**Assertions** (Go):
```go
// 1. textContent fue asignado con el payload
if result.TextContentAssigned != "my payload" {
    t.Fatalf("textContent = %q, want %q", result.TextContentAssigned, "my payload")
}

// 2. Event("input", { bubbles: true }) fue despachado
inputEventFound := false
for _, ev := range result.Events {
    if ev.Type == "input" && ev.Bubbles {
        inputEventFound = true
        break
    }
}
if !inputEventFound {
    t.Fatal("expected bubbling input event dispatched after textContent assignment")
}
```

**Notas**:
- El elemento no debe tener `"value"` en sus propiedades (ni directamente ni via `Object.getOwnPropertyDescriptor` en su prototype) para asegurar que el path 2 falla y se llega al path 3.
- `global.window` no debe tener `HTMLTextAreaElement` definido, o si lo tiene, su prototype no debe tener descriptor `value.set`.

---

## Definition of Done

- [ ] `TestExtensionContent_SetInputValue_ExecCommandPath` agregado y pasando.
- [ ] `TestExtensionContent_SetInputValue_ContenteditablePath` agregado y pasando.
- [ ] `go test ./...` pasa al 100% (incluyendo todos los tests existentes).
- [ ] Los 3 paths de `setInputValue` tienen cobertura en el test file.
- [ ] No se modificó `content.js`.

## Reviewer Guidance

- Verificar que el harness de T001 fuerza el path 1 (execCommand) y no accidentalmente el path 2.
- Verificar que el harness de T002 fuerza el path 3 (contenteditable) — el elemento no debe tener `"value"` ni `document.execCommand`.
- Correr `go test ./... -run TestExtensionContent_SetInputValue` para aislar los nuevos tests.
- Confirmar que los tests existentes (`TestExtensionContent_ProcessesGenerateCommand`) siguen pasando sin cambios.

## Risks

- **Riesgo**: El harness de T001 define `document.execCommand` pero también tiene un `FakeTextArea` con `_value` interno. Si hay un error de configuración, el native setter podría ser alcanzado antes de execCommand. Mitigación: verificar que el setter NO fue llamado (assertion explícita).
- **Riesgo**: `Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, "value")` en Node.js cuando `HTMLTextAreaElement` no está definido en `window` → `undefined?.set` → `undefined`. Verificar que no causa un error de runtime.
