### Tarea 2: Implementación de Seguridad IPC (User Story 1.1)
* Establecer el servidor IPC utilizando un socket Unix en `/tmp/aibbe.sock`.
* Aplicar explícita y estrictamente la máscara de permisos `0600` al socket Unix tras su creación.
* Validar que la conexión local rechace interacciones de identificadores de usuario (UID) distintos al propietario, devolviendo un error de "Permission Denied" nativo del sistema operativo.