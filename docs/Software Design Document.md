# Software Design Document (SDD): aibbe

## 1. Visión General de la Arquitectura
El sistema implementa una arquitectura híbrida diseñada para orquestar interacciones automatizadas con interfaces web de inteligencia artificial. Se fundamenta en un modelo de comunicación de paso de mensajes asíncrono gestionado por un daemon residente, el cual expone una interfaz síncrona y transaccional hacia una herramienta de línea de comandos (CLI). El puente principal de comunicación aprovecha la API Native Messaging de Chromium, aislando la lógica de interacción del Document Object Model (DOM) dentro de una extensión de navegador dedicada y manteniendo el entorno local agnóstico a los cambios estructurales en las plataformas de terceros.

## 2. Diagrama de Componentes (Descripción lógica)
* **CLI (Cliente Efímero):** Proceso sin estado que captura la entrada del ingeniero, transmite el requerimiento a través de un socket Unix y bloquea el hilo de ejecución hasta recibir una respuesta definitiva (código fuente autogenerado o excepción).
* **Daemon (Host de Native Messaging):** Binario residente desarrollado en Go. Mantiene el estado de la conexión persistente con el navegador a través de los flujos estándar (`stdin`/`stdout`). Opera como un servidor IPC (Inter-Process Communication) sobre un socket Unix (ej. `/tmp/aibbe.sock`), traduciendo las peticiones síncronas de la CLI en eventos asíncronos para la extensión.
* **API Native Messaging de Chromium:** Interfaz estándar del navegador que facilita y securiza el intercambio de cargas útiles entre la extensión y el binario anfitrión.
* **Extensión de Chromium:**
    * *Background Script / Service Worker:* Escucha los eventos provenientes del daemon, gestiona el enrutamiento bidireccional de los mensajes hacia pestañas de NotebookLM y controla el ciclo de vida de la inyección de scripts.
    * *Content Script:* Componente inyectado dinámicamente en `notebooklm.google.com`. Ejecuta el mapeo de selectores, la inserción de contexto (RAG automatizado) y la extracción del código fuente validado.

## 3. Modelo de Datos y Almacenamiento
La arquitectura opera bajo un paradigma de almacenamiento estrictamente volátil. El ciclo de vida de los datos, incluyendo el contexto técnico inyectado, las plantillas de prompts y el código resultante, reside de manera exclusiva en la memoria operativa (RAM) durante la ejecución transaccional del comando. Se omite de forma deliberada la persistencia en el sistema de archivos local y la implementación de APIs de almacenamiento del navegador (ej. `chrome.storage.local`) para garantizar la latencia mínima y satisfacer el alcance del MVP.

## 4. Diseño de Interfaces (APIs)
* **Contrato de Datos (Daemon - Extensión):** Utiliza un esquema de tipado dinámico basado en texto (JSON). La comunicación cumple estrictamente con las especificaciones del protocolo Native Messaging: cada transmisión está precedida por un entero de 32 bits (4 bytes) en orden de bytes nativo que indica la longitud del mensaje, seguido del objeto JSON codificado en UTF-8.
* **Contrato de Datos (CLI - Daemon):** La transferencia de instrucciones a través del socket Unix emplea el mismo esquema JSON (ej. `{"cmd": "...", "target": "...", "payload": "..."}`), asegurando la interoperabilidad de las estructuras de datos y facilitando la depuración directa del flujo interno sin la sobrecarga de formatos de serialización binaria en esta iteración. El campo `target` permite el enrutamiento dirigido a contextos específicos.

## 5. Infraestructura y Despliegue
El modelo de distribución se basa en la transferencia de código fuente, descartando la implementación de pipelines de integración continua (CI/CD) o empaquetado de binarios.
* **Compilación Local:** El código del daemon requiere compilación explícita mediante las cadenas de herramientas de Go directamente en el entorno de desarrollo anfitrión (optimizado y probado para entornos Debian 12).
* **Registro de Native Messaging:** Exige la creación y configuración manual del archivo de manifiesto JSON en los directorios de configuración del usuario (ej. `~/.config/chromium/NativeMessagingHosts/aibbe.json`), estableciendo la política de orígenes permitidos y la ruta absoluta al ejecutable compilado.
* **Carga de Extensión (Sideloading):** Requiere la instalación manual de la extensión no empaquetada a través de la interfaz de gestión de Chromium en modo de desarrollador, evadiendo los tiempos de validación de la Chrome Web Store.

