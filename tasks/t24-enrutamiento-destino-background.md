---
title: Implementar Enrutamiento por Destino en Background Script
change_name: t24-enrutamiento-destino-background
---
### B. Contexto y Objetivo
El último paso de la tubería requiere que el Service Worker de la extensión (Background Script) dirija el mensaje entrante del daemon hacia el tab correcto, basándose en la biblioteca solicitada. El objetivo es modificar la gestión del ciclo de vida de los tabs en el `tabRegistry` para persistir el parámetro `target` y actualizar la lógica de enrutamiento para realizar "Target-based Library Routing".

### C. Requisitos de Implementación (Especificaciones)
* Contratos/Interfaces:
  * Estructura del tabRegistry: Cada entrada almacenada en la memoria para un tab debe añadir el campo `target`: `{ state: "free", service: "notebooklm", target: "<NombreDeLaBiblioteca>" }`.
  * Contrato de error: Retornar estructurado al daemon si el destino no existe: `{ status: "error", error: "target_not_found", message: "No se encontró un tab con el target solicitado" }`.
* Lógica de Negocio:
  * `tabRegistry` Update: Al recibir un mensaje `HANDSHAKE` desde el Content Script, actualizar la entrada correspondiente en el mapa `tabRegistry` asignando el valor de `target` provisto.
  * Lógica de Enrutamiento (`findFreeTab` o equivalente): 
    * Si el mensaje entrante del daemon incluye un parámetro `target` válido y no vacío, iterar sobre el `tabRegistry` buscando el primer tab cuyo estado sea `"free"` y su `target` coincida de manera exacta (o case-insensitive si se prefiere).
    * Si el mensaje del daemon **no** incluye `target` (comportamiento legacy), devolver el primer tab con estado `"free"`.
  * Manejo de Ausencias: Si se solicita un `target` específico y no hay ningún tab "free" que coincida (ya sea porque no está abierto o porque está ocupado), rechazar la solicitud inmediatamente y enviar el mensaje de error de vuelta por Native Messaging al daemon.
* Restricciones:
  * Mantener las operaciones de mapa eficientes (O(n) sobre el mapa en memoria está bien dado el bajo volumen de tabs).
  * Tratar los cierres de tabs o recargas (`tabs.onRemoved`, `tabs.onUpdated`) manteniendo la coherencia del nuevo campo `target`.

### D. Criterios de Aceptación (Acceptance Criteria)
* Escenario 1: Dado un registro con tabs {id: 1, target: "Lib A"} y {id: 2, target: "Lib B"} (ambos libres), cuando llega un request con `target: "Lib B"`, entonces el mensaje se enruta exclusivamente al tab 2.
* Escenario 2: Dado un registro con varios tabs libres, cuando llega un request sin la propiedad `target`, entonces el mensaje se enruta al primer tab libre disponible (comportamiento legacy).
* Escenario 3: Dado un registro donde no hay tabs con el target "Lib C", cuando llega un request con `target: "Lib C"`, entonces el request no se envía a ningún tab y se retorna un mensaje de error `{ status: "error", error: "target_not_found" }` al daemon.
* Escenario 4: Dado que un Content Script reenvía un `HANDSHAKE` con un nuevo target por navegación SPA, cuando se inspecciona el `tabRegistry`, entonces el target del tabId correspondiente se ha actualizado exitosamente.

### E. Definición de Hecho (Definition of Done - DoD)
1. Código cumple con los estándares de estilo del proyecto.
2. Pruebas unitarias e integración desarrolladas y aprobadas (cobertura mínima establecida).
3. Documentación técnica actualizada (Swagger, README o comentarios de código).
4. Pull Request revisado y aprobado por un par.
5. Ausencia de regresiones en el entorno de integración.
