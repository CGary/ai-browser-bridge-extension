---
work_package_id: WP02
title: Tests — Cobertura y Humanización
dependencies:
- WP01
requirement_refs:
- FR-003
- FR-004
- FR-005
- FR-006
- FR-007
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T001
- T002
- T007
- T008
- T009
- T010
agent: "gemini"
shell_pid: "171365"
history:
- date: '2026-04-18'
  event: created
authoritative_surface: extension_handshake_test.go
execution_mode: code_change
owned_files:
- extension_handshake_test.go
tags: []
---

# WP02 — Tests: Cobertura y Humanización

## Branch Strategy

- **Planning base**: `main` (después de que WP01 esté mergeado)
- **Merge target**: `main`
- **Nota**: Este WP depende de WP01. Iniciar solo cuando WP01 está mergeado a `main`.
- **Comando de implementación**: `spec-kitty agent action implement WP02 --agent <name>`

## Objective

Agregar 6 tests a `extension_handshake_test.go`:
- **T001–T002**: Cobertura de los paths de `setInputValue` sin tests (execCommand y contenteditable).
- **T007–T010**: Verificación del comportamiento de `typeWithJitter` e `injectAndSubmit` humanizado.

**Solo se modifica `extension_handshake_test.go`. `content.js` ya fue modificado por WP01.**

## Context

Con WP01 integrado, `content.js` tiene:
- `sleep(ms)`, `randomBetween(min, max)`, `typeWithJitter(element, text, range)`
- `injectAndSubmit` con check `__AIBBE_HUMAN_TYPING`

Los tests existentes cubren el **native setter path** de `setInputValue` pero no el `execCommand` path ni el `contenteditable` path. Los tests de humanización son completamente nuevos.

Todos los tests siguen el patrón `runNodeJSON` del test file existente.

---

## Struct extensions para `nodeResult`

Antes de implementar los tests, verificar qué campos ya existen en `nodeResult` y agregar solo los necesarios. Los campos nuevos requeridos son:

```go
// Agregar a nodeResult:
ExecCalls           []execCall  `json:"execCalls"`
SetterCalled        bool        `json:"setterCalled"`
TextContentAssigned string      `json:"textContentAssigned"`
Events              []evtRecord `json:"events"`
InsertedChars       []string    `json:"insertedChars"`
KeyEvents           []keyEvent  `json:"keyEvents"`
EventLog            []string    `json:"eventLog"`
SubmitClicks        int         `json:"submitClicks"`

// Nuevos tipos auxiliares:
type execCall struct {
    Cmd string `json:"cmd"`
    Val any    `json:"val"`
}

type evtRecord struct {
    Type   string `json:"type"`
    Bubbles bool  `json:"bubbles"`
}

type keyEvent struct {
    Type string `json:"type"`
    Key  string `json:"key"`
}
```

Agregar estos tipos al package-level del test file (junto a los tipos existentes como `extensionManifest`).

---

## Subtask T001 — Test path execCommand de `setInputValue`

**Nombre del test**: `TestExtensionContent_SetInputValue_ExecCommandPath`

**Purpose**: Verificar que cuando `document.execCommand` está disponible, `setInputValue` lo usa para insertar el payload (selectAll + insertText) y no usa el native setter.

**Lógica del harness Node.js**:
```js
const execCalls = [];
let setterCalled = false;

class FakeInput {
  constructor() {}
  focus() {}
  dispatchEvent() { return true; }
  // Sin select() — fuerza el sub-path execCommand("selectAll")
  // Sin "value" property — si se accediera al native setter, lo detectaríamos
}

// Spy sobre el setter: si alguien llama nativeSetter.call(element, ...), registrarlo
// En Node.js, no hay HTMLTextAreaElement nativo.
// No exponer window.HTMLTextAreaElement con prototype.value.set → nativeSetter será undefined

global.window = {
  requestAnimationFrame: (cb) => cb(),
  __AIBBE_SETTLE_MS: 0,
  // Sin HTMLTextAreaElement con native setter
};
global.requestAnimationFrame = (cb) => cb();

global.document = {
  execCommand(cmd, _, val) {
    execCalls.push({ cmd, val: val ?? null });
    return true;
  },
  querySelector(selector) {
    if (selector.includes('textarea') || selector.includes('contenteditable')) return new FakeInput();
    if (selector.includes('button')) return { click() {} };
    return null;
  },
  querySelectorAll(selector) {
    if (selector.includes('to-user-container')) return [responseContainer];
    return [];
  },
};

// responseContainer con estado "ready" para que waitForAIResponse complete
const responseText = {
  textContent: "ok", innerText: "ok",
  cloneNode() { return this; },
  querySelectorAll() { return []; },
};
const responseContainer = {
  querySelector(sel) {
    if (sel.includes('message-text-content')) return responseText;
    if (sel.includes('thinking')) return null;
    if (sel.includes('message-actions')) return {};
    return null;
  },
};

let observerCallback = null;
global.MutationObserver = class {
  constructor(cb) { observerCallback = cb; }
  observe() {}
  disconnect() {}
};
global.Event = class { constructor(type, opts={}) { this.type=type; this.bubbles=opts.bubbles??false; } };
global.console = { log(){}. warn(){}, error(){} };

let onMessageListener = null;
const contentResponses = [];
global.chrome = {
  runtime: {
    sendMessage() {},
    onMessage: { addListener(fn) { onMessageListener = fn; } },
  },
};

require(path.resolve(process.cwd(), "extension/content.js"));
const sendResponse = (r) => contentResponses.push(r);
onMessageListener({ cmd: "generate", payload: "hello" }, {}, sendResponse);

// Trigger observer to resolve waitForAIResponse
setTimeout(() => {
  observerCallback([]);
  setTimeout(() => {
    process.stdout.write(JSON.stringify({
      execCalls,
      setterCalled,
      contentResponses,
    }));
  }, 0);
}, 0);
```

