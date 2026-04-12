---
title: Fortalecer validaciones de enrutamiento en T13
change_name: t16-fortalecimiento-validaciones
---
### B. Contexto y Objetivo
Reforzar la integridad del flujo transaccional implementado en `t13-orquestacion-enrutamiento-transaccional`. El objetivo es elevar la calidad del test de "Happy Path" mediante assertions explícitas sobre el comportamiento de comunicación entre el Background Script y los Content Scripts, y asegurar la correcta gestión del ciclo de vida del estado de la pestaña.

### C. Requisitos de Implementación (Especificaciones)
* Contratos/Interfaces:
  * Extensión de tests en `extension_handshake_test.go` o equivalente de integración.
  * Assertions de llamadas a `chrome.tabs.sendMessage`.
* Lógica de Negocio:
  * Implementar verificación de `sentTabMessages` para asegurar que el `tabId` y el `payload` enviados coinciden con la solicitud original.
  * Implementar verificación del ciclo de vida del estado: tras completar la transacción (o recibir respuesta), validar que el estado del tab registrado en `tabRegistry` retorne a `free`.
* Restricciones:
  * Mantener compatibilidad con la estructura actual de `tabRegistry`.

### D. Criterios de Aceptación (Acceptance Criteria)
* Escenario 1: En el test del "Happy Path", cuando se completa la transacción, el sistema verifica mediante assertions que el mensaje se envió con los parámetros correctos.
* Escenario 2: Tras finalizar la transacción exitosa, el estado del `tabId` en el registro debe ser inequívocamente `free`.

### E. Definición de Hecho (Definition of Done - DoD)
1. Assertions añadidas a los tests de integración existentes.
2. Verificación de retorno a estado `free` implementada y validada en test.
3. Pull Request validando estas nuevas assertions.
4. Ausencia de regresiones en tests existentes.
