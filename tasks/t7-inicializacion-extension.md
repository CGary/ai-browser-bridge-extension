---
title: Inicializar Service Worker de Extensión Chromium
change_name: t7-inicializacion-extension
---
### B. Contexto y Objetivo
Desarrollar la estructura fundamental de la extensión de Chromium bajo el estándar Manifest V3. El objetivo es inicializar un Background Script (Service Worker) que posea los permisos necesarios para invocar la API nativa y establecer una conexión persistente con el Daemon, generando un ID de extensión estático indispensable para el registro posterior del Host.
### C. Requisitos de Implementación (Especificaciones)
* Contratos/Interfaces: Manifiesto V3 de Chromium (`manifest.json`).
* Lógica de Negocio:
  * Definir un Service Worker (`background.js`).
  * Declarar el permiso `nativeMessaging` en el manifiesto.
  * Implementar la llamada `chrome.runtime.connectNative('aibbe')` en la rutina de inicialización del Service Worker.
  * Implementar manejadores de eventos básicos (`onMessage`, `onDisconnect`) que emitan telemetría a la consola de la extensión para propósitos de depuración.
* Restricciones: Forzar un ID de extensión determinista (fijando la propiedad `key` en el `manifest.json` mediante una llave pública RSA) para evitar reconfiguraciones del entorno local en cada recarga de la extensión.
### D. Criterios de Aceptación (Acceptance Criteria)
* Escenario 1: Dado el entorno de Chromium en modo desarrollador, cuando el ingeniero carga el directorio de la extensión descomprimida, entonces la extensión se instala exitosamente, conserva un ID persistente y el Service Worker se registra sin errores de sintaxis o permisos.
### E. Definición de Hecho (Definition of Done - DoD)
1. Código cumple con los estándares de estilo del proyecto.
2. Pruebas unitarias e integración desarrolladas y aprobadas (cobertura mínima establecida).
3. Documentación técnica actualizada (Swagger, README o comentarios de código).
4. Pull Request revisado y aprobado por un par.
5. Ausencia de regresiones en el entorno de integración.