**Assertions Go**:
```go
// 1. execCommand("insertText", "hello") fue llamado
foundInsert := false
for _, c := range result.ExecCalls {
    if c.Cmd == "insertText" && fmt.Sprint(c.Val) == "hello" {
        foundInsert = true
        break
    }
}
if !foundInsert {
    t.Fatalf("expected execCommand insertText with payload 'hello', got %v", result.ExecCalls)
}

// 2. execCommand("selectAll") fue llamado (porque FakeInput no tiene select())
foundSelectAll := false
for _, c := range result.ExecCalls {
    if c.Cmd == "selectAll" {
        foundSelectAll = true
        break
    }
}
if !foundSelectAll {
    t.Fatalf("expected execCommand selectAll, got %v", result.ExecCalls)
}

// 3. native setter no fue invocado
if result.SetterCalled {
    t.Fatal("expected native setter NOT called when execCommand is available")
}
```

---

## Subtask T002 — Test path contenteditable de `setInputValue`

**Nombre del test**: `TestExtensionContent_SetInputValue_ContenteditablePath`

**Purpose**: Verificar que cuando `document.execCommand` NO está disponible y el input es un div contenteditable, `setInputValue` asigna `textContent` y despacha `Event("input", { bubbles: true })`.

**Lógica del harness**:
```js
const events = [];
let textContentValue = "";

const fakeDiv = {
  getAttribute(name) { return name === "contenteditable" ? "true" : null; },
  focus() {},
  dispatchEvent(e) { events.push({ type: e.type, bubbles: e.bubbles }); return true; },
  get textContent() { return textContentValue; },
  set textContent(v) { textContentValue = v; },
  // Sin "value" property
};

global.window = {
  requestAnimationFrame: (cb) => cb(),
  __AIBBE_SETTLE_MS: 0,
  // Sin HTMLTextAreaElement con native setter
};
global.requestAnimationFrame = (cb) => cb();

global.document = {
  // Sin execCommand → setInputValue toma path 3
  querySelector(selector) {
    if (selector.includes('textarea') || selector.includes('contenteditable')) return fakeDiv;
    if (selector.includes('button')) return { click() {} };
    return null;
  },
  querySelectorAll(selector) {
    if (selector.includes('to-user-container')) return [responseContainer];
    return [];
  },
};

// ... (resto del harness: responseContainer, MutationObserver, chrome, etc.)
```

**Assertions Go**:
```go
// 1. textContent fue asignado con el payload
if result.TextContentAssigned != "hello" {
    t.Fatalf("textContent = %q, want hello", result.TextContentAssigned)
}

// 2. Event("input", { bubbles: true }) fue despachado
found := false
for _, ev := range result.Events {
    if ev.Type == "input" && ev.Bubbles {
        found = true
        break
    }
}
if !found {
    t.Fatal("expected bubbling input event dispatched after textContent assignment")
}
```

---

## Subtask T007 — Test char-by-char insertion

**Nombre del test**: `TestExtensionContent_HumanTyping_InsertsCharsOneByOne`

**Purpose**: Verificar que con `__AIBBE_HUMAN_TYPING: true`, `typeWithJitter` inserta el payload caracter-por-caracter via `execCommand("insertText", char)`.

