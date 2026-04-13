---
title: Ampliar Contrato IPC y CLI con Parámetro de Destino
change_name: t22-ampliacion-contrato-ipc-target
---
### B. Contexto y Objetivo
Se requiere la capacidad de dirigir comandos a una biblioteca específica de NotebookLM. Para ello, el primer paso es actualizar la capa base de comunicación introduciendo el parámetro de destino. El objetivo es agregar el campo `Target` al contrato de mensajes IPC y exponerlo como un flag opcional en la interfaz de línea de comandos (CLI), permitiendo que el dato fluya hasta el daemon y posteriormente hacia la extensión.

### C. Requisitos de Implementación (Especificaciones)
* Contratos/Interfaces:
  * Modificar `internal/ipc/ipc.go`: Agregar el campo `Target string `json:"target,omitempty"` al struct `Request`.
* Lógica de Negocio:
  * Modificar `cmd/cli/main.go`: Agregar un nuevo flag opcional `-target` (tipo string).
  * Incluir el valor del flag `-target` al construir la instancia de `ipc.Request`.
  * Modificar `daemon/main.go` (opcional/log): Actualizar el log de recepción `[INFO] [Daemon] received...` para que, si el `Request` deserializado tiene un `Target` no vacío, lo imprima en el log para observabilidad.
* Restricciones:
  * No alterar la lógica de transmisión del daemon. El payload de Native Messaging ya reenvía los bytes crudos del JSON recibido, por lo que el nuevo campo `target` viajará transparentemente.
  * Mantener retrocompatibilidad: si `-target` no se provee, el campo no debe serializarse o debe estar vacío (garantizado por `omitempty`).

### D. Criterios de Aceptación (Acceptance Criteria)
* Escenario 1: Dado el CLI compilado, cuando el usuario ejecuta `./cli -cmd "ask" -target "SIAT" -payload "hola"`, entonces el JSON enviado por el socket Unix incluye la propiedad `"target":"SIAT"`.
* Escenario 2: Dado el CLI, cuando el usuario ejecuta `./cli -cmd "ask" -payload "hola"` (sin target), entonces el JSON enviado no contiene la clave `"target"` o su valor es vacío.
* Escenario 3: Dado el daemon en ejecución, cuando recibe un payload con target, entonces emite un log informativo que incluye el valor del target recibido.

### E. Definición de Hecho (Definition of Done - DoD)
1. Código cumple con los estándares de estilo del proyecto.
2. Pruebas unitarias e integración desarrolladas y aprobadas (cobertura mínima establecida).
3. Documentación técnica actualizada (Swagger, README o comentarios de código).
4. Pull Request revisado y aprobado por un par.
5. Ausencia de regresiones en el entorno de integración.
