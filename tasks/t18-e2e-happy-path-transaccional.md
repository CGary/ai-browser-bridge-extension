---
title: Validar Happy Path Transaccional Completo CLI-Daemon-Stub
change_name: t18-e2e-happy-path-transaccional
---
### B. Contexto y Objetivo
El test E2E existente (`TestEndToEnd_CLI_To_Daemon_RoundTrip`) valida un eco directo sin transformacion semantica. El Hito 5 exige la "validacion transaccional completa desde la invocacion en CLI hasta el retorno del codigo fuente autogenerado". El objetivo es implementar un test E2E que utilice el stub NM programable (t17) configurado para simular el comportamiento realista de la extension: recibir un comando `generate`, procesar el payload, y retornar una respuesta tipificada `{status: "success", result: "<codigo>"}` que el daemon propaga hasta la CLI como salida en stdout con exit code 0.

### C. Requisitos de Implementación (Especificaciones)
* Contratos/Interfaces:
  * Test function: `TestEndToEnd_FullTransaction_HappyPath(t *testing.T)` en `daemon/main_test.go`.
  * Utiliza el stub NM de t17 con `WithSuccessResponse(...)`.
  * Invoca el binario CLI real contra el daemon real (mismo patron que `runCLIBinaryFromDaemonTests`).
* Logica de Negocio:
  * Iniciar daemon con stdin/stdout conectados al stub NM programable (no pipes manuales).
  * Invocar CLI con `-cmd generate -payload "context data"`.
  * El stub recibe el mensaje NM, verifica que contenga `{cmd: "generate", payload: "context data"}`, y responde con `{status: "success", result: "func main() { fmt.Println(\"hello\") }"}`.
  * Validar que:
    * CLI stdout contiene la respuesta JSON completa con el campo `result`.
    * CLI exit code es 0.
    * CLI stderr esta vacio o contiene solo logs informativos (no errores).
    * El stub registro exactamente 1 mensaje recibido con los campos correctos.
* Restricciones:
  * Reutilizar helpers existentes (`tempSocketPath`, `waitForDial`, `buildCLIBinary`, etc.).
  * No depender de un navegador real ni de la extension cargada.
  * Timeout de test: 10 segundos maximo.

### D. Criterios de Aceptación (Acceptance Criteria)
* Escenario 1: Dado un daemon con stub NM configurado para exito, cuando la CLI envia `-cmd generate -payload "context data"`, entonces stdout contiene `{status: "success", result: "..."}`, exit code es 0, y el stub registro el mensaje correcto.
* Escenario 2: Dado el mismo escenario, cuando se ejecutan dos transacciones secuenciales, entonces ambas completan exitosamente con sus respectivas respuestas, validando que el daemon no queda en estado inconsistente.
* Escenario 3: Dado el escenario de exito, el contenido de stdout es JSON valido parseable y contiene exclusivamente la respuesta de la extension sin metadatos del daemon.

### E. Definicion de Hecho (Definition of Done - DoD)
1. Codigo cumple con los estandares de estilo del proyecto.
2. Tests E2E implementados y aprobados con cobertura del happy path completo.
3. Documentacion tecnica actualizada en comentarios de codigo.
4. Pull Request revisado y aprobado por un par.
5. Ausencia de regresiones en el entorno de integracion.
