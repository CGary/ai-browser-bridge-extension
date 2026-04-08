# Software Design Document (SDD): aibbe

## 1. Visión General de la Arquitectura
El sistema implementa una arquitectura híbrida diseñada para orquestar interacciones automatizadas con interfaces web de inteligencia artificial. Se fundamenta en un modelo de comunicación de paso de mensajes asíncrono gestionado por un daemon residente, el cual expone una interfaz síncrona y transaccional hacia una herramienta de línea de comandos (CLI). El puente principal de comunicación aprovecha la API Native Messaging de Chromium, aislando la lógica de interacción del Document Object Model (DOM) dentro de una extensión de navegador dedicada y manteniendo el entorno local agnóstico a los cambios estructurales en las plataformas de terceros.

## 2. Diagrama de Componentes (Descripción lógica)
* **CLI (Cliente Efímero):** Proceso sin estado que captura la entrada del ingeniero, transmite el requerimiento a través de un socket Unix y bloquea el hilo de ejecución hasta recibir una respuesta definitiva (código fuente autogenerado o excepción).
* **Daemon (Host de Native Messaging):** Binario residente desarrollado en Go. Mantiene el estado de la conexión persistente con el navegador a través de los flujos estándar (`stdin`/`stdout`). Opera como un servidor IPC (Inter-Process Communication) sobre un socket Unix (ej. `/tmp/aibbe.sock`), traduciendo las peticiones síncronas de la CLI en eventos asíncronos para la extensión.
* **API Native Messaging de Chromium:** Interfaz estándar del navegador que facilita y securiza el intercambio de cargas útiles entre la extensión y el binario anfitrión.
* **Extensión de Chromium:**
    * *Background Script / Service Worker:* Escucha los eventos provenientes del daemon, gestiona el enrutamiento bidireccional de los mensajes hacia pestañas de NotebookLM y controla el ciclo de vida de la inyección de scripts.
    * *Content Script:* Componente inyectado dinámicamente en `notebooklm.google.com`. Ejecuta el mapeo de selectores, la inserción de contexto (RAG automatizado) y la extracción del código fuente validado.

## 3. Modelo de Datos y Almacenamiento
La arquitectura opera bajo un paradigma de almacenamiento estrictamente volátil. El ciclo de vida de los datos, incluyendo el contexto técnico inyectado, las plantillas de prompts y el código resultante, reside de manera exclusiva en la memoria operativa (RAM) durante la ejecución transaccional del comando. Se omite de forma deliberada la persistencia en el sistema de archivos local y la implementación de APIs de almacenamiento del navegador (ej. `chrome.storage.local`) para garantizar la latencia mínima y satisfacer el alcance del MVP.

## 4. Diseño de Interfaces (APIs)
* **Contrato de Datos (Daemon - Extensión):** Utiliza un esquema de tipado dinámico basado en texto (JSON). La comunicación cumple estrictamente con las especificaciones del protocolo Native Messaging: cada transmisión está precedida por un entero de 32 bits (4 bytes) en orden de bytes nativo que indica la longitud del mensaje, seguido del objeto JSON codificado en UTF-8.
* **Contrato de Datos (CLI - Daemon):** La transferencia de instrucciones a través del socket Unix emplea el mismo esquema JSON, asegurando la interoperabilidad de las estructuras de datos y facilitando la depuración directa del flujo interno sin la sobrecarga de formatos de serialización binaria en esta iteración.

## 5. Infraestructura y Despliegue
El modelo de distribución se basa en la transferencia de código fuente, descartando la implementación de pipelines de integración continua (CI/CD) o empaquetado de binarios.
* **Compilación Local:** El código del daemon requiere compilación explícita mediante las cadenas de herramientas de Go directamente en el entorno de desarrollo anfitrión (optimizado y probado para entornos Debian 12).
* **Registro de Native Messaging:** Exige la creación y configuración manual del archivo de manifiesto JSON en los directorios de configuración del usuario (ej. `~/.config/chromium/NativeMessagingHosts/aibbe.json`), estableciendo la política de orígenes permitidos y la ruta absoluta al ejecutable compilado.
* **Carga de Extensión (Sideloading):** Requiere la instalación manual de la extensión no empaquetada a través de la interfaz de gestión de Chromium en modo de desarrollador, evadiendo los tiempos de validación de la Chrome Web Store.

## 6. Consideraciones de Seguridad
* **Manejo de Excepciones (Patrón Fail-Fast):** El sistema rechaza la implementación de políticas de reintentos automáticos ante mutaciones en el DOM. Cualquier fallo en la aserción de selectores o error de inyección aborta la secuencia inmediatamente. La excepción se propaga de manera síncrona hacia la CLI, terminando el proceso con un código de salida distinto de cero y requiriendo la intervención técnica para el ajuste del código de la extensión.
* **Aislamiento IPC:** Los permisos del socket Unix local deben restringirse al usuario propietario del sistema que ejecuta la sesión del navegador (`chmod 0600`). Esta medida es crítica para mitigar vectores de escalada de privilegios o la ejecución de comandos arbitrarios por parte de otros procesos o usuarios concurrentes en la misma máquina.
* **Jerarquía de Guardarraíles de Tamaño:** La validación de tamaño opera en dos capas. La capa **IPC** es el guardarraíl primario y rechaza cualquier solicitud de la CLI que exceda 1 MiB antes de que llegue a la lógica del daemon. La capa **Native Messaging** es un guardarraíl secundario y defensivo: vuelve a validar el tamaño justo antes de escribir hacia Chromium para cubrir futuros casos donde el daemon construya mensajes internos más grandes que la carga original recibida por IPC.
* **Chequeo Defensivo en Native Messaging:** Aunque en el flujo actual el daemon reenvía el payload IPC sin expandirlo, el límite de 1 MiB en Native Messaging se conserva como red de seguridad del protocolo. Esto documenta explícitamente que IPC maneja los límites de entrada externos, mientras que Native Messaging protege la salida del host frente a crecimiento interno futuro.
