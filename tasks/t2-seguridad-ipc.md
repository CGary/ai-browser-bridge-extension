---
title: Tarea 2 - Implementación de Seguridad IPC (User Story 1.1)
change_name: t2-seguridad-ipc
---

## Summary

Implementación de creación segura de socket Unix vía `listenSecure` usando `syscall.Umask(0o177)`, logging de inicio del daemon actualizado, y tests Go para modo de socket 0600 y UID del propietario.

**Detalles:**
- Usar umask alrededor de `net.Listen("unix", ...)` mantiene el socket seguro desde su creación
- La prueba de subprocesos existente fue suficiente para verificar la propiedad UID sin nuevas dependencias
- 12/12 tareas completadas, go vet limpio, todos los tests PASS