## 6. Consideraciones de Seguridad
* **Manejo de Excepciones (Patrón Fail-Fast):** El sistema rechaza la implementación de políticas de reintentos automáticos ante mutaciones en el DOM. Cualquier fallo en la aserción de selectores o error de inyección aborta la secuencia inmediatamente. La excepción se propaga de manera síncrona hacia la CLI, terminando el proceso con un código de salida distinto de cero y requiriendo la intervención técnica para el ajuste del código de la extensión.
* **Aislamiento IPC:** Los permisos del socket Unix local deben restringirse al usuario propietario del sistema que ejecuta la sesión del navegador (`chmod 0600`). Esta medida es crítica para mitigar vectores de escalada de privilegios o la ejecución de comandos arbitrarios por parte de otros procesos o usuarios concurrentes en la misma máquina.
* **Jerarquía de Guardarraíles de Tamaño:** La validación de tamaño opera en dos capas. La capa **IPC** es el guardarraíl primario y rechaza cualquier solicitud de la CLI que exceda 1 MiB antes de que llegue a la lógica del daemon. La capa **Native Messaging** es un guardarraíl secundario y defensivo: vuelve a validar el tamaño justo antes de escribir hacia Chromium para cubrir futuros casos donde el daemon construya mensajes internos más grandes que la carga original recibida por IPC.
* **Chequeo Defensivo en Native Messaging:** Aunque en el flujo actual el daemon reenvía el payload IPC sin expandirlo, el límite de 1 MiB en Native Messaging se conserva como red de seguridad del protocolo. Esto documenta explícitamente que IPC maneja los límites de entrada externos, mientras que Native Messaging protege la salida del host frente a crecimiento interno futuro.

---

## 7. Estado de Implementación (post-MVP)

> **Nota**: Este documento representa el diseño base del MVP. Las siguientes capacidades fueron implementadas después del diseño original y están documentadas en los archive reports de Engram. Para deltas de implementación, consultar los reportes individuales de cada cambio (t7–t16).

### Componentes implementados (resumen)

| Componente | Archivo | Estado | Change |
|---|---|---|---|
| IPC types/constants | `internal/ipc/ipc.go` | ✅ | t1–t3 |
| Native Messaging writer/reader | `internal/nativemessaging/nativemessaging.go` | ✅ | t4, t9 |
| CLI client | `cmd/cli/main.go` | ✅ | t3 |
| Go Daemon (full pipeline) | `daemon/main.go` | ✅ | t1–t6, t9–t10 |
| Extension manifest (MV3) | `extension/manifest.json` | ✅ | t7 |
| NM host manifest | `configs/aibbe.nm-host.json` | ✅ | t8 |
| Background Script (SW) | `extension/background.js` | ✅ | t7, t10–t12 |
| Content Script | `extension/content.js` | ✅ | t11, t14–t15 |

### Extensiones al diseño original

El código actual incluye capacidades que **no estaban en este SDD original**:

1. **`tabRegistry` (volatile Map)** — `background.js` mantiene un `Map<tabId, {state, service, lastSeen}>` descubierto vía HANDSHAKE del Content Script. Cada tab registrado transiciona `libre` → `ocupado` → `libre` por transacción. *(t11, t12, t13)*
2. **`chrome.tabs.onRemoved` listener** — Purga reactiva de tabs cerrados del registry. *(t12)*
3. **`findFreeTab()` routing** — El Background Script localiza el primer tab `libre`, lo marca `busy`, envía el payload vía `chrome.tabs.sendMessage`, y resetea el estado en `finally`. *(t10/t13)*
4. **DOM Injection (`injectAndSubmit`)** — El Content Script localiza el input, inyecta el payload vía `execCommand("insertText")` (trusted event) o fallback de native setter, y hace click en el submit button. *(t14)*
5. **MutationObserver extraction (`waitForAIResponse`)** — Observa `document.body` con settle timer (750ms), timeout configurable (`window.__AIBBE_TIMEOUT ?? 150000`), extracción de bloques `<code>/<pre>`, y limpieza de ruido de citaciones. *(t15)*
6. **Error response pipeline** — Los errores (timeout, input not found, etc.) se propagan como objetos JSON `{ status: "error", error: "<code>" }` por el pipeline completo de respuesta, no solo via stderr. *(t15, t16)*

Para detalles de implementación por cambio, consultar los archive reports en Engram bajo `sdd/{change-name}/archive-report`.
les de implementación por cambio, consultar los archive reports en Engram bajo `sdd/{change-name}/archive-report`.
