---
title: Tarea 3 - Desarrollo del Cliente Efímero (CLI)
change_name: t3-cliente-efimero
---

## Summary

Implementación del contrato IPC compartido, handler de requests del daemon, CLI sin estado, y verificación con tests Go/build para el primer batch de T3.

**Detalles:**
- Contrato IPC compartido en `internal/ipc/ipc.go`
- Handler de requests en `daemon/main.go`, CLI stateless en `cmd/cli/main.go`
- Los tests de socket Unix en `cmd/cli` deben usar capability probe y hacer skip cuando `net.Listen("unix", ...)` falla con `setsockopt: operation not permitted`
- Todos los tests PASS