---
title: Tarea 5 - Formateo y Transmisión hacia Native Messaging
change_name: t5-formateo-transmision
---

## Summary

Verificación de implementación existente de formato wire en `internal/nativemessaging/nativemessaging.go` y routing stdout/stderr en `daemon/main.go`.

**Detalles:**
- Se corrigieron los artefactos spec/design/tasks para alinearse con el límite compartido de 1MB IPC/Native Messaging (el diseño viejo asumía un límite IPC menor que NM)
- Se fortaleció `TestHandleConnection_OversizedRequest_Rejected` para verificar que no se emiten bytes Native Messaging y que se escribe un log de oversize
- No se rediseñó la pipeline del daemon — se alinearon los artefactos al código existente
- Todos los tests PASS