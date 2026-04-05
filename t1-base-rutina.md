### Tarea 1: Configuración Base y Rutina de Inicialización (Daemon)
* Configurar la estructura del binario residente (Daemon) en Go.
* Programar una rutina de inicialización que fuerce la eliminación de cualquier archivo residual en la ruta `/tmp/aibbe.sock` antes de intentar crear un nuevo enlace. 
* Esta mitigación prevé el riesgo de bloqueo de ejecución IPC por conflictos de estado en el sistema de archivos del anfitrión.