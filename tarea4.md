### Tarea 4: Validación Síncrona de Entrada (User Story 1.2 - Escenario 1)
* Implementar en el Daemon la recepción de la carga útil JSON enviada por la CLI a través del socket.
* Codificar una capa de validación que verifique la integridad estructural del esquema JSON y garantice que el tamaño total del paquete es estrictamente menor a 1 MB.
* *Nota:* El manejo avanzado de errores por cargas inválidas (Escenario 2 de la US 1.2) queda explícitamente postergado para iteraciones posteriores según el límite de Trabajo en Progreso (WIP) establecido.