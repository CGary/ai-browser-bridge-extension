# AI Browser Bridge Extension (aibbe)

## Propósito

Puente de automatización entre una CLI/daemon en Go y **NotebookLM** (Google) corriendo en un navegador Chromium. Permite a un agente externo enviar prompts, leer respuestas y ajustar selectores DOM en caliente, sin recargar la extensión.

La extensión está acoplada a NotebookLM (el content script sólo matchea `https://notebooklm.google.com/*`). El resto del stack (CLI, daemon, protocolo) es agnóstico de servicio.

## Arquitectura

```
┌─────────┐    Unix Socket    ┌────────────┐    Native Messaging    ┌──────────────────────┐
│  CLI    │ ──────────────► │  Daemon    │ ────────────────────► │  Chrome Extension    │
│ cmd/cli │   ipc.Request    │  daemon/   │   4-byte LE + JSON    │  background.js (SW)  │
│         │ ◄────────────── │            │ ◄──────────────────── │  content.js          │
└─────────┘   ipc.Response   └────────────┘                       │  → notebooklm.google │
                                                                   └──────────────────────┘
```

| Capa | Formato | Límite |
|---|---|---|
| CLI ↔ Daemon | JSON sobre Unix socket (`ipc.Request{Cmd, Target, Payload}`) | 1 MB |
| Daemon ↔ Extension | Native Messaging: prefijo uint32 LE + JSON | 1 MB |
| Extension → Tab | `chrome.tabs.sendMessage` al content script de la pestaña elegida | — |

Una solicitud a la vez, sincrónico. `fail-fast`: cualquier error aborta con exit code 1 (sin reintentos).

## Componentes

| Ruta | Descripción |
|---|---|
| `cmd/cli/main.go` | CLI efímera. Acepta `-cmd`, `-target`, `-payload` |
| `daemon/main.go` | Daemon residente. Unix socket → Native Messaging |
| `extension/manifest.json` | Manifest V3, ID estático, permisos `nativeMessaging`/`storage`/`tabs` |
| `extension/background.js` | Service Worker. Conecta al native host, rutea comandos a pestañas |
| `extension/content.js` | Inyectado en NotebookLM. Selectores DOM + calibración runtime |
| `internal/ipc/` | `Request`/`Response`, resolución de socket path |
| `internal/nativemessaging/` | Codec 4-byte LE + JSON |
| `configs/aibbe.nm-host.json` | Manifest de Native Messaging (Linux host) |
| `configs/aibbe.nm-host.docker.json` | Manifest de Native Messaging para el contenedor |
| `configs/docker/docker-compose.yml` | Stack Chromium + daemon en Docker |

## Routing a pestañas (HANDSHAKE)

Cada pestaña de NotebookLM, al cargar el content script, envía un `HANDSHAKE` al background con el nombre del notebook como `target`. El background mantiene un `tabRegistry` de pestañas libres.

- Sin flag `-target`: el daemon encuentra la primera pestaña libre.
- Con `-target "nombre del notebook"`: rutea a la pestaña cuyo título coincide exactamente.

Para que el CLI pueda rutear aún antes de que aparezca el título, el content script envía un handshake inicial con `target=null` y luego lo actualiza cuando el título renderiza.

## Comandos disponibles

| Cmd | Payload | Respuesta | Qué hace |
|---|---|---|---|
| `generate` | texto del prompt | `{status, result}` | Inyecta el prompt, submit, espera respuesta completa |
| `probe-selectors` | — | `{status, report}` | Reporta cuántos elementos matchea cada selector (diagnóstico) |
| `get-active-selectors` | — | `{status, selectors}` | Devuelve selectores vivos indicando si vienen de default o calibración |
| `calibrate` | JSON `{KEY: "selector", ...}` | `{status, applied}` | Sobrescribe selectores en `chrome.storage.local` y broadcast a todas las pestañas |
| `reset-selectors` | — | `{status}` | Borra todas las calibraciones, vuelve a defaults del código |

Claves válidas para `calibrate`: `INPUT`, `SUBMIT_BUTTON`, `RESPONSE_CONTAINER`, `RESPONSE_TEXT`, `THINKING_MARKERS`, `RESPONSE_READY_MARKERS`, `CITATION_NOISE`.

## Quickstart — Docker (recomendado)

Setup completo con Chromium aislado en contenedor: ver [`docs/quickstart-docker.md`](docs/quickstart-docker.md). Resumen:

```bash
# 1. Compilar binarios para linux/amd64 (el compose los monta desde bin/)
GOOS=linux GOARCH=amd64 go build -o bin/aibbe-daemon ./daemon/
GOOS=linux GOARCH=amd64 go build -o bin/aibbe-cli ./cmd/cli/

# 2. Credenciales VPN (ProtonVPN OpenVPN; vpn.env queda git-ignorado)
cp configs/docker/vpn.env.example configs/docker/vpn.env  # y completarlo

# 3. Levantar el stack (servicios: vpn + chrome)
docker compose -f configs/docker/docker-compose.yml up -d

# 4. Cargar la extensión en Chrome (http://localhost:9500) desde /config/extensions/aibbe

# 5. Usar el CLI dentro del contenedor (el socket vive dentro del container)
docker exec chrome aibbe-cli -cmd generate -payload "hola"
```

