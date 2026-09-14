# Plan de corrección FIX-001 — `generate` devuelve `response_timeout` con la respuesta ya renderizada

> Documento ejecutable por `/q-orchestrate`. Contiene el brief-input resuelto (scope, aceptación,
> restricciones, tier, ID) para que el orquestador arranque sin entrevistar al humano.
> Estado: **planificado, no implementado**. Fecha: 2026-09-14.

## 1. Síntoma

```
$ aibbe -cmd generate -payload "consulta al RAG del SIAT"
{"status":"error","error":"response_timeout"}
```

En el navegador el prompt se inyecta, se envía y NotebookLM renderiza la respuesta completa. El
error llega igualmente al CLI tras ~150 s.

## 2. Diagnóstico (verificado contra el código, 2026-09-14)

### 2.1 Cadena de timeouts

| Capa | Valor | Dónde | Al expirar |
|---|---|---|---|
| `extension/content.js` (global) | 150 000 ms | `content.js:328,332` | `sendResponse({status:"error",error:"response_timeout"})` (`content.js:337`) |
| `extension/content.js` (settle) | 750 ms | `content.js:329,375` | `flushResponse` (`content.js:340`) |
| `extension/background.js` | ninguno | `background.js:136` | `await chrome.tabs.sendMessage`, sin timeout |
| `daemon/main.go` | ninguno | `daemon/main.go:193` | bloquea en `<-responseCh`, sin deadline |
| `cmd/cli/main.go` | ninguno | `cmd/cli/main.go:53` | `io.ReadAll(conn)` bloquea |

**Único productor** del string `response_timeout`: `content.js:337`. El daemon y el CLI solo lo
propagan intacto (`daemon/main_test.go:633` lo cubre). El bug vive en `content.js`.

### 2.2 Predicado de completitud (`content.js:346`)

```js
if (!state.hasThinkingMarkers && state.hasReadyMarkers
    && state.result.trim()
    && state.result === pendingSnapshot)
```

Mecánica actual (`content.js:356-376`):

- `MutationObserver` sobre `document.body` con `{childList, subtree, attributes, attributeFilter:["disabled"]}`. Sin `characterData`.
- En cada callback se lee el **último** `RESPONSE_CONTAINER` (no hay baseline de mensajes previos al submit) y se evalúa el estado.
- Estado "malo" (thinking presente, sin ready marker o texto vacío): se **limpia** el settle timer y no se re-arma (`content.js:362-368`).
- Estado "bueno": se captura `pendingSnapshot` y se **re-arma** `setTimeout(flushResponse, 750)` (`content.js:370-375`).
- `flushResponse` re-lee el DOM; si `state.result !== pendingSnapshot` **retorna sin hacer nada** (sin `else`, sin re-armar). El settle queda inerte hasta la próxima mutación.

Selectores por defecto (`content.js:5-13`):

| Clave | CSS |
|---|---|
| `RESPONSE_CONTAINER` | `mat-card-content.to-user-message-inner-content` |
| `RESPONSE_TEXT` | `.message-content` |
| `RESPONSE_READY_MARKERS` | `mat-card-actions.message-actions` |
| `THINKING_MARKERS` | `thinking-animation, .loading-spinner, [class*="thinking"]` |
| `CITATION_NOISE` | `button.citation-marker, .xap-inline-dialog` |

Ámbito de búsqueda de marcadores: `latestResponse.closest("chat-message") || parentElement || latestResponse` (`content.js:250`).

### 2.3 Hipótesis de causa raíz (ordenadas)