**Harness base** (compartido con T008 y T009 — puede refactorizarse en una función helper JS):
```js
const insertedChars = []; // chars insertados via execCommand("insertText", char)
const keyEvents = [];
let submitClicks = 0;

global.KeyboardEvent = class {
  constructor(type, init = {}) {
    keyEvents.push({ type, key: init.key });
    this.type = type; this.key = init.key; this.bubbles = init.bubbles ?? false;
  }
};

global.document = {
  execCommand(cmd, _, val) {
    if (cmd === "insertText") insertedChars.push(val);
    return true;
  },
  querySelector(selector) {
    if (selector.includes('textarea') || selector.includes('contenteditable')) return fakeInput;
    if (selector.includes('button')) return { click() { submitClicks++; } };
    return null;
  },
  querySelectorAll(selector) {
    if (selector.includes('to-user-container')) return [responseContainer];
    return [];
  },
};

const fakeInput = { focus() {}, dispatchEvent() { return true; } };

global.window = {
  requestAnimationFrame: (cb) => cb(),
  __AIBBE_SETTLE_MS: 0,
  __AIBBE_HUMAN_TYPING: true,
  __AIBBE_JITTER_RANGE: [0, 0],
  __AIBBE_SUBMIT_DELAY_RANGE: [0, 0],
};
global.requestAnimationFrame = (cb) => cb();

// ... responseContainer, MutationObserver, Event, chrome como en tests existentes
```

**Assertions Go** (payload = "hi"):
```go
wantChars := []string{"h", "i"}
if len(result.InsertedChars) != len(wantChars) {
    t.Fatalf("insertedChars = %v, want %v", result.InsertedChars, wantChars)
}
for i, want := range wantChars {
    if result.InsertedChars[i] != want {
        t.Fatalf("insertedChars[%d] = %q, want %q", i, result.InsertedChars[i], want)
    }
}
if result.SubmitClicks != 1 {
    t.Fatalf("submitClicks = %d, want 1", result.SubmitClicks)
}
if got := result.ContentResponses[0]["status"]; got != "success" {
    t.Fatalf("response status = %v, want success", got)
}
```

**Nota**: `typeWithJitter` también llama `execCommand("selectAll")` y `execCommand("delete")` en la fase de limpieza. El harness filtra `insertedChars` solo con `cmd === "insertText"` para no confundir.

---

## Subtask T008 — Test KeyboardEvent dispatch per char

**Nombre del test**: `TestExtensionContent_HumanTyping_DispatchesKeyboardEventsPerChar`

**Purpose**: Verificar que por cada caracter se emiten exactamente `keydown`, `keypress`, `keyup` en ese orden.

**Setup**: Harness base con `payload = "ab"` (2 chars → 6 KeyboardEvents).

**Assertions Go**:
```go
wantEvents := []struct{ Type, Key string }{
    {"keydown", "a"}, {"keypress", "a"}, {"keyup", "a"},
    {"keydown", "b"}, {"keypress", "b"}, {"keyup", "b"},
}
if len(result.KeyEvents) != len(wantEvents) {
    t.Fatalf("keyEvents count = %d, want %d\ngot: %v", len(result.KeyEvents), len(wantEvents), result.KeyEvents)
}
for i, want := range wantEvents {
    got := result.KeyEvents[i]
    if got.Type != want.Type || got.Key != want.Key {
        t.Fatalf("keyEvents[%d] = {%s %s}, want {%s %s}", i, got.Type, got.Key, want.Type, want.Key)
    }
}
```

---

## Subtask T009 — Test delay pre-submit

**Nombre del test**: `TestExtensionContent_HumanTyping_SleepsBeforeSubmit`

**Purpose**: Verificar que el sleep pre-submit ocurre ANTES del click en el submit button.

**Setup**: Harness con `__AIBBE_SUBMIT_DELAY_RANGE: [50, 50]` (rango fijo no-cero para detectarlo), `__AIBBE_JITTER_RANGE: [0, 0]`.

```js
const eventLog = [];
const submitButton = {
  click() { eventLog.push("submit_click"); },
};

const realSetTimeout = global.setTimeout;
global.setTimeout = (cb, ms) => {
  if (ms > 0) {
    eventLog.push("sleep:" + ms);
    return realSetTimeout(cb, 0); // ejecutar inmediatamente
  }
  return realSetTimeout(cb, ms);
};
```

**Assertions Go**:
```go
sleepIdx, clickIdx := -1, -1
for i, ev := range result.EventLog {
    if strings.HasPrefix(ev, "sleep:") { sleepIdx = i }
    if ev == "submit_click" { clickIdx = i }
}
if sleepIdx == -1 { t.Fatal("expected sleep before submit") }
if clickIdx == -1 { t.Fatal("expected submit_click in event log") }
if sleepIdx >= clickIdx {
    t.Fatalf("sleep must occur BEFORE submit_click: sleepIdx=%d, clickIdx=%d", sleepIdx, clickIdx)
}
```

