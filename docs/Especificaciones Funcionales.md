# Especificaciones Funcionales: aibbe

## Épica 1: Gestión de Comunicación y Seguridad Local (CLI & Daemon)
### User Story 1.1: Aislamiento de seguridad IPC en Socket Unix
**Como** sistema anfitrión (Daemon)
**Quiero** inicializar el socket Unix de comunicación local aplicando estrictamente los permisos de sistema de archivos `0600`
**Para** garantizar que únicamente el usuario propietario del proceso posea privilegios de lectura y escritura, mitigando vectores de ataque por escalada de privilegios o inyección desde procesos concurrentes.

**Criterios de Aceptación:**
* **Escenario 1:** Conexión de cliente autorizado
  * **Dado** que el Daemon ha inicializado el socket Unix `/tmp/aibbe.sock` con permisos `0600`
  * **Cuando** el usuario propietario del sistema operativo (mismo UID) invoca la CLI para enviar un requerimiento
  * **Entonces** el sistema operativo permite la conexión IPC y el Daemon procesa el requerimiento.
* **Escenario 2:** Intento de acceso no autorizado
  * **Dado** que el Daemon ha inicializado el socket Unix con permisos `0600`
  * **Cuando** un proceso o usuario distinto al propietario del sistema operativo intenta escribir o leer en el socket
  * **Entonces** el sistema operativo bloquea la interacción nativamente mediante un error de permisos (Permission Denied) y el Daemon no registra ninguna conexión.

### User Story 1.2: Validación síncrona de entrada de datos
**Como** sistema anfitrión (Daemon)
**Quiero** interceptar y validar estructuralmente la carga útil JSON proveniente de la CLI antes de su transferencia
**Para** asegurar el cumplimiento del contrato de datos, no superar el límite de 1 MB impuesto por la API Native Messaging y prevenir la finalización forzada del puente por parte de Chromium.

**Criterios de Aceptación:**
* **Escenario 1:** Carga útil válida y dentro de los límites de memoria
  * **Dado** que la CLI envía un requerimiento en formato JSON a través del socket Unix
  * **Cuando** el Daemon verifica que contiene los campos obligatorios del esquema y el tamaño total es menor a 1 MB
  * **Entonces** el Daemon formatea el paquete con el prefijo de 4 bytes y lo transmite exitosamente a través de `stdout` hacia la extensión.
* **Escenario 2:** Carga útil inválida o sobredimensionada
  * **Dado** que la CLI envía un requerimiento a través del socket Unix
  * **Cuando** el Daemon detecta la ausencia de campos obligatorios o que el tamaño excede 1 MB
  * **Entonces** el Daemon aplica el patrón Fail-Fast, aborta la transacción localmente de forma síncrona, retorna un código de salida distinto de cero a la CLI e impide la transferencia hacia Chromium.

### User Story 1.3: Manejo de desincronización del protocolo Native Messaging
**Como** sistema anfitrión (Daemon)
**Quiero** abortar mi propia ejecución ante cualquier anomalía en el flujo binario de `stdin`
**Para** aplicar el patrón Fail-Fast, evitar el procesamiento de datos truncados y delegar la bitácora de errores a la salida de error estándar (`stderr`) capturada por Chromium.

**Criterios de Aceptación:**
* **Escenario 1:** Desincronización de carga útil
  * **Dado** que el Daemon está en estado de escucha activa
  * **Cuando** recibe un mensaje desde la extensión que no respeta el prefijo de longitud de 4 bytes
  * **Entonces** el Daemon emite un registro detallado del fallo a través de `stderr`, ejecuta `os.Exit(1)` para finalizar su proceso inmediatamente y desconecta el canal.

## Épica 2: Registro y Enrutamiento Determinista (Background Script)
### User Story 2.1: Gestión del ciclo de vida y Handshake de interfaces web
**Como** enrutador lógico (Background Script)
**Quiero** registrar y mantener el estado de disponibilidad (libre u ocupado) de cada `tabId` que cargue el Content Script
**Para** disponer de un inventario determinista que permita orquestar peticiones sin provocar colisiones en el DOM.

