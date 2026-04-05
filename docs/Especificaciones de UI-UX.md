# Especificaciones de UI/UX: aibbe

## 1. Arquitectura de la Información
La arquitectura carece de interfaz gráfica tradicional; el enrutamiento visual se sustituye por el flujo de datos unidireccional a través de los canales estándar del sistema operativo y el protocolo Native Messaging.

* **Capa de Entrada (CLI):** Receptor de argumentos y carga útil en formato JSON.
* **Capa de Transporte Local (Daemon):** Socket Unix (`/tmp/aibbe.sock`) con restricción de acceso estricta (`0600`). Funciona como puente de validación síncrona y formateo binario.
* **Capa de Enrutamiento Lógico (Background Script):** Registro en memoria volátil de interfaces web (DOMs) disponibles, categorizadas por servicio y estado (`libre` | `ocupado`).
* **Capa de Interacción (Content Script):** Manipulador del DOM en modo *headless virtual* para inyección de prompts y extracción de respuestas mediante `MutationObserver`.
* **Capa de Salida (Terminal):** Bifurcación estricta de canales; `stdout` exclusivo para la carga útil resultante y `stderr` para la telemetría de errores.

## 2. Descripción de Vistas Principales (Wireframes Lógicos)
Las "vistas" se abstraen en los flujos de salida de la terminal y las consolas de depuración, optimizadas para la lectura humana técnica y la canalización automatizada (piping).

* **Vista de Salida Estándar (`stdout`):** Interfaz aséptica. Emite de forma exclusiva el bloque de código fuente resultante en texto plano puro, sin caracteres de escape ANSI, metadatos de progreso o decoraciones visuales, asegurando la compatibilidad absoluta con operadores de redirección (ej. `>`, `|`).
* **Vista de Error Estándar (`stderr`):** Interfaz de observabilidad. Renderiza cadenas de texto estructuradas bajo el formato `[NIVEL] [COMPONENTE] Descripción del evento`.
* **Panel de Monitoreo de Estado (Consola de Extensión):** Accesible vía `chrome://extensions`. Actúa como registro de solo lectura de las transiciones de estado del inventario determinista (registro y purga de `tabId`s, cambios entre `libre` y `ocupado`).

## 3. Flujos de Interacción Críticos (User Flows)
**Flujo Principal: Generación y Extracción Exitosa**
1.  **Invocación:** El usuario ejecuta el comando CLI con la carga útil.
2.  **Validación IPC:** El Daemon verifica los permisos del socket, valida la estructura JSON y el límite de 1 MB.
3.  **Transmisión:** El Daemon empaqueta el mensaje con el prefijo de 4 bytes hacia el Background Script vía `stdout` (entorno de Chromium).
4.  **Enrutamiento:** El Background Script localiza un `tabId` compatible en estado `libre`, lo marca como `ocupado` y transfiere la petición.
5.  **Inyección y Espera:** El Content Script inserta el prompt imperceptiblemente en el DOM y activa el `MutationObserver`.
6.  **Extracción y Retorno:** Al detectar el código renderizado, se extrae el texto plano, se cancela el observador y la carga útil viaja de retorno hasta el Daemon.
7.  **Finalización:** El Daemon imprime la respuesta en `stdout` y finaliza con código de salida `0`.

**Flujo de Excepción DOM: Tiempo de Espera Agotado (Timeout)**
1.  **Ejecución:** Pasos 1 al 5 del flujo principal completados.
2.  **Límite Alcanzado:** El temporizador interno del Content Script agota el límite sin detectar la mutación esperada en el DOM.
3.  **Interrupción:** Se aborta el `MutationObserver` y el Content Script emite una excepción tipificada al Background Script.
4.  **Liberación:** El Background Script cambia el estado del `tabId` a `libre`.
5.  **Reporte de Fallo:** El Daemon recibe la excepción, imprime `[ERROR] [Content Script] Timeout en la observación de mutación` en la salida `stderr`.
6.  **Terminación Forzada:** El Daemon invoca `os.Exit(1)`, finalizando la ejecución con código de salida distinto de cero para interrumpir cualquier pipeline dependiente.

## 4. Estados de Componentes y Manejo de Errores
El sistema implementa un patrón Fail-Fast para evitar procesos huérfanos y estados de espera indefinida.

* **Estado de Espera (Loading):** Representado por el bloqueo síncrono de la terminal durante la ejecución de la CLI. No se emiten indicadores de progreso visuales (spinners) para no contaminar los flujos de salida.
* **Estado de Fallo de Autenticación Local:** Al rechazar el sistema operativo la conexión al socket Unix por discrepancia de UID, el Daemon no registra actividad y la CLI falla nativamente con error de permisos (Permission Denied).
* **Estado de Fallo de Validación:** Si el JSON es inválido o excede 1 MB, el Daemon aborta sincrónicamente, retornando `[ERROR] [Daemon] Validación de carga útil fallida: Tamaño o estructura incorrecta` en `stderr` y estado `> 0`.
* **Estado de Desincronización Binaria:** Ante la recepción de datos truncados desde Chromium, el Daemon imprime `[FATAL] [Daemon] Desincronización de protocolo Native Messaging` en `stderr` y ejecuta una terminación inmediata.

## 5. Directrices de Accesibilidad y Responsividad
En el contexto de una herramienta de línea de comandos, la accesibilidad y responsividad se definen mediante la compatibilidad del sistema y la estandarización operativa.

* **Filosofía POSIX:** Estricta separación de datos funcionales (`stdout`) y metadatos/errores (`stderr`).
* **Interoperabilidad de Terminal:** Ausencia de secuencias de escape ANSI, garantizando que la salida sea procesable independientemente de las capacidades gráficas del emulador de terminal, del shell o del sistema operativo anfitrión.
* **Señalización Estándar:** Uso riguroso de códigos de estado de salida (Exit Status). `0` indica éxito absoluto, y cualquier valor `> 0` indica fallo, permitiendo la integración de aibbe en secuencias de comandos condicionales (ej. `aibbe request.json > output.js || echo "Fallo en la generación"`).
* **Invisibilidad Estructural (DOM):** La inyección de datos en la interfaz del proveedor de IA debe ejecutarse a nivel de manipulación de valores en los nodos nativos, sin incrustar nodos CSS/HTML artificiales que alteren los árboles de accesibilidad del navegador o la experiencia visual original.