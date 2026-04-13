---
title: Extraer y Notificar Destino en Content Script
change_name: t23-extraccion-destino-content-script
---
### B. Contexto y Objetivo
Para que el Background Script pueda enrutar mensajes a la biblioteca correcta, debe conocer qué biblioteca está abierta en cada pestaña. El objetivo es que el Content Script (`extension/content.js`) analice el DOM de la Single Page Application (SPA) de NotebookLM, extraiga el identificador o título de la biblioteca actual y lo envíe en el mensaje de inicialización (`HANDSHAKE`). Además, debe detectar navegaciones internas de la SPA y actualizar el estado.

### C. Requisitos de Implementación (Especificaciones)
* Contratos/Interfaces:
  * El mensaje de handshake enviado desde el content script al background script debe incluir la nueva propiedad: `{ type: "HANDSHAKE", service: "notebooklm", target: "<NombreDeLaBiblioteca>" }`.
* Lógica de Negocio:
  * Identificación del selector: Encontrar el selector del DOM fiable que contiene el título de la biblioteca activa en NotebookLM.
  * Sincronización SPA (Inicial): Como Angular/React pueden tardar en renderizar el título, implementar un `MutationObserver` (o mecanismo de espera basado en eventos/intervalos acotados) al inicializar el script para aguardar a que el selector esté presente antes de enviar el primer `HANDSHAKE`.
  * Sincronización SPA (Navegación): Mantener un `MutationObserver` activo sobre el título o contenedor relevante. Si el usuario cambia de biblioteca sin recargar la pestaña, extraer el nuevo nombre y enviar un nuevo mensaje `HANDSHAKE` con el `target` actualizado.
* Restricciones:
  * Garantizar la limpieza de los observers si el contexto se invalida.
  * No bloquear la inicialización de otros listeners de mensajes mientras se espera el target.

### D. Criterios de Aceptación (Acceptance Criteria)
* Escenario 1: Dado que el usuario abre una biblioteca en NotebookLM, cuando el DOM termina de renderizar el título, entonces el Content Script envía un mensaje `HANDSHAKE` con la propiedad `target` poblada.
* Escenario 2: Dado que el usuario está en una biblioteca "Lib A", cuando navega mediante la UI de la SPA a "Lib B" (sin recargar la página completa), entonces el Content Script detecta el cambio en el DOM y envía un nuevo mensaje `HANDSHAKE` con `target: "Lib B"`.

### E. Definición de Hecho (Definition of Done - DoD)
1. Código cumple con los estándares de estilo del proyecto.
2. Pruebas unitarias e integración desarrolladas y aprobadas (cobertura mínima establecida).
3. Documentación técnica actualizada (Swagger, README o comentarios de código).
4. Pull Request revisado y aprobado por un par.
5. Ausencia de regresiones en el entorno de integración.
