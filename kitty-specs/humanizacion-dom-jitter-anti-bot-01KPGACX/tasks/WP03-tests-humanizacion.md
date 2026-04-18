---
work_package_id: WP03
title: Tests de Humanización
dependencies:
- WP01
- WP02
requirement_refs:
- FR-003
- FR-004
- FR-007
planning_base_branch: main
merge_target_branch: main
branch_strategy: WP03 inicia solo después de que WP01 y WP02 están mergeados a main. Se ejecuta en un worktree aislado. Al completarse, PR y merge a main.
subtasks:
- T007
- T008
- T009
- T010
history:
- date: '2026-04-18'
  event: created
authoritative_surface: extension_handshake_test.go
execution_mode: code_change
owned_files:
- extension_handshake_test.go
tags: []
---

# WP03 — Tests de Humanización

## Branch Strategy

- **Planning base**: `main` (después de que WP01 y WP02 se hayan mergeado)
- **Merge target**: `main`
- **Nota**: Este WP depende de WP01 (test file sin overlaps) y WP02 (implementación en content.js). Comenzar solo cuando ambos están mergeados a `main`.
- **Comando de implementación**: `spec-kitty agent action implement WP03 --agent <name>`

## Objective

Verificar el comportamiento de `typeWithJitter` e `injectAndSubmit` en modo humanizado mediante tests determinísticos en `extension_handshake_test.go`. Los tests usan `window.__AIBBE_JITTER_RANGE: [0, 0]` y `__AIBBE_SUBMIT_DELAY_RANGE: [0, 0]` para eliminar aleatoriedad sin modificar el código de producción.

**Solo se modifica `extension_handshake_test.go`. WP02 ya habrá integrado los cambios en `content.js`.**

## Context

Con WP02 integrado, `content.js` tiene:
- `sleep(ms)` — wraps setTimeout
- `randomBetween(min, max)` — entero aleatorio
- `typeWithJitter(element, text, range)` — async, char-por-char con KeyboardEvents
- `injectAndSubmit` — chequea `__AIBBE_HUMAN_TYPING` y deriva

Los tests de este WP verifican que estas funciones se comportan correctamente a través de la interfaz `onMessage` listener, siguiendo el patrón `runNodeJSON` ya establecido en el test file.

**Estructura del harness base** (compartido por T007, T008, T009):

```js
const insertedChars = [];  // chars pasados a execCommand("insertText", ...)
const keyEvents = [];       // { type, key } por cada KeyboardEvent construido
let submitClicks = 0;
let sleepCalls = [];        // ms pasados a cada sleep (via setTimeout override)

global.KeyboardEvent = class FakeKeyboardEvent {
  constructor(type, init = {}) {
    keyEvents.push({ type, key: init.key });
    this.type = type;
    this.key = init.key;
    this.bubbles = init.bubbles ?? false;
  }
};

const execCalls = [];
global.document = {
  execCommand(cmd, _, val) {
    execCalls.push({ cmd, val: val ?? null });
    if (cmd === "insertText") insertedChars.push(val);
    return true;
  },
  querySelector(selector) { /* devuelve fakeInput o submitButton según selector */ },
  querySelectorAll(selector) { /* devuelve [responseContainer] */ },
};

global.window = {
  requestAnimationFrame: (cb) => cb(),
  __AIBBE_SETTLE_MS: 0,
  __AIBBE_HUMAN_TYPING: true,
  __AIBBE_JITTER_RANGE: [0, 0],
  __AIBBE_SUBMIT_DELAY_RANGE: [0, 0],
  // NO HTMLTextAreaElement native setter (typeWithJitter usa execCommand)
};

// Override setTimeout para capturar delays de sleep()
const realSetTimeout = global.setTimeout;
global.setTimeout = (cb, ms) => {
  if (ms === 0 || ms === undefined) return realSetTimeout(cb, ms);
  sleepCalls.push(ms);
  return realSetTimeout(cb, 0); // ejecutar inmediatamente en tests
};

global.requestAnimationFrame = (cb) => cb();
global.MutationObserver = class FakeMutationObserver { ... };
global.Event = class FakeEvent { ... };
global.chrome = { runtime: { sendMessage() {}, onMessage: { addListener(fn) { ... } } } };
```

