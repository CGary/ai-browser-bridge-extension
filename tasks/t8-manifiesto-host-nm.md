---
title: Configurar Manifiesto del Host Native Messaging
change_name: t8-manifiesto-host-nm
---
### B. Contexto y Objetivo
Vincular el entorno de ejecución de Chromium con el binario del Daemon local. Requiere la creación del archivo de configuración JSON que el navegador utiliza para ubicar el ejecutable y autorizar la comunicación exclusiva con la extensión desarrollada en la tarea anterior.
### C. Requisitos de Implementación (Especificaciones)
* Contratos/Interfaces: Estructura de Native Messaging Host Manifest de Chromium.
* Lógica de Negocio:
  * Crear el archivo `aibbe.json`.
  * Definir las propiedades requeridas: `name` ("aibbe"), `description`, `path` (ruta absoluta al binario precompilado del Daemon), y `type` ("stdio").
  * Configurar la propiedad `allowed_origins` utilizando el ID estático de la extensión (`chrome-extension://[ID_ESTATICO]/`).
* Restricciones: El archivo debe ubicarse y probarse en la ruta estándar para entornos Linux/Debian (`~/.config/chromium/NativeMessagingHosts/aibbe.json`).
### D. Criterios de Aceptación (Acceptance Criteria)
* Escenario 1: Dado que el manifiesto está en el directorio correcto y el binario en su ruta correspondiente, cuando el Service Worker ejecuta `connectNative('aibbe')`, entonces Chromium lanza el proceso del Daemon local exitosamente sin emitir errores de host no encontrado o acceso denegado en la consola de inspección.
### E. Definición de Hecho (Definition of Done - DoD)
1. Código cumple con los estándares de estilo del proyecto.
2. Pruebas unitarias e integración desarrolladas y aprobadas (cobertura mínima establecida).
3. Documentación técnica actualizada (Swagger, README o comentarios de código).
4. Pull Request revisado y aprobado por un par.
5. Ausencia de regresiones en el entorno de integración.