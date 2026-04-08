---
title: Tarea 1 - Configuración Base y Rutina de Inicialización (Daemon)
change_name: t1-base-rutina
---

## Summary

Implementación completa de t1-base-rutina — go.mod, daemon/main.go, daemon/main_test.go. Todos los tests en verde.

**Detalles:**
- `go build ./...` fallaba con colisión de nombre cuando existe un directorio `daemon/` — se usó `go build -o /tmp/aibbe-daemon ./daemon/` como workaround
- Estrategia para TestCleanupSocket_RemoveError sin mocks: pasar un directorio no vacío como socketPath. os.Remove sobre dir no vacío retorna ENOTEMPTY (no es IsNotExist) → ejercita el error path limpiamente
- Cobertura total del paquete reporta 18.8% porque main() no se unit-testea. Cobertura de cleanupSocket es 100%
- Status: DONE — 11/11 tareas completadas, go vet limpio, 3/3 tests PASS