**Nota sobre el override de setTimeout**: Llamar `realSetTimeout(cb, 0)` en lugar de `ms` hace que `sleep(0)` resuelva inmediatamente en el test. Con `__AIBBE_JITTER_RANGE: [0,0]` y `randomBetween(0,0) = 0`, el delay siempre es 0, por lo que el override no necesita hacer nada especial — `setTimeout(cb, 0)` ya resuelve en el siguiente tick.

---

## Subtask T007 — Test char-by-char insertion

**Purpose**: Verificar que con `__AIBBE_HUMAN_TYPING: true`, `typeWithJitter` inserta el payload caracter-por-caracter via `execCommand("insertText", char)` y no como string completo.

**Nombre del test**: `TestExtensionContent_HumanTyping_InsertsCharsOneByOne`

**Setup**: Harness base descrito arriba con `payload = "hi"`.

**Assertions**:
```go
// 1. Cada char fue insertado individualmente
wantChars := []string{"h", "i"}
if len(result.InsertedChars) != len(wantChars) {
    t.Fatalf("insertedChars = %v, want %v", result.InsertedChars, wantChars)
}
for i, want := range wantChars {
    if result.InsertedChars[i] != want {
        t.Fatalf("insertedChars[%d] = %q, want %q", i, result.InsertedChars[i], want)
    }
}

// 2. El submit button fue clickeado exactamente una vez
if result.SubmitClicks != 1 {
    t.Fatalf("submitClicks = %d, want 1", result.SubmitClicks)
}

// 3. La respuesta fue exitosa
if got := result.ContentResponses[0]["status"]; got != "success" {
    t.Fatalf("response status = %v, want success", got)
}
```

**Notas de implementación**:
- Payload corto ("hi", "ab", "ok") — suficiente para verificar el patrón sin test verboso.
- El harness también necesita que `typeWithJitter` complete — esto implica que `waitForAIResponse` eventualmente resuelva. Seguir el mismo patrón de los tests existentes: invocar `observerCallback` manualmente con el estado "ready".
- El paso de **limpieza** de `typeWithJitter` llama `execCommand("selectAll") + execCommand("delete")`. Estos también van a `execCalls`. El test debe filtrar `insertedChars` (que solo trackea "insertText") separado de `execCalls`.

---

## Subtask T008 — Test KeyboardEvent dispatch per char

**Purpose**: Verificar que por cada caracter del payload, se emiten exactamente 3 eventos: `keydown`, `keypress`, `keyup`, en ese orden.

**Nombre del test**: `TestExtensionContent_HumanTyping_DispatchesKeyboardEventsPerChar`

**Setup**: Harness base con `payload = "ab"` (2 chars → 6 KeyboardEvents esperados).

**Assertions**:
```go
// Para payload "ab", esperamos:
// [keydown a, keypress a, keyup a, keydown b, keypress b, keyup b]
wantEvents := []struct{ Type, Key string }{
    {"keydown",  "a"},
    {"keypress", "a"},
    {"keyup",    "a"},
    {"keydown",  "b"},
    {"keypress", "b"},
    {"keyup",    "b"},
}

if len(result.KeyEvents) != len(wantEvents) {
    t.Fatalf("keyEvents count = %d, want %d\ngot: %v", len(result.KeyEvents), len(wantEvents), result.KeyEvents)
}

for i, want := range wantEvents {
    got := result.KeyEvents[i]
    if got.Type != want.Type || got.Key != want.Key {
        t.Fatalf("keyEvents[%d] = {%q, %q}, want {%q, %q}", i, got.Type, got.Key, want.Type, want.Key)
    }
}
```

**Notas de implementación**:
- `global.KeyboardEvent` spy debe capturar TODOS los eventos del ciclo, incluyendo los de la fase de limpieza (si la hay). Pero la limpieza usa `execCommand`, no KeyboardEvent. Así que los únicos KeyboardEvents son los del loop de tipeo.
- Si el payload tiene un solo char, se esperan exactamente 3 eventos.
- El struct `keyEvent` necesita ser agregado a `nodeResult` en el test file:
  ```go
  type keyEvent struct {
      Type string `json:"type"`
      Key  string `json:"key"`
  }
  ```
  Y `KeyEvents []keyEvent` al struct `nodeResult`.

---

## Subtask T009 — Test delay pre-submit

