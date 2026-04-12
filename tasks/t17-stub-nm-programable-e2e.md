---
title: Construir Stub de Extensión Programable para Tests E2E del Daemon
change_name: t17-stub-nm-programable-e2e
---
### B. Contexto y Objetivo
El test E2E existente (`TestEndToEnd_CLI_To_Daemon_RoundTrip`) valida únicamente un eco directo del mensaje Native Messaging: el daemon reenvía el payload de la CLI y el mismo payload se inyecta manualmente en stdin como respuesta. Este modelo no verifica la transformación semántica que la extensión real ejecuta (enrutamiento a tab, inyección DOM, extracción de resultado tipificado). El objetivo es construir un **stub de Native Messaging programable** en Go que simule el comportamiento del Background Script + Content Script a nivel de protocolo NM, permitiendo configurar respuestas deterministas para cada escenario de test (éxito, timeout, no_free_tabs, input_not_found).

### C. Requisitos de Implementación (Especificaciones)
* Contratos/Interfaces:
  * El stub debe implementar la interfaz de lectura/escritura del protocolo NM (prefijo de 4 bytes LE + JSON UTF-8) sobre `io.Reader`/`io.Writer`.
  * API pública del stub: `NewNMStub(config StubConfig) (*NMStub, io.ReadCloser, io.WriteCloser)` que devuelve el stub y los extremos de pipe para conectar al daemon.
  * `StubConfig` debe soportar: `ResponseFunc func(request json.RawMessage) (response json.RawMessage, delay time.Duration)` para respuestas dinámicas.
  * Presets de fábrica: `WithEchoResponse()`, `WithSuccessResponse(result string)`, `WithErrorResponse(errorCode string)`, `WithDelayedResponse(response json.RawMessage, delay time.Duration)`.
* Lógica de Negocio:
  * El stub lee mensajes del pipe NM stdin (lo que el daemon escribe a stdout), decodifica el JSON, aplica `ResponseFunc`, y escribe la respuesta al pipe NM stdout (lo que el daemon lee de stdin).
  * Soportar múltiples mensajes secuenciales (para tests de procesamiento en serie).
  * Exponer un log interno de mensajes recibidos (`ReceivedMessages() []json.RawMessage`) para assertions en tests.
* Restricciones:
  * Ubicar en `daemon/testutil_test.go` o `daemon/nmstub_test.go` (solo compilación de test).
  * Respetar el límite de 1 MB del protocolo NM.
  * No introducir dependencias externas.
  * Implementar cierre limpio de pipes (`io.Closer`) en la interfaz del stub para evitar fugas de file descriptors durante ejecuciones intensivas de tests.

### D. Criterios de Aceptación (Acceptance Criteria)
* Escenario 1: Dado un stub configurado con `WithSuccessResponse("print('hello')")`, cuando el daemon le envía un mensaje NM `{cmd: "generate", payload: "data"}`, entonces el stub responde `{status: "success", result: "print('hello')"}` usando el protocolo NM de 4 bytes + JSON.
* Escenario 2: Dado un stub configurado con `WithErrorResponse("no_free_tabs")`, cuando el daemon le envía cualquier mensaje NM, entonces el stub responde `{status: "error", error: "no_free_tabs"}` respetando el formato wire.
* Escenario 3: Dado un stub que ha procesado N mensajes, cuando se consulta `ReceivedMessages()`, entonces retorna exactamente los N mensajes recibidos en orden.
* Escenario 4: Dado un stub configurado con `WithDelayedResponse(response, 50ms)`, cuando el daemon le envía un mensaje, entonces la respuesta se emite tras el delay configurado.

### E. Definicion de Hecho (Definition of Done - DoD)
1. Codigo cumple con los estandares de estilo del proyecto.
2. Pruebas unitarias del propio stub (validacion de presets, log de mensajes, protocolo wire).
3. Documentacion tecnica del API del stub en comentarios de codigo.
4. Pull Request revisado y aprobado por un par.
5. Ausencia de regresiones en el entorno de integracion.
