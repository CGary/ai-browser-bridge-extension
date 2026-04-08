---
title: Tarea 6 - Aclarar y validar el escenario de payload > 1 MiB
change_name: t6-oversize-validation-warning
---

## Summary

Refactorización del daemon para testabilidad, documentación de la jerarquía de guardrails, y tests para validación de payload oversize.

**Detalles:**

### Phase 1: Refactor for Testability
- Extraído `run(socketPath string, stop <-chan struct{}) error` desde `main()` en `daemon/main.go`
- Movido `cleanupSocket`, `listenSecure` y el accept loop a la función `run`
- Implementado goroutine que cierra el listener cuando el canal `stop` recibe señal

### Phase 2: Documentation
- Actualizado `docs/Software Design Document.md` para documentar la Guardrail Hierarchy: IPC (Primary) y Native Messaging (Secondary/Defensive)
- Aclarado que el check NM de 1 MiB es safety net para crecimiento interno del daemon, mientras IPC maneja límites externos

### Phase 3: Testing
- Añadido `TestNativeMessaging_OversizedPayload_Rejected` en `daemon/main_test.go` llamando `WriteMessage` con `1<<20 + 1` bytes
- Añadido `TestRun_StartsAndStops` para verificar daemon start/stop in-process sin binarios externos
- Cobertura aumentada a >70%

### Phase 4: Cleanup
- Removido logging temporal y código de debug
- Todos los tests PASS