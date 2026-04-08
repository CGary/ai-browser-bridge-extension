---
title: Integrar Pipeline Bidireccional de Native Messaging
change_name: t10-tuberia-bidireccional
---
### B. Contexto y Objetivo
Verificar la integridad del puente de comunicación consolidando las implementaciones previas. El objetivo es transmitir un requerimiento desde la herramienta de línea de comandos (CLI) hasta la extensión de Chromium, y asegurar que la extensión retorne el mensaje al Daemon para su impresión final en la consola del usuario.
### C. Requisitos de Implementación (Especificaciones)
* Contratos/Interfaces: Socket Unix (CLI -> Daemon), `stdout` (Daemon -> Extensión), `stdin` (Extensión -> Daemon), Terminal (`stdout` final).
* Lógica de Negocio:
  * Modificar temporalmente el Background Script (`background.js`) para que funcione como un servicio de eco (Echo Service): al interceptar un mensaje en `port.onMessage`, debe retransmitir inmediatamente la carga útil de vuelta al puerto usando `port.postMessage`.
  * Conectar el bucle de lectura de `stdin` del Daemon (Tarea t9) a la interfaz de salida, imprimiendo el resultado JSON retornado por la extensión hacia la salida estándar original de la terminal.
* Restricciones: La salida del proceso en la terminal debe ser estrictamente el texto plano JSON devuelto por Chromium. No se permiten caracteres adicionales, prefijos o metadatos en `stdout` (estos deben ir exclusivamente a `stderr`).
### D. Criterios de Aceptación (Acceptance Criteria)
* Escenario 1: Dada la arquitectura en ejecución, cuando el ingeniero ejecuta `aibbe --cmd="ping"`, entonces el mensaje viaja por el socket, ingresa a Chromium, la extensión hace eco del mismo, el Daemon lo procesa por `stdin` y la terminal imprime el JSON resultante, finalizando la ejecución con código de salida `0`.
### E. Definición de Hecho (Definition of Done - DoD)
1. Código cumple con los estándares de estilo del proyecto.
2. Pruebas unitarias e integración desarrolladas y aprobadas (cobertura mínima establecida).
3. Documentación técnica actualizada (Swagger, README o comentarios de código).
4. Pull Request revisado y aprobado por un par.
5. Ausencia de regresiones en el entorno de integración.