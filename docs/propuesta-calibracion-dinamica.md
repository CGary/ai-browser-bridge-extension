# Calibración dinámica de selectores — estado y propuesta pendiente

## Estado: Fase 1 IMPLEMENTADA ✅

El sistema de calibración dinámica propuesto originalmente en este documento ya está en
producción. Referencia rápida de lo implementado:

- **Persistencia de overrides**: clave `aibbe_calibrations` en `chrome.storage.local`
  (`extension/background.js`), con cascada calibración → defaults del código en
  `extension/content.js` (`loadSelectors()` / `activeSelectors`).
- **Actualización en caliente sin recarga**: `background.js` hace broadcast de
  `UPDATE_SELECTORS` a todas las pestañas; `content.js` lo escucha y aplica.
- **Comandos IPC**: `calibrate`, `reset-selectors`, `get-active-selectors`,
  `probe-selectors` — documentados en el README (secciones "Comandos disponibles" y
  "Flujo de calibración").

Claves calibrables: `INPUT`, `SUBMIT_BUTTON`, `RESPONSE_CONTAINER`, `RESPONSE_TEXT`,
`THINKING_MARKERS`, `RESPONSE_READY_MARKERS`, `CITATION_NOISE`.

## Propuesta pendiente: Fase 2 — Visual Picker

Única parte aún NO implementada: un comando que resalta elementos en la página de
NotebookLM y permite al usuario seleccionarlos haciendo clic, auto-generando el selector
óptimo (evitando clases generadas `ng-*`, `mat-mdc-*`, `cdk-*` y prefiriendo clases
semánticas estables). Eliminaría la necesidad de inspeccionar el DOM manualmente en
DevTools durante una recalibración.