## Quickstart — Host local (sin Docker)

```bash
# Compilar
go build -o /tmp/aibbe-daemon ./daemon/
go build -o /tmp/aibbe-cli ./cmd/cli/

# Registrar native host (Chromium)
mkdir -p ~/.config/chromium/NativeMessagingHosts/
cp configs/aibbe.nm-host.json ~/.config/chromium/NativeMessagingHosts/aibbe.json
# editar el "path" del manifest para apuntar al binario

# Cargar la extensión
#   chrome://extensions → Modo desarrollador → Cargar descomprimida → extension/

# Levantar daemon
/tmp/aibbe-daemon

# Usar
/tmp/aibbe-cli -cmd generate -payload "qué es eslint"
```

## Flujo de calibración cuando NotebookLM cambia el DOM

```bash
# 1. Diagnosticar
aibbe-cli -cmd probe-selectors
#   → identifica qué selectores quedaron en "missing" o "multiple"

# 2. Inspeccionar DOM en DevTools, buscar una clase semántica estable
#    (ignorar ng-*, mat-mdc-*, cdk-*, _nghost-*, ng-tns-*)

# 3. Override runtime (sin recargar nada)
aibbe-cli -cmd calibrate -payload '{"SUBMIT_BUTTON": "button.nueva-clase"}'

# 4. Validar
aibbe-cli -cmd probe-selectors
aibbe-cli -cmd generate -payload "test"

# 5. Si andás bien, pasá el selector a los defaults en extension/content.js y:
aibbe-cli -cmd reset-selectors
```

## Variables de entorno

| Variable | Default | Descripción |
|---|---|---|
| `AIBBE_SOCKET_PATH` | `/tmp/aibbe.sock` | Ruta del socket Unix (CLI y daemon deben coincidir) |

## Extensión Chromium

- **Version**: 0.1.0
- **ID estático**: `bedlojjaiogmaefoadfpdecgajipcpgj` (fijado vía `key` en el manifest)
- **Permisos**: `nativeMessaging`, `storage`, `tabs`
- **Host matches**: `https://notebooklm.google.com/*`
- **Native host name**: `aibbe`

## Decisiones de diseño

- **Fail-fast**: sin reintentos ni fallbacks. Cualquier error protocolar aborta con exit 1.
- **Storage volátil para automatización**: sin persistencia a disco. Solo las calibraciones viven en `chrome.storage.local` (persistente por diseño).
- **Socket 0600**: creado con `umask 0o177`, sólo el dueño puede leer/escribir.
- **Validación de tamaño en dos capas**: IPC (1 MB primario) + Native Messaging (1 MB defensivo).
- **Selectores locale-agnósticos por default**: priorizar clases CSS semánticas de NotebookLM (`query-box-input`, `submit-button`, `to-user-message-inner-content`, `message-actions`) sobre `aria-label`, para que ande en `es`, `nl`, `en`, etc.

## Troubleshooting

| Síntoma | Causa probable | Fix |
|---|---|---|
| `generate` devuelve `response_timeout` | Un selector (probable `RESPONSE_READY_MARKERS` o `RESPONSE_CONTAINER`) no matchea | `probe-selectors` + `calibrate` |
| `generate` devuelve solo parte del texto | `RESPONSE_TEXT` apunta a un nodo demasiado estrecho | Inspeccionar y recalibrar |
| `no_free_tabs` | Ninguna pestaña de NotebookLM registrada en el handshake | Abrir/refrescar la pestaña; revisar consola del content script |
| `target_not_found` | El `-target` no coincide con el título de ningún notebook | Omitir `-target` o usar el nombre exacto |
| `native messaging host has not registered` | Manifest mal ubicado o path de binario incorrecto | Verificar `~/.config/chromium/NativeMessagingHosts/aibbe.json` |
| Socket connection refused | Daemon no corriendo | Levantar daemon |
| Permission denied en el socket | Dueño incorrecto (típico en Docker por UID/GID mismatch) | Ver `docs/quickstart-docker.md` |

## Desarrollo

```bash
# Tests
go test ./...
go test ./daemon/ -run TestCleanupSocket_FileExists

# Análisis estático
go vet ./...

# Syntax check de la extensión
node --check extension/content.js
node --check extension/background.js
```

## Documentación adicional

- [`docs/quickstart-docker.md`](docs/quickstart-docker.md) — setup Docker paso a paso
- [`docs/Software Design Document.md`](docs/Software%20Design%20Document.md) — decisiones arquitecturales
- [`docs/propuesta-calibracion-dinamica.md`](docs/propuesta-calibracion-dinamica.md) — diseño del sistema de calibración
- [`CLAUDE.md`](CLAUDE.md) — guía para agentes que trabajan en este repo