| # | Hipótesis | Evidencia | Estado |
|---|---|---|---|
| H1 | **Inanición del settle timer.** El observer es body-wide; cualquier mutación no relacionada (toggle `disabled`, overlays, telemetría, cursor) con el estado ya "bueno" re-arma los 750 ms indefinidamente. La respuesta está completa pero la ventana de silencio nunca llega. | `content.js:356-376` | Plausible confirmada por lectura de código |
| H2 | `RESPONSE_READY_MARKERS` (`mat-card-actions.message-actions`) ya no matchea el DOM actual de NotebookLM. | `content.js:10` | Plausible; requiere `probe-selectors` |
| H3 | `closest("chat-message")` falla (tag renombrado) y el fallback `parentElement` no contiene los ready markers. | `content.js:250` | Plausible; requiere `probe-selectors` + log de scope |
| H4 | `[class*="thinking"]` hace match por substring con un nodo residual → `hasThinkingMarkers` siempre `true`. | `content.js:11` | Plausible, especulativa |
| H5 | `message-actions` se renderiza en un overlay/portal CDK fuera del ancestro `chat-message` → nunca entra en `messageScope`. | CLAUDE.md señala ruido `cdk-*` | Plausible, no probada |
| H6 | Terminación del Service Worker MV3 a mitad del `await`. | `background.js:136` | Improbable: produciría error de transporte, no este payload limpio |

Defecto estructural independiente de cuál hipótesis sea la activa: **el predicado depende de la
cadencia de mutaciones en vez de sondear el DOM**, y un snapshot desactualizado en `flushResponse`
descarta el intento sin reprogramarlo.

## 3. Plan de corrección

### Fase 0 — Diagnóstico de campo (humano, 5 min, antes de implementar)

Ejecutar con una respuesta ya renderizada en la pestaña:

```bash
go run cmd/cli/main.go -cmd probe-selectors
```

Interpretación:

- `RESPONSE_READY_MARKERS = 0` → H2/H5 activas: incluir recalibración del default en el alcance.
- `THINKING_MARKERS > 0` con la respuesta terminada → H4 activa: estrechar el selector.
- Todos los conteos > 0 → H1 (starvation) es la causa; el fix principal basta.

Opcional (DevTools de la pestaña): `console.count` en `content.js:357` (observer) y `content.js:341` (flush). Si observer ≫ flush y flush = 0, H1 confirmada.

El resultado de esta fase se pega en la sección 6 (Decisiones de campo) antes de lanzar `/q-orchestrate`.

### Fase 1 — Fix principal: espera por sondeo, no por cadencia de mutaciones (`extension/content.js`)

1. Extraer la evaluación del estado a una función pura `evaluateResponseState(doc, selectors)` que devuelva `{hasThinkingMarkers, hasReadyMarkers, result, scopeTag}` sin efectos secundarios. Reutiliza la lectura ya existente en `content.js:240-260`.
2. Sustituir el patrón "re-armar settle en cada mutación" por un **poller de intervalo fijo** (`setInterval`, 500 ms) que evalúe el predicado directamente. La estabilidad se decide comparando el `result` de dos ticks consecutivos (snapshot previo === actual), no por silencio del DOM.
3. Mantener el `MutationObserver` solo como **acelerador** (ejecuta una evaluación inmediata) y restringir su raíz al contenedor de chat cuando exista (`latestResponse.closest("chat-message")?.parentElement || document.body`), sin depender de él para el cierre.
4. Capturar un **baseline** antes del submit: conteo de `RESPONSE_CONTAINER` existentes. El poller sólo acepta como respuesta un contenedor con índice ≥ baseline (evita leer la respuesta anterior).
5. En `flushResponse`, si el snapshot difiere: actualizar el snapshot y continuar (nunca retornar silenciosamente). Si hay `RESPONSE_READY_MARKERS` y el texto lleva N ticks estable, resolver aunque el observer siga disparando.
6. Conservar el timeout global de 150 s como techo (fail-fast, sin reintentos). Limpiar interval, observer y timeout en todos los caminos de salida.
7. Enriquecer el payload de error: `{status:"error", error:"response_timeout", detail:{hasThinkingMarkers, hasReadyMarkers, resultLength, scopeTag, observerFires, pollTicks}}`. El daemon y el CLI ya propagan JSON íntegro; no requiere cambio de protocolo.

### Fase 2 — Selectores (condicional a Fase 0)