**Purpose**: Verificar que con `__AIBBE_HUMAN_TYPING: true`, se llama a `sleep` antes del `submitButton.click()`, y que el delay viene de `__AIBBE_SUBMIT_DELAY_RANGE`.

**Nombre del test**: `TestExtensionContent_HumanTyping_SleepsBeforeSubmit`

**Setup**: Harness base. Para poder verificar el orden (sleep ANTES del click), capturar eventos en un log ordenado:

```js
const eventLog = [];

const submitButton = {
  click() { eventLog.push("submit_click"); },
};

// Override setTimeout para el sleep pre-submit:
const realSetTimeout = global.setTimeout;
global.setTimeout = (cb, ms) => {
  if (ms > 0) {
    eventLog.push(`sleep:${ms}`);
    return realSetTimeout(cb, 0); // ejecutar inmediatamente
  }
  return realSetTimeout(cb, ms);
};
```

Con `__AIBBE_SUBMIT_DELAY_RANGE: [50, 50]` (rango fijo no-cero para este test):

```go
// 1. El eventLog contiene "sleep:50" ANTES de "submit_click"
sleepIdx := -1
clickIdx := -1
for i, ev := range result.EventLog {
    if strings.HasPrefix(ev, "sleep:") {
        sleepIdx = i
    }
    if ev == "submit_click" {
        clickIdx = i
    }
}
if sleepIdx == -1 {
    t.Fatal("expected a sleep call before submit")
}
if clickIdx == -1 {
    t.Fatal("expected submit_click in event log")
}
if sleepIdx >= clickIdx {
    t.Fatalf("sleep must occur before submit_click: sleepIdx=%d, clickIdx=%d", sleepIdx, clickIdx)
}
```

**Notas**:
- Este test usa `__AIBBE_SUBMIT_DELAY_RANGE: [50, 50]` (rango fijo de 50ms) en lugar de `[0,0]`, para poder detectar el sleep call con un valor concreto en el eventLog.
- El harness de chars también genera sleeps (uno por char con `__AIBBE_JITTER_RANGE`). Mantener `__AIBBE_JITTER_RANGE: [0, 0]` para que los sleeps de chars sean 0 y no se confundan con el sleep pre-submit de 50ms.
- El `eventLog` como slice de strings permite verificar el ORDEN de las operaciones, no solo su presencia.

---

## Subtask T010 — Regression: humanización desactivada preserva comportamiento existente

**Purpose**: Garantizar que con `__AIBBE_HUMAN_TYPING: false` (o sin definir), el comportamiento de `injectAndSubmit` es idéntico al previo a WP02. Ningún KeyboardEvent se emite, ningún sleep extra ocurre, y el payload se inyecta como bloque.

**Nombre del test**: `TestExtensionContent_HumanTyping_DisabledPreservesExistingBehavior`

**Setup**: El mismo harness que `TestExtensionContent_ProcessesGenerateCommand` existente, pero con `global.KeyboardEvent` como clase spy para verificar que NO se emite ningún evento.

```js
const keyEvents = [];
global.KeyboardEvent = class {
  constructor(type, init) {
    keyEvents.push({ type, key: init?.key });
  }
};

global.window = {
  requestAnimationFrame: (cb) => cb(),
  __AIBBE_SETTLE_MS: 0,
  // __AIBBE_HUMAN_TYPING NO definido (o definido como false)
};
```

**Assertions**:
```go
// 1. Ningún KeyboardEvent fue emitido
if len(result.KeyEvents) != 0 {
    t.Fatalf("expected no KeyboardEvents when __AIBBE_HUMAN_TYPING is false, got %v", result.KeyEvents)
}

// 2. El comportamiento de inserción bulk funciona (input value = payload)
if result.InputValue != "my payload" {
    t.Fatalf("inputValue = %q, want %q", result.InputValue, "my payload")
}

// 3. Button fue clickeado 1 vez
if result.ButtonClicks != 1 {
    t.Fatalf("buttonClicks = %d, want 1", result.ButtonClicks)
}

// 4. Response exitosa
if got := result.ContentResponses[0]["status"]; got != "success" {
    t.Fatalf("response status = %v, want success", got)
}
```

**Notas**:
- Este test protege contra regresiones: si alguien modifica `injectAndSubmit` y rompe el path no-humanizado, este test falla.
- Puede rehusar parte del harness del test existente `TestExtensionContent_ProcessesGenerateCommand`, pero con `global.KeyboardEvent` adicional como spy.

