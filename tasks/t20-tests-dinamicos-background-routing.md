---
title: Implementar Tests Dinamicos de Enrutamiento en Background Script
change_name: t20-tests-dinamicos-background-routing
---
### B. Contexto y Objetivo
El test actual del Background Script (`TestExtensionBackground_RoutesNativeMessagesToTabs` en `extension_background_test.go`) realiza exclusivamente analisis estatico del codigo fuente mediante `strings.Contains`. Esto valida la presencia de patrones textuales pero no verifica el comportamiento dinamico del enrutamiento: que un mensaje NM entrante se transforme correctamente, se dirija al tab correcto, y que la respuesta del Content Script se retorne al NM port. El objetivo es reemplazar los tests estaticos con tests dinamicos que ejecuten `background.js` via Node.js (patron establecido en `extension_handshake_test.go`) y validen el ciclo completo de enrutamiento: NM port message → `findFreeTab()` → `chrome.tabs.sendMessage()` → response → `port.postMessage()`.

### C. Requisitos de Implementación (Especificaciones)
* Contratos/Interfaces:
  * Tests en `extension_handshake_test.go` (o nuevo archivo `extension_routing_test.go`) siguiendo el patron `runNodeJSON`.
  * Mock de Chrome APIs: `chrome.runtime.connectNative`, `chrome.runtime.onMessage`, `chrome.tabs.sendMessage`, `chrome.tabs.onRemoved`, `port.onMessage`, `port.postMessage`.
* Logica de Negocio:
  * **Test 1 — Routing exitoso a tab libre:**
    1. Simular HANDSHAKE de un tab (tabId=42, service="notebooklm").
    2. Emitir mensaje NM `{cmd: "generate", payload: "data"}` via `port.onMessage` listener.
    3. Verificar que `chrome.tabs.sendMessage` se invoco con tabId=42 y el payload correcto.
    4. Simular respuesta del Content Script: `{status: "success", result: "code"}`.
    5. Verificar que `port.postMessage` envio la respuesta intacta al NM port.
    6. Verificar que el estado del tab retorno a "free" en `tabRegistry`.
  * **Test 2 — Sin tabs libres:**
    1. No registrar ningun tab via HANDSHAKE.
    2. Emitir mensaje NM.
    3. Verificar que `port.postMessage` recibio `{status: "error", error: "no_free_tabs"}`.
  * **Test 3 — Tab ocupado (unico):**
    1. Registrar un tab y marcarlo como "busy" manualmente.
    2. Emitir mensaje NM.
    3. Verificar respuesta `no_free_tabs`.
  * **Test 4 — Error del Content Script:**
    1. Registrar tab libre.
    2. Configurar `chrome.tabs.sendMessage` para rechazar con error.
    3. Verificar que `port.postMessage` propaga el error.
    4. Verificar que el tab retorna a "free".
  * **Test 5 — Tab cerrado mid-transaccion:**
    1. Registrar tab libre.
    2. Emitir mensaje NM (tab se marca "busy").
    3. Emitir `chrome.tabs.onRemoved` para ese tabId.
    4. Verificar que el tab se purga del registry.
* Restricciones:
  * Mantener el patron de ejecucion Node.js via `runNodeJSON` ya establecido.
  * No introducir dependencias npm; mock de Chrome APIs inline en el script Node.
  * Preservar los tests estaticos existentes hasta que los dinamicos los reemplacen completamente.

### D. Criterios de Aceptación (Acceptance Criteria)
* Escenario 1: Dado un tab registrado como "free", cuando llega un mensaje NM, entonces el sistema lo enruta al tab correcto, recibe la respuesta, y la reenvía al NM port con el payload intacto.
* Escenario 2: Dado ningun tab registrado, cuando llega un mensaje NM, entonces el sistema responde `no_free_tabs` sin intentar enviar a ningun tab.
* Escenario 3: Dado un tab que retorna error, cuando el Background Script recibe la excepcion, entonces propaga el error al NM port y resetea el estado del tab a "free".
* Escenario 4: Dado un tab que se cierra durante una transaccion, cuando se emite `onRemoved`, entonces el tab se purga del registry.

### E. Definicion de Hecho (Definition of Done - DoD)
1. Codigo cumple con los estandares de estilo del proyecto.
2. Minimo 5 tests dinamicos implementados cubriendo los escenarios descritos.
3. Tests estaticos anteriores removidos o marcados como reemplazados.
4. Pull Request revisado y aprobado por un par.
5. Ausencia de regresiones en el entorno de integracion.