- Si `RESPONSE_READY_MARKERS = 0`: actualizar el default en `content.js:10` con el selector semántico vigente (preferir `.message-actions` sin prefijo de tag). Ignorar clases `ng-*`, `mat-mdc-*`, `cdk-*`.
- Si `THINKING_MARKERS` da falsos positivos: eliminar `[class*="thinking"]` o acotarlo al ámbito del último mensaje.
- Documentar el cambio en la tabla de troubleshooting de `README.md:156` y `es-README.md:156`.

### Fase 3 — Tests

Seguir el patrón de `extension_routing_test.go` (Go ejecuta el JS de la extensión bajo `node` con un harness de DOM/`chrome` falso, sin navegador). Nuevo archivo `extension_generate_wait_test.go`:

| Test | Escenario | Esperado |
|---|---|---|
| `TestGenerateWait_ResolvesUnderContinuousMutations` | Respuesta completa + mutaciones no relacionadas cada 100 ms durante > 2 s | `status:"ok"` con el texto, antes del timeout global |
| `TestGenerateWait_IgnoresPreviousResponse` | Existe un contenedor previo con ready markers; el nuevo tarda en aparecer | No devuelve el texto viejo; espera al índice ≥ baseline |
| `TestGenerateWait_TimeoutCarriesDiagnostics` | Ready marker nunca aparece | `error:"response_timeout"` con `detail.hasReadyMarkers=false` |
| `TestGenerateWait_ThinkingMarkerBlocksResolution` | Thinking marker persistente | No resuelve; timeout con `detail.hasThinkingMarkers=true` |

Los tests existentes (`daemon/main_test.go:633` propagación del error) deben seguir en verde.

## 4. Brief-input resuelto para `/q-orchestrate`

```yaml
task_id: FIX-001
title: generate devuelve response_timeout aunque NotebookLM ya renderizó la respuesta
difficulty_tier: logic-on-skeleton   # default; escalar blueprint a arquitecto (riesgo: cambia el núcleo de espera del content script)
risk: medium

scope:
  touch:
    - extension/content.js            # waitForAIResponse / flushResponse / evaluateResponseState / baseline
    - extension_generate_wait_test.go # nuevo, patrón de extension_routing_test.go
    - README.md                       # fila troubleshooting L156 (solo si cambia un selector default)
    - es-README.md                    # ídem
  forbid:
    - daemon/**                       # el daemon solo propaga; no añadir deadlines ni reintentos
    - cmd/cli/**                      # semántica síncrona intacta
    - internal/**                     # protocolo native messaging / IPC intactos
    - extension/background.js         # routing y tabRegistry intactos (salvo que Fase 0 demuestre H6)
    - extension/manifest.json
    - configs/**                      # incluye docker; el contenedor vpn NUNCA se toca
  out_of_scope:
    - persistencia de datos de automatización
    - reintentos automáticos de generate
    - soporte a otros servicios distintos de NotebookLM

acceptance:
  automated:
    - node --check extension/content.js
    - node --check extension/background.js
    - go vet ./...
    - go test ./...                   # incluye los 4 tests nuevos de extension_generate_wait_test.go
  manual:
    - go run cmd/cli/main.go -cmd generate -payload "<consulta al RAG SIAT>" devuelve status ok con el texto de la respuesta
    - el mismo comando con una pestaña donde el ready marker no existe devuelve response_timeout con detail diagnóstico en < 150 s
    - go run cmd/cli/main.go -cmd probe-selectors reporta conteos > 0 para RESPONSE_CONTAINER, RESPONSE_TEXT y RESPONSE_READY_MARKERS

constraints:
  - Fail-fast: sin reintentos; timeout global 150 s se mantiene como techo.
  - Selectores locale-agnostic: clases semánticas de NotebookLM, nunca aria-label ni ng-*/mat-mdc-*/cdk-*.
  - Las 7 claves de calibrate (INPUT, SUBMIT_BUTTON, RESPONSE_CONTAINER, RESPONSE_TEXT, THINKING_MARKERS, RESPONSE_READY_MARKERS, CITATION_NOISE) conservan nombre y semántica; chrome.storage.local sigue teniendo prioridad sobre defaults.
  - El shape de respuesta {status, result|error} no cambia; detail es un campo adicional opcional.
  - Manifest V3: no usar APIs que requieran keepalive del SW; la espera vive en content.js.
  - Limpiar interval/observer/timeout en todo camino de salida (sin fugas entre requests; una pestaña ocupada debe volver a estar libre).
  - Contenedor vpn de configs/docker: prohibido tocar.

verification_commands:
  fast:
    - node --check extension/content.js
    - go test . -run TestGenerateWait
  full:
    - go vet ./... && go test ./...
```

