---
title: Implementar Handshake y Registro de Tabs
change_name: t11-handshake-registro-tabs
---
### B. Contexto y Objetivo
Establecer el mecanismo de descubrimiento de interfaces web activas. El objetivo es que el Content Script, al ser inyectado, notifique su presencia al Background Script, permitiendo que este último mantenga un inventario actualizado de pestañas disponibles para el procesamiento de requerimientos.

### C. Requisitos de Implementación (Especificaciones)
* Contratos/Interfaces: 
  * Mensaje Handshake: `{ type: "HANDSHAKE", service: "notebooklm" }`.
  * Registro Interno (Background): `Map<tabId, { state: "free" | "busy", service: "notebooklm", lastSeen: number }>`.
* Lógica de Negocio:
  * Actualizar `manifest.json` para incluir un Content Script (`extension/content.js`) que se inyecte exclusivamente en `https://notebooklm.google.com/*`.
  * Crear `extension/content.js` que emita el mensaje `HANDSHAKE` con el servicio "notebooklm" al cargar la página.
  * Modificar `background.js` para escuchar mensajes (`chrome.runtime.onMessage`) y registrar el `tabId` del remitente con estado inicial `free`.
* Restricciones: 
  * El registro debe ser puramente volátil (en memoria).
  * No se debe persistir información en `chrome.storage` para este hito.

### D. Criterios de Aceptación (Acceptance Criteria)
* Escenario 1: Dada la extensión cargada, cuando el usuario abre `https://chatgpt.com/`, entonces el Content Script se ejecuta y el Background Script registra la pestaña en su inventario interno con estado `free`.

### E. Definición de Hecho (Definition of Done - DoD)
1. Código cumple con los estándares de estilo del proyecto.
2. Pruebas de integración manual (consola de extensión) verifican el registro del tabId.
3. Documentación técnica actualizada (README o comentarios en background.js).
4. Pull Request revisado y aprobado por un par.
5. Ausencia de regresiones en la conexión de Native Messaging previa.
