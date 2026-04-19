# Propuesta Técnica: Sistema de Calibración Dinámica de Selectores (DCS)

## 1. El Problema
El ciclo actual de desarrollo para adaptar la extensión a cambios en el DOM de NotebookLM es ineficiente:
`Inspeccionar -> Editar Código -> Recargar Extensión -> Refrescar Pestaña -> Probar`.
Cualquier cambio menor en la UI de Google (clases CSS generadas dinámicamente) rompe la integración y requiere intervención manual en el código fuente.

## 2. La Solución: Arquitectura de Overrides
Transformar los selectores de constantes estáticas en el código a un sistema de **configuración persistente y en tiempo real**.

### 2.1. Almacenamiento y Precedencia
La extensión utilizará `chrome.storage.local` para gestionar los selectores. El orden de prioridad será:
1.  **Local Storage (Calibraciones):** Selectores guardados por el usuario/CLI.
2.  **Código Fuente (Defaults):** Selectores base definidos en `content.js`.

### 2.2. Flujo de Trabajo Dinámico (Sin Recargas)
1.  El **Daemon** recibe un comando de calibración desde la CLI.
2.  El **Background Script** envía los nuevos selectores a todas las pestañas registradas.
3.  El **Content Script** actualiza su objeto `SELECTORS` en memoria inmediatamente y lo guarda en `chrome.storage`.
4.  **Resultado:** Los siguientes comandos de inyección usan los nuevos selectores sin necesidad de recargar la extensión ni la página.

## 3. Implementación Técnica

### 3.1. Nuevos Comandos IPC
- `calibrate`: Recibe un JSON con los selectores a actualizar (ej. `{"INPUT": ".new-class"}`).
- `reset-selectors`: Borra todas las calibraciones y vuelve a los valores de fábrica del código.
- `get-active-selectors`: Devuelve los selectores que se están usando actualmente (útil para depuración).

### 3.2. Cambios en Content Script
- Sustituir `const SELECTORS` por una variable `let activeSelectors`.
- Implementar una función `loadSelectors()` que se ejecute al inicio y tras recibir mensajes de calibración.
- Escuchar mensajes del tipo `UPDATE_SELECTORS`.

### 3.3. Interacción vía CLI
Se habilitará un nuevo modo en `aibbe-cli`:
```bash
# Ejemplo de calibración rápida desde terminal
./aibbe-cli -cmd "calibrate" -payload '{"INPUT": "#new-input-id", "SUBMIT_BUTTON": ".btn-send"}'
```

## 4. Beneficios
1.  **Productividad:** Tiempo de ajuste reducido de minutos a segundos.
2.  **Resiliencia:** Permite corregir la extensión en entornos de producción (Docker) sin tocar archivos ni reconstruir imágenes.
3.  **Agnosticismo:** Facilita la adaptación de la extensión a otros servicios de IA (Claude, Gemini, ChatGPT) simplemente cambiando los selectores sobre la marcha.

## 5. Próximos Pasos (Fase 2)
Una vez establecida la persistencia, se implementará el **Visual Picker**: un comando que resalta elementos en la página de NotebookLM y permite al usuario seleccionarlos haciendo clic, auto-generando el selector óptimo.