---

## Nota sobre `nodeResult` struct

WP03 requiere agregar campos al struct `nodeResult` en `extension_handshake_test.go`. Verificar qué campos ya existen y agregar solo los faltantes:

```go
type nodeResult struct {
    // Ya existentes:
    Logs             []string         `json:"logs"`
    Sent             []map[string]any `json:"sent"`
    ContentResponses []map[string]any `json:"contentResponses"`
    InputValue       string           `json:"inputValue"`
    InputEvents      int              `json:"inputEvents"`
    ButtonClicks     int              `json:"buttonClicks"`
    ObserverDisconnected bool         `json:"observerDisconnected"`
    ObserverConfig   map[string]any   `json:"observerConfig"`
    QuerySelectorCalls []string       `json:"querySelectorCalls"`
    ListenerReturnedTrue bool         `json:"listenerReturnedTrue"`
    SettleTimerPending bool           `json:"settleTimerPending"`

    // Agregar en WP01 (si no están):
    ExecCalls        []execCall       `json:"execCalls"`
    SetterCalled     bool             `json:"setterCalled"`
    TextContentAssigned string        `json:"textContentAssigned"`
    Events           []eventRecord    `json:"events"`

    // Agregar en WP03:
    InsertedChars    []string         `json:"insertedChars"`
    KeyEvents        []keyEvent       `json:"keyEvents"`
    SleepCalls       []int            `json:"sleepCalls"`
    EventLog         []string         `json:"eventLog"`
    SubmitClicks     int              `json:"submitClicks"`
}

type execCall struct {
    Cmd string `json:"cmd"`
    Val any    `json:"val"`
}

type eventRecord struct {
    Type   string `json:"type"`
    Bubbles bool  `json:"bubbles"`
}

type keyEvent struct {
    Type string `json:"type"`
    Key  string `json:"key"`
}
```

Si WP01 ya agregó `execCall` y `eventRecord`, WP03 solo agrega `keyEvent`, `InsertedChars`, `KeyEvents`, `SleepCalls`, `EventLog`, `SubmitClicks`.

---

## Definition of Done

- [ ] `TestExtensionContent_HumanTyping_InsertsCharsOneByOne` pasando.
- [ ] `TestExtensionContent_HumanTyping_DispatchesKeyboardEventsPerChar` pasando.
- [ ] `TestExtensionContent_HumanTyping_SleepsBeforeSubmit` pasando.
- [ ] `TestExtensionContent_HumanTyping_DisabledPreservesExistingBehavior` pasando.
- [ ] `go test ./...` pasa al 100% (incluyendo WP01 y todos los tests existentes).
- [ ] No se modificó `extension/content.js`.

## Reviewer Guidance

- Verificar T007: que `insertedChars` contiene chars individuales, no el string completo.
- Verificar T008: que el orden keydown→keypress→keyup se respeta por caracter. Un solo array desordenado no alcanza.
- Verificar T009: que el sleep ocurre ANTES del click (orden en eventLog), no después.
- Verificar T010: que sin `__AIBBE_HUMAN_TYPING`, zero KeyboardEvents son emitidos.
- Correr: `go test ./... -run TestExtensionContent_HumanTyping` para aislar los nuevos tests.

## Risks

- **Riesgo**: Los sleeps del loop de chars (uno por char) y el sleep pre-submit pueden interferir en el harness de T009 si no se distinguen correctamente. Mitigación: usar `__AIBBE_JITTER_RANGE:[0,0]` para los delays de chars (valor 0) y `__AIBBE_SUBMIT_DELAY_RANGE:[50,50]` para el pre-submit (valor 50) — así son distinguibles en el eventLog.
- **Riesgo**: El override de `global.setTimeout` en Node.js puede afectar los settle timers del `waitForAIResponse`. Mitigación: el override solo captura delays `> 0` o solo los que corresponden al rango de submit delay. Usar el mismo patrón que los tests existentes que ya override setTimeout (`__AIBBE_SETTLE_MS: 0` hace que settleMs = 0 y usen `setTimeout(cb, 0)`).
- **Riesgo**: El struct `nodeResult` en Go puede ya tener algunos campos definidos por WP01. El agente de WP03 debe verificar cuáles existen antes de agregar para no duplicar.