Invocación sugerida:

```
/q-orchestrate FIX-001 — usar docs/plan-correccion-FIX-001-response-timeout.md como brief resuelto; no entrevistar; tier logic-on-skeleton con blueprint escalado a arquitecto.
```

## 5. Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| Fase 0 revela que el DOM cambió (H2/H5) y el poller solo no basta | Fase 2 es parte del scope; el blueprint decide el selector con el `probe-selectors` pegado en §6 |
| Resolver antes de tiempo (texto estable durante streaming lento) | Exigir ready marker + 2 ticks estables (1 s); el ready marker de NotebookLM aparece al final |
| Leer la respuesta anterior | Baseline de contenedores antes del submit (Fase 1.4) + test dedicado |
| Fugas de timers dejan la pestaña `busy` | Cleanup único en `finally`; test de timeout verifica que no quedan timers activos |
| Recalibración manual previa en `chrome.storage.local` enmascara el fix | Antes del e2e manual: `go run cmd/cli/main.go -cmd reset-selectors` |

## 6. Decisiones de campo (Fase 0 ejecutada 2026-09-14)

Ejecutado vía `docker exec chrome /usr/local/bin/aibbe-cli -cmd probe-selectors` (el daemon corre
dentro del contenedor `chrome`; no hay socket en el host). Pestaña con 10 respuestas ya renderizadas.

```
probe-selectors (2026-09-14, contenedor chrome):
  INPUT                   = 1   (unique)
  SUBMIT_BUTTON           = 1   (unique)
  RESPONSE_CONTAINER      = 10
  RESPONSE_TEXT           = 20
  RESPONSE_READY_MARKERS  = 10   -> H2 y H5 DESCARTADAS
  THINKING_MARKERS        = 24   -> H4 ACTIVA (con todo renderizado)
  CITATION_NOISE          = 181
Hipótesis activa: H4 (causa raíz), H1 (defecto estructural secundario)
Fase 2 requerida: SÍ (obligatoria, es el fix mínimo)
```

Desglose de `THINKING_MARKERS` (calibrate aislado por sub-selector + `reset-selectors` al final):

| Sub-selector | Matches |
|---|---|
| `thinking-animation` | 0 |
| `.loading-spinner` | 0 |
| `[class*="thinking"]` | **24** |

Los 24 falsos positivos provienen íntegramente del match por substring de atributo. Con ~2.4 nodos
por mensaje, el ámbito del último mensaje (`messageScope`, `content.js:250`) contiene al menos uno,
por lo que `hasThinkingMarkers` es permanentemente `true` y el predicado de `content.js:346` nunca
puede cumplirse: el timeout de 150 s está garantizado aunque la respuesta esté completa. El flujo no
llega siquiera a armar el settle timer, así que H1 no es la causa del síntoma reportado — pero sigue
siendo un defecto estructural real y permanece en el alcance de la Fase 1.

Estado de `chrome.storage.local` tras el diagnóstico: `reset-selectors` ejecutado, sin overrides.

## 7. Trazabilidad del análisis

- Análisis externo (rung 0, `opencode_go` / `qwen3.7-plus`): cadena de timeouts, predicado, 5 hipótesis. Registrado en el ledger de fleet-delegate (2026-09-14).
- Verificación interna (sonnet/high): corrigió el comportamiento del settle timer (abort en estado malo, re-arm solo en bueno; `flushResponse` sin `else`), añadió H5 y las opciones exactas del observer.
