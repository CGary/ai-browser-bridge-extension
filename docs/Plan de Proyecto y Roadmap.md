# Plan de Proyecto y Roadmap: aibbe

## 1. Resumen de Ejecución y Equipo
El proyecto se ejecutará bajo un modelo de recurso único (1 FTE - Senior Software Engineer), aplicando un enfoque de desarrollo estrictamente secuencial. La metodología operativa es Kanban de tracción continua, estableciendo un límite de Trabajo en Progreso (WIP) de una sola tarea activa. El alcance del Producto Mínimo Viable (MVP) está restringido a la ruta de ejecución exitosa ("Happy Path"), posponiendo el manejo avanzado de excepciones, desincronización de flujos y control estricto de latencia para iteraciones posteriores. El entorno de desarrollo y despliegue objetivo asume compilación local nativa optimizada para ecosistemas basados en Debian 12.

## 2. Roadmap e Hitos Principales (Milestones)
* **Hito 1 (M1): Infraestructura IPC Segura.** Despliegue del binario Daemon y establecimiento del canal de comunicación local seguro (Socket Unix) con la CLI.
* **Hito 2 (M2): Puente Native Messaging.** Configuración del manifiesto de Chromium y establecimiento de la tubería de datos síncrona/asíncrona entre el Daemon y el Background Script.
* **Hito 3 (M3): Orquestador de Pestañas.** Implementación del registro de estado en memoria para el enrutamiento determinista de requerimientos hacia la interfaz web activa.
* **Hito 4 (M4): Motor de Inyección y Extracción (RAG Local).** Automatización de la inyección de contexto en el DOM objetivo y captura de la salida generada (Happy Path).
* **Hito 5 (M5): Pruebas End-to-End (MVP).** Validación transaccional completa desde la invocación en CLI hasta el retorno del código fuente autogenerado.

## 3. Planificación de Sprints (Backlog Mapping)
Dada la selección de un modelo Kanban, las iteraciones se definen como lotes de trabajo secuenciales extraídos según su dependencia arquitectónica en el SDD:

* **Iteración Kanban 1: Capa de Transporte Local (Épica 1)**
    * **Implementar:** User Story 1.1 (Aislamiento de seguridad IPC en Socket Unix con permisos `0600`).
    * **Implementar:** User Story 1.2 (Validación síncrona de entrada de datos - Escenario 1: Carga útil válida < 1 MB).
    * *Postergado:* User Story 1.3 (Manejo de desincronización del protocolo Native Messaging) y Escenario 2 de US 1.2.

* **Iteración Kanban 2: Capa de Enrutamiento Lógico (Épica 2)**
    * **Implementar:** User Story 2.1 (Gestión del ciclo de vida y Handshake de interfaces web).
    * **Implementar:** User Story 2.2 (Enrutamiento exclusivo de requerimientos a `tabId` libre).

* **Iteración Kanban 3: Capa de Manipulación DOM (Épica 3)**
    * **Implementar:** User Story 3.1 (Inyección imperceptible de contexto técnico).
    * **Implementar:** Lógica de extracción reactiva post-mutación (Escenario 1 de User Story 3.2).
    * *Postergado:* Escenario 2 de User Story 3.2 (Timeout estricto y recolección de basura por latencia).

## 4. Matriz de Riesgos y Mitigación
* **Riesgo 1: Límite de Memoria de Chromium Native Messaging (1 MB).** Al omitir el manejo de errores complejos en el MVP, un payload excesivo cerrará el canal abruptamente. 
  * *Mitigación:* Implementar un validador de longitud estricto (bloqueante) en el paso de mensajes de la CLI antes de enviar al socket Unix.
* **Riesgo 2: Bloqueo de Ejecución IPC en Entorno Local.** Permisos incorrectos en el socket `/tmp/aibbe.sock` impedirán la comunicación inicial, deteniendo el flujo completo.
  * *Mitigación:* Codificar la eliminación forzada del archivo `.sock` en la rutina de inicialización del Daemon en Go antes de aplicar el enmascaramiento `0600` para prevenir conflictos de estado residual en el sistema de archivos de la máquina anfitriona.
* **Riesgo 3: Procesos Zombis por Ausencia de Timeout.** Al postergar la US 3.2 (Escenario 2), mutaciones inesperadas en la interfaz de la IA dejarán la CLI en estado de espera indefinida.
  * *Mitigación:* Establecer un atajo de teclado a nivel de CLI (ej. `Ctrl+C`) que cierre el socket local y finalice la transacción del lado del cliente, asumiendo la pérdida de sincronía del `tabId` en el navegador hasta el reinicio de la pestaña.