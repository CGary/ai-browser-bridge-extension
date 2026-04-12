---
title: Implementar Receptor de Flujo Estándar (stdin)
change_name: t9-receptor-stdin-daemon
---
### B. Contexto y Objetivo
Dotar al Daemon de la capacidad para leer mensajes entrantes desde el navegador. Esta tarea implementa la lectura binaria sobre `stdin` acatando el protocolo de Chromium y da cumplimiento estricto a la User Story 1.3 (Manejo de desincronización) mediante el patrón Fail-Fast.
### C. Requisitos de Implementación (Especificaciones)
* Contratos/Interfaces: Flujo de entrada estándar (`os.Stdin`). Lógica de lectura bloqueante.
* Lógica de Negocio:
  * Iniciar un bucle de lectura continua sobre `stdin`.
  * Leer exactamente 4 bytes en orden Little-Endian para determinar la longitud de la carga útil (`N`).
  * Asignar un búfer del tamaño `N` y leer la carga útil JSON.
  * Validar escenarios de error: fin de archivo (EOF) o fallos de lectura.
* Restricciones: Implementación obligatoria de la User Story 1.3. Si la lectura de los primeros 4 bytes o del búfer subsiguiente falla o resulta en datos truncados, el sistema debe emitir un log detallado hacia `stderr` y ejecutar `os.Exit(1)` inmediatamente, finalizando el proceso sin intentos de recuperación.
### D. Criterios de Aceptación (Acceptance Criteria)
* Escenario 1: Dado un flujo de datos válido en `stdin`, cuando el Daemon lee el prefijo, entonces extrae correctamente el tamaño y decodifica el JSON adyacente sin errores.
* Escenario 2 (US 1.3): Dado que el Daemon se encuentra en lectura activa, cuando se inyecta un byte incompleto o se interrumpe el flujo abruptamente en `stdin`, entonces el Daemon imprime `[FATAL] [Daemon] Desincronización de protocolo Native Messaging` en `stderr` y finaliza su ejecución con un código de estado distinto de cero.
### E. Definición de Hecho (Definition of Done - DoD)
1. Código cumple con los estándares de estilo del proyecto.
2. Pruebas unitarias e integración desarrolladas y aprobadas (cobertura mínima establecida).
3. Documentación técnica actualizada (Swagger, README o comentarios de código).
4. Pull Request revisado y aprobado por un par.
5. Ausencia de regresiones en el entorno de integración.

---

## Status: ✅ Implemented & Archived

**Archived**: 2026-04-07 | **Engram Archive**: #313
**SDD Cycle**: explore (#292) → propose (#294) → spec (#298) → design (#302) → tasks (#303) → apply (#306) → verify (#310) → archive (#313)

**Files Modified**:
- `internal/nativemessaging/nativemessaging.go` — Added `ReadMessage(io.Reader) ([]byte, error)`
- `internal/nativemessaging/nativemessaging_test.go` — Added table-driven tests
- `daemon/main.go` — Added `stdinLoop` goroutine before accept loop
- `daemon/main_test.go` — Added unit + integration tests

**Verification Evidence**:
- 16/16 tasks complete
- Build, vet, tests passed
- Runtime verification outside sandbox passed (3/3 critical scenarios)
- Coverage: 92.9% nativemessaging, 15.9% daemon