**Criterios de Aceptación:**
* **Escenario 1:** Inicialización de nueva interfaz web
  * **Dado** que el usuario abre una nueva pestaña hacia el dominio del proveedor de IA
  * **Cuando** el Content Script se inyecta y emite el evento de disponibilidad (Handshake)
  * **Entonces** el Background Script almacena el `tabId` asociado al servicio específico en su registro interno con el estado "libre".
* **Escenario 2:** Cierre de interfaz web rastreada
  * **Dado** que el sistema mantiene un registro activo de un `tabId`
  * **Cuando** el navegador emite el evento `chrome.tabs.onRemoved` para ese `tabId` específico
  * **Entonces** el Background Script purga inmediatamente ese registro de su memoria para prevenir enrutamientos a contextos inexistentes.

### User Story 2.2: Enrutamiento exclusivo de requerimientos
**Como** enrutador lógico (Background Script)
**Quiero** direccionar el mensaje entrante del Daemon únicamente al `tabId` correspondiente al servicio demandado que se encuentre en estado "libre"
**Para** evitar ejecuciones duplicadas y corrupción de la secuencia lógíca en múltiples pestañas activas.

**Criterios de Aceptación:**
* **Escenario 1:** Enrutamiento determinista exitoso
  * **Dado** que el Background Script recibe un requerimiento validado desde el Daemon solicitando el "Servicio X"
  * **Cuando** consulta su registro y localiza un `tabId` asociado al "Servicio X" en estado "libre"
  * **Entonces** cambia el estado de ese `tabId` a "ocupado", transfiere la carga útil exclusivamente a ese Content Script e ignora el resto de pestañas activas del mismo dominio.

## Épica 3: Orquestación de Interfaz y Extracción de Datos (Content Script)
### User Story 3.1: Inyección imperceptible de contexto técnico (RAG)
**Como** inyector de lógica (Content Script)
**Quiero** insertar el requerimiento en el DOM del proveedor manipulando nativamente los selectores de entrada
**Para** automatizar el inicio de la generación sin alterar el flujo de renderizado (reflow) ni inyectar artefactos visuales que invaliden el CSS original.

**Criterios de Aceptación:**
* **Escenario 1:** Inyección de contexto estándar
  * **Dado** que el Content Script recibe la carga útil desde el Background Script
  * **Cuando** localiza el selector de entrada definido para el proveedor e inserta el texto automatizado
  * **Entonces** la manipulación ocurre en modo *headless virtual* sin generar banners, superposiciones o alteraciones visuales ajenas a la interfaz original de la IA, y desencadena el evento de envío nativo.

### User Story 3.2: Extracción con interrupción estricta por latencia (Timeout)
**Como** inyector de lógica (Content Script)
**Quiero** condicionar la observación de las mutaciones del DOM a un límite de tiempo estricto
**Para** liberar el hilo de la CLI ante cambios estructurales en el proveedor y evitar estados de polling indefinido (procesos zombis).

**Criterios de Aceptación:**
* **Escenario 1:** Generación de código exitosa
  * **Dado** que el sistema ha emitido el requerimiento y el `MutationObserver` está activo
  * **Cuando** el proveedor renderiza el bloque de código fuente dentro del tiempo de espera estipulado
  * **Entonces** el Content Script extrae el texto, cancela el observador de latencia y retorna el código validado hacia el Background Script para su entrega a la CLI.
* **Escenario 2:** Tiempo de espera agotado
  * **Dado** que el sistema ha emitido el requerimiento y el observador de mutación está activo
  * **Cuando** se alcanza el límite de tiempo configurado sin que el DOM presente el resultado esperado
  * **Entonces** el Content Script aborta el `MutationObserver`, emite un evento JSON tipificado como excepción de "Timeout" hacia el Background Script y libera el estado de la pestaña a "libre", permitiendo a la CLI finalizar con error.