---
title: Extraer Respuesta Generada vía Mutación de DOM
change_name: t15-observador-mutacion-extraccion
---
### B. Contexto y Objetivo
Implementar un observador de DOM en el Content Script capaz de reaccionar asíncronamente a los cambios estructurales producidos durante la fase de generación de texto de la IA proveedora, extrayendo el bloque de código resultante al momento de completarse para retornarlo al Daemon a través de la tubería bidireccional.

### C. Requisitos de Implementación (Especificaciones)
* Contratos/Interfaces: 
  * Interfaz de mensajes salientes: `chrome.runtime.sendMessage` hacia `background.js` o uso del callback de la respuesta asíncrona enviando el objeto `{ status: "success", result: string }`.
  * API nativa del navegador: `MutationObserver`.
* Lógica de Negocio:
  * Inicializar una instancia de `MutationObserver` asociada al contenedor padre de las respuestas o hilo del chat justo después de forzar la inyección (dependencia de t14).
  * Monitorear activamente las mutaciones del subárbol evaluando la aparición de nuevos nodos de texto o elementos de bloque correspondientes a la finalización de la respuesta de IA (ej. bloques de `<code>`, `pre` o habilitación nuevamente del botón *submit*).
  * Tras detectar que el texto se generó completamente, extraer la cadena de texto ignorando la semántica visual del DOM.
  * Emitir la respuesta extraída al Background Script y purgar la instancia del `MutationObserver` mediante el método `disconnect()`.
* Restricciones: 
  * Se descarta la implementación de tiempos de espera estrictos (Timeout). La lógica se enfoca exclusivamente en la extracción de la respuesta exitosa ("Happy Path" de la US 3.2). El bloqueo infinito se mitigará desde el exterior según M5 en posteriores iteraciones.
  * La dependencia del selector es estricta al proveedor, pero se deben encapsular constantes.

### D. Criterios de Aceptación (Acceptance Criteria)
* Escenario 1: Generación exitosa de código (Happy Path). Dado que el sistema emitió el requerimiento en el DOM y el observador está activo, cuando el proveedor de IA renderiza el bloque esperado al completarse su inferencia, entonces el Content Script captura correctamente el texto resultante, desconecta el observador y emite la carga útil hacia el Background Script, finalizando su bloque de ejecución asíncrono.

### E. Definición de Hecho (Definition of Done - DoD)
1. Código cumple con los estándares de estilo del proyecto.
2. Pruebas unitarias e integración desarrolladas y aprobadas (cobertura mínima establecida).
3. Documentación técnica actualizada (Swagger, README o comentarios de código).
4. Pull Request revisado y aprobado por un par.
5. Ausencia de regresiones en el entorno de integración.