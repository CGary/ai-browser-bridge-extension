---
title: Validar Contrato de Salida CLI (stdout/stderr/exit codes)
change_name: t21-validacion-contrato-salida-cli
---
### B. Contexto y Objetivo
Las especificaciones de UI/UX definen un contrato estricto de salida para la CLI: `stdout` exclusivo para la carga util resultante en texto plano puro sin caracteres ANSI, `stderr` exclusivo para telemetria de errores, y codigos de salida estandarizados (0 para exito, >0 para fallo). Los tests existentes verifican fragmentos de este contrato de forma indirecta. El objetivo es implementar una suite de tests dedicada que valide sistematicamente la conformidad del contrato de salida en todos los escenarios del pipeline: exito transaccional, errores de validacion, errores de extension, y desincronizacion de protocolo.

### C. Requisitos de Implementación (Especificaciones)
* Contratos/Interfaces:
  * Tests en `cmd/cli/main_test.go` o `daemon/main_test.go` segun el escenario.
  * Validacion cruzada contra el SDD seccion 4 (Contratos de Datos) y las Especificaciones de UI/UX seccion 5 (POSIX).
* Logica de Negocio:
  * **Suite 1 — stdout en exito:**
    1. Transaccion exitosa: stdout contiene exclusivamente el JSON de respuesta. No contiene logs del daemon, prefijos de timestamp, ni caracteres ANSI.
    2. El JSON de stdout es parseable y contiene los campos del contrato (`status`, `result`).
    3. stdout es compatible con operador pipe: `aibbe ... | jq .result` debe funcionar.
  * **Suite 2 — stderr en exito:**
    1. En transaccion exitosa, stderr puede contener logs informativos pero NO errores de nivel ERROR o FATAL.
    2. Los logs en stderr siguen el formato `[NIVEL] [COMPONENTE] Descripcion`.
  * **Suite 3 — exit codes:**
    1. Exito transaccional → exit code 0.
    2. Error de validacion (payload invalido) → exit code 1.
    3. Error de extension (no_free_tabs, timeout) → exit code 0 (CLI es pass-through puro; la respuesta JSON contiene el status de error).
    4. Error de conexion (daemon no levantado) → exit code 1.
  * **Suite 4 — Ausencia de contaminacion:**
    1. stdout no contiene secuencias ANSI (`\033[`).
    2. stdout contiene exclusivamente la respuesta JSON del daemon (pass-through puro). Ningun log, prefijo, ni metadata adicional.
    3. Ningun log del daemon, incluso logs informativos, se filtra a stdout. Todo log debe ir a stderr.
* Restricciones:
  * Utilizar el stub NM de t17 para escenarios que requieran el pipeline completo.
  * Para escenarios de CLI pura (conexion fallida, flags invalidos), testear contra un daemon ausente o mock.
  * Preservar compatibilidad con el patron `buildCLIBinary` existente.

### D. Criterios de Aceptación (Acceptance Criteria)
* Escenario 1: Dado una transaccion exitosa, cuando se captura stdout, entonces contiene exclusivamente JSON parseable sin caracteres de control ni logs del daemon.
* Escenario 2: Dado un error de extension, cuando se captura stderr, entonces contiene el mensaje de error estructurado y el exit code es distinto de 0.
* Escenario 3: Dado cualquier escenario (exito o fallo), cuando se inspecciona stdout byte a byte, entonces no contiene secuencias de escape ANSI.
* Escenario 4: Dado un daemon no levantado, cuando la CLI intenta conectar, entonces termina con exit code 1 y un mensaje en stderr indicando el fallo de conexion.

### E. Definicion de Hecho (Definition of Done - DoD)
1. Codigo cumple con los estandares de estilo del proyecto.
2. Suite de tests con minimo 8 casos cubriendo las 4 suites descritas.
3. Documentacion tecnica actualizada en comentarios de codigo.
4. Pull Request revisado y aprobado por un par.
5. Ausencia de regresiones en el entorno de integracion.