---

## Subtask T010 — Regression: humanización desactivada

**Nombre del test**: `TestExtensionContent_HumanTyping_DisabledPreservesExistingBehavior`

**Purpose**: Garantizar que con `__AIBBE_HUMAN_TYPING` no definido (o `false`), cero `KeyboardEvent` se emiten y el comportamiento existente es idéntico.

**Setup**: Harness tipo `TestExtensionContent_ProcessesGenerateCommand` existente, pero añadiendo `global.KeyboardEvent` como clase spy.

```js
const keyEvents = [];
global.KeyboardEvent = class {
  constructor(type, init = {}) { keyEvents.push({ type, key: init.key }); }
};

global.window = {
  requestAnimationFrame: (cb) => cb(),
  __AIBBE_SETTLE_MS: 0,
  HTMLTextAreaElement: FakeTextArea,  // native setter path como siempre
  // __AIBBE_HUMAN_TYPING no definido
};
```

**Assertions Go**:
```go
// Sin KeyboardEvents
if len(result.KeyEvents) != 0 {
    t.Fatalf("expected no KeyboardEvents when __AIBBE_HUMAN_TYPING is false, got %v", result.KeyEvents)
}
// Inserción bulk funcionó (input value = payload)
if result.InputValue != "hello" {
    t.Fatalf("inputValue = %q, want hello", result.InputValue)
}
// Button clickeado
if result.ButtonClicks != 1 {
    t.Fatalf("buttonClicks = %d, want 1", result.ButtonClicks)
}
// Response exitosa
if got := result.ContentResponses[0]["status"]; got != "success" {
    t.Fatalf("status = %v, want success", got)
}
```

---

## Definition of Done

- [ ] `TestExtensionContent_SetInputValue_ExecCommandPath` pasando.
- [ ] `TestExtensionContent_SetInputValue_ContenteditablePath` pasando.
- [ ] `TestExtensionContent_HumanTyping_InsertsCharsOneByOne` pasando.
- [ ] `TestExtensionContent_HumanTyping_DispatchesKeyboardEventsPerChar` pasando.
- [ ] `TestExtensionContent_HumanTyping_SleepsBeforeSubmit` pasando.
- [ ] `TestExtensionContent_HumanTyping_DisabledPreservesExistingBehavior` pasando.
- [ ] Tipos auxiliares (`execCall`, `evtRecord`, `keyEvent`) agregados al test file.
- [ ] `go test ./...` pasa al 100%.
- [ ] No se modificó `extension/content.js`.

## Reviewer Guidance

- **T001**: Verificar que el harness no expone `window.HTMLTextAreaElement` con native setter — si lo expone, `setInputValue` tomará path 2 antes de llegar al fallback que aquí no aplica.
- **T002**: Verificar que el elemento no tiene propiedad `"value"` y que `document.execCommand` NO está definido.
- **T007**: `insertedChars` filtra solo `cmd === "insertText"` — la limpieza inicial usa "delete" y "selectAll", que no deben aparecer en `insertedChars`.
- **T008**: El count de keyEvents depende del payload. Con "ab" son 6. Con "hi" son 6. Con "a" son 3.
- **T009**: `__AIBBE_SUBMIT_DELAY_RANGE: [50, 50]` (no `[0,0]`) para que el sleep sea detectable en el eventLog.
- **T010**: El harness debe incluir `FakeTextArea` con native setter para que el path existente funcione.

## Risks

- **Riesgo**: El override de `global.setTimeout` en T009 puede afectar los settle timers del observer. Mitigación: `__AIBBE_SETTLE_MS: 0` hace que el settle use `setTimeout(cb, 0)` — el override de T009 solo captura `ms > 0`, por lo que el settle (0ms) pasa sin interferencia.
- **Riesgo**: En T001, `waitForAIResponse` inicia un `MutationObserver` y espera que `observerCallback` sea invocado con el estado "ready". Si el harness no invoca el callback correctamente, el test puede colgar. Usar el mismo patrón que los tests existentes.

## Activity Log

- 2026-04-18T15:08:50Z – gemini – shell_pid=165878 – Started implementation via action command
- 2026-04-18T15:09:04Z – gemini – shell_pid=165878 – Ready for review
- 2026-04-18T15:12:22Z – gemini – shell_pid=171365 – Started review via action command
- 2026-04-18T15:14:45Z – gemini – shell_pid=171365 – Review passed: 6 new tests added and verified. Coverage for execCommand, contenteditable and humanization is complete.
