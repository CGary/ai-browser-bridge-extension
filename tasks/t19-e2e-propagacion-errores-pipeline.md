---
title: Validar Propagacion de Errores a traves del Pipeline E2E Completo
change_name: t19-e2e-propagacion-errores-pipeline
---
### B. Contexto y Objetivo
Los errores tipificados de la extension (`no_free_tabs`, `response_timeout`, `input_not_found`) se testean actualmente solo a nivel de componente individual (Content Script via Node.js, Background Script via analisis estatico). No existe validacion de que estos errores fluyan correctamente desde su punto de origen en la extension simulada, a traves del protocolo NM y el daemon, hasta la CLI con el exit code y la salida en stderr apropiados. El objetivo es implementar tests E2E que validen la propagacion integra de cada tipo de error a lo largo de todo el pipeline.

### C. Requisitos de Implementación (Especificaciones)
* Contratos/Interfaces:
  * Tests table-driven: `TestEndToEnd_ErrorPropagation(t *testing.T)` en `daemon/main_test.go`.
  * Reutiliza el stub NM de t17 con `WithErrorResponse(errorCode)`.
  * Cada caso de la tabla configura un error diferente y valida su propagacion.
* Logica de Negocio:
  * Casos de error a validar:
    1. `no_free_tabs`: Stub responde `{status: "error", error: "no_free_tabs"}`. CLI debe recibir esta respuesta intacta en stdout.
    2. `response_timeout`: Stub responde `{status: "error", error: "response_timeout"}`. CLI debe propagar la respuesta sin truncar.
    3. `input_not_found`: Stub responde `{status: "error", error: "input_not_found"}`. CLI propaga el JSON intacto.
    4. Error generico de extension: Stub responde `{status: "error", error: "unexpected_extension_error"}`. CLI propaga sin crash.
  * Para cada caso validar:
    * La respuesta JSON del stub llega intacta al proceso CLI en stdout (pass-through puro).
    * El campo `status` es "error" y el campo `error` contiene el codigo esperado.
    * La CLI sale con exit code 0 (el MVP utiliza pass-through; interpretacion de errores es Future Work).
    * No hay panic, deadlock, ni proceso zombie.
    * El contenido de stderr contiene solo logs informativos del daemon, no la respuesta JSON.
* Restricciones:
  * Timeout de test por caso: 5 segundos.
  * Reutilizar la infraestructura de t17 y t18.
  * La CLI implementa pass-through puro del payload de respuesta (exit code es siempre 0 en esta tarea). Diferenciacion de exit codes por tipo de error es Future Work post-MVP.

### D. Criterios de Aceptación (Acceptance Criteria)
* Escenario 1: Dado un stub que responde `no_free_tabs`, cuando la CLI envia un comando, entonces recibe el JSON de error intacto y el proceso termina sin bloqueo.
* Escenario 2: Dado un stub que responde `response_timeout`, cuando la CLI envia un comando, entonces la respuesta contiene el codigo de error y la transaccion completa en menos de 5 segundos.
* Escenario 3: Dado un stub que responde `input_not_found`, cuando la CLI envia un comando, entonces la respuesta propaga el error sin truncar campos.
* Escenario 4: Dado un stub que responde con un codigo de error desconocido, cuando la CLI envia un comando, entonces el daemon no entra en panico y la CLI recibe la respuesta tal cual.

### E. Definicion de Hecho (Definition of Done - DoD)
1. Codigo cumple con los estandares de estilo del proyecto.
2. Tests table-driven para los 4 escenarios de error implementados y aprobados.
3. Documentacion tecnica actualizada en comentarios de codigo.
4. Pull Request revisado y aprobado por un par.
5. Ausencia de regresiones en el entorno de integracion.
