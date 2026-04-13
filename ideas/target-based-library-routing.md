# Idea: Target-based Library Routing

## Contexto Actual

El parámetro `-cmd` del CLI actualmente funciona como **identificador de acción** (`generate`, `echo`, `ping`). El daemon valida que no esté vacío y lo reenvía tal cual a la extensión Chrome. El Content Script ejecuta la misma lógica sin importar el valor del comando.

## Problema

El usuario tiene **múltiples bibliotecas de NotebookLM abiertas simultáneamente** (ej: "SIAT", "PF", "React"). Al ejecutar una consulta desde el CLI, no hay forma de dirigir la pregunta a la biblioteca específica. El routing actual (`findFreeTab`) simplemente toma el primer tab libre sin distinguir qué biblioteca tiene abierta cada tab.

## Propuesta

### Nuevo parámetro: `-target`

Agregar un tercer parámetro al CLI que identifique **la biblioteca destino** de la consulta.

```bash
-cmd "generate" -target "SIAT" -payload "¿qué es el SIAT?"
-cmd "generate" -target "PF" -payload "¿qué es la programación funcional?"
```

### Semántica de los parámetros

| Flag | Rol | Ejemplo |
|------|-----|---------|
| `-cmd` | **Qué hacer** (acción del Content Script) | `generate`, `ask`, `echo` |
| `-target` | **Dónde hacerlo** (biblioteca destino) | `SIAT`, `PF`, `React` |
| `-payload` | **Contenido** (la pregunta o prompt) | `¿qué es el SIAT?` |

### Por qué `-target` y no parseo del payload

Usar un delimitador como `:` en el payload (ej: `SIAT: ¿qué es?`) es **frágil**:
- El payload puede contener `:` naturalmente en la pregunta
- Requiere split, escape y reconstrucción innecesaria
- Complejidad en el daemon que no le corresponde

`-target` es **explícito, robusto y extensible** (permite futuros parámetros como `-flags`, `-version`, etc.).

## Impacto Arquitectónico

### 1. IPC Contract (`internal/ipc/ipc.go`)

Agregar campo `Target` al `Request`:

```go
type Request struct {
    Cmd     string `json:"cmd"`
    Target  string `json:"target,omitempty"`
    Payload string `json:"payload"`
}
```

### 2. CLI (`cmd/cli/main.go`)

Agregar flag `-target` (opcional):

```go
target := flag.String("target", "", "target library name (optional, defaults to first free tab)")
```

### 3. Daemon (`daemon/main.go`)

**Sin cambios de lógica requeridos.** El daemon ya reenvía los bytes crudos del JSON (línea 179: `nativemessaging.WriteMessage(nativeOut, data)`). Si `Target` está en el `Request` serializado desde el CLI, se reenvía automáticamente. El campo pasa transparentemente sin intervención del daemon.

**Mejora menor (opcional)**: Actualizar la línea de log `fmt.Fprintf(os.Stderr, "[INFO] [Daemon] received: cmd=%s payload=%s\n", ...)` para incluir también `target=%s` cuando esté presente.

### 4. Background Script (`extension/background.js`)

#### Cambio en el handshake
El Content Script al inicializar debe **leer el identificador de la biblioteca** desde el DOM de NotebookLM (título del tab, selector de la biblioteca activa, etc.) y enviarlo en el handshake:

```js
// De: { type: "HANDSHAKE", tabId: 123, service: "notebooklm" }
// A:  { type: "HANDSHAKE", tabId: 123, service: "notebooklm", target: "SIAT" }
```

⚠️ **Nota crítica sobre timing**: NotebookLM es una SPA. El Content Script carga sincrónicamente antes de que Angular renderice el título de la biblioteca. Usar `MutationObserver` o `requestAnimationFrame` para detectar cuándo el selector está disponible en el DOM, luego enviar el HANDSHAKE. Si se envía antes, `target` será `null/undefined`.

#### Cambio en la tabRegistry
Almacenar `target` por cada tab registrado:

```js
tabRegistry.set(tabId, { state: "free", service: "notebooklm", target: "SIAT" });
```

#### Cambio en el routing (`findFreeTab`)
En lugar de tomar el primer tab libre, buscar el que coincida con `target`:

```js
// Antes: primer tab libre
// Ahora: primer tab libre cuyo target === request.target
function findTargetedTab(target) {
  for (const [id, entry] of tabRegistry) {
    if (entry.state === "free" && entry.target === target) return id;
  }
  return null; // no match
}
```

## Riesgos y Mitigación

| Riesgo | Mitigación |
|--------|-----------|
| **[CRÍTICO] HANDSHAKE timing en SPA** | El Content Script se ejecuta antes de que Angular renderice el selector del título. `MutationObserver` + `requestAnimationFrame` para esperar el DOM, luego enviar HANDSHAKE con `target` correcto. |
| **[CRÍTICO] Target stale en navegación SPA** | El usuario navega entre bibliotecas dentro del mismo tab sin recargar. El `target` en `tabRegistry` se vuelve obsoleto. El Content Script necesita un `MutationObserver` en el selector de título para re-enviar HANDSHAKE cuando cambia la biblioteca. |
| ¿Cómo identificar el target en el DOM? | NotebookLM muestra el título de la biblioteca en el header/sidebar — identificar el selector exacto mediante inspección manual |
| Dos tabs con el mismo target | Tomar el primero libre (comportamiento actual) |
| `-target` especificado pero ninguna biblioteca abierta | Retornar error estructurado: `{ status: "error", error: "target_not_found" }` |
| `-target` vacío o no especificado | Fallback al comportamiento actual: primer tab libre |

## Estado

**Propuesta** — pendiente de implementación.
