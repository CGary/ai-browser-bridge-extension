---
title: Orquestar Enrutamiento Transaccional y Mock de Respuesta
change_name: t13-orquestacion-enrutamiento-transaccional
---
### B. Contexto y Objetivo
Consolidar la lógica del "Tab Orchestrator" mediante el enrutamiento determinista de peticiones Native Messaging. El objetivo es que las peticiones que llegan del Daemon sean redirigidas a una pestaña libre, marcada como ocupada durante la transacción, y finalmente respondidas de vuelta al Daemon liberando el estado de la pestaña.

### C. Requisitos de Implementación (Especificaciones)
* Contratos/Interfaces: 
  * `chrome.tabs.sendMessage` (Background -> Content).
  * Objeto Requerimiento: `{ cmd: "generate", payload: string }`.
  * Objeto Respuesta Mock: `{ status: "success", result: "mocked code source" }`.
* Lógica de Negocio:
  * Modificar el listener de Native Messaging en `background.js` para buscar el primer `tabId` con estado `free` en el inventario.
  * Cambiar el estado del `tabId` seleccionado a `busy`.
  * Enviar el requerimiento al Content Script de esa pestaña mediante `chrome.tabs.sendMessage`.
  * En el Content Script (`content.js`), responder al mensaje con un objeto JSON mockeado de éxito.
  * En el Background Script, capturar la respuesta del Content Script, retransmitirla al puerto de Native Messaging (`port.postMessage`) y resetear el estado del `tabId` a `free`.
* Restricciones: 
  * Ante la ausencia de pestañas libres, retornar un error estructurado al Daemon inmediatamente.
  * No implementar tiempos de espera (timeout) en esta fase (se delega a M4).

### D. Criterios de Aceptación (Acceptance Criteria)
* Escenario 1: Dada la arquitectura activa, cuando el ingeniero ejecuta `aibbe request.json`, entonces el mensaje llega al Background Script, se enruta a una pestaña libre, esta responde con el mock y el Daemon imprime el resultado final en la terminal.
* Escenario 2: Sin pestañas registradas, cuando se invoca la CLI, el Background Script detecta la ausencia de destino y el Daemon reporta un error estructurado en `stderr`.

### E. Definición de Hecho (Definition of Done - DoD)
1. Código cumple con los estándares de estilo del proyecto.
2. Pruebas de integración manual demuestran el flujo completo desde CLI hasta la respuesta mock del Content Script.
3. El estado de la pestaña transiciona correctamente de `free` a `busy` y de vuelta a `free`.
4. Pull Request revisado y aprobado por un par.
5. Ausencia de regresiones en el entorno de integración.
