---
title: Gestión de Ciclo de Vida y Purga de Tabs
change_name: t12-gestion-ciclo-vida-tabs
---
### B. Contexto y Objetivo
Garantizar la integridad del inventario de pestañas en el Background Script. El objetivo es purgar automáticamente las entradas del registro cuando el usuario cierra una pestaña o el navegador la finaliza, evitando intentos de enrutamiento hacia contextos inexistentes que provocarían fallos en la comunicación.

### C. Requisitos de Implementación (Especificaciones)
* Contratos/Interfaces: `chrome.tabs.onRemoved` (Evento nativo de Chromium).
* Lógica de Negocio:
  * Implementar un listener en `background.js` para `chrome.tabs.onRemoved`.
  * Localizar el `tabId` que ha sido eliminado en el `Map` del inventario interno.
  * Eliminar la entrada correspondiente si existe, liberando la memoria y asegurando que futuras consultas al inventario solo retornen pestañas activas.
* Restricciones: 
  * Se asume que cualquier pestaña rastreada que se cierre debe ser purgada inmediatamente de forma reactiva.

### D. Criterios de Aceptación (Acceptance Criteria)
* Escenario 1: Dada una pestaña registrada en el inventario del Background Script, cuando el usuario cierra dicha pestaña en el navegador, entonces el registro para ese `tabId` es eliminado del `Map` interno.

### E. Definición de Hecho (Definition of Done - DoD)
1. Código cumple con los estándares de estilo del proyecto.
2. Pruebas manuales demuestran que el cierre de pestañas libera las entradas en el registro (monitoreo vía consola del Background Script).
3. Pull Request revisado y aprobado por un par.
4. Ausencia de regresiones en la estabilidad del Background Script.
