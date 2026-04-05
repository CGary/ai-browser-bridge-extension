### Tarea 5: Formateo y Transmisión hacia Native Messaging
* Codificar la serialización de la carga útil JSON validada a formato UTF-8.
* Implementar la adición de un prefijo de 4 bytes (entero de 32 bits en orden de bytes nativo) al inicio del paquete JSON.
* Configurar la salida del Daemon para transmitir el paquete final exclusivamente a través de la salida estándar (`stdout`).
* Asegurar que la salida de `stdout` esté libre de secuencias de escape ANSI o decoraciones visuales para garantizar compatibilidad binaria con Chromium.