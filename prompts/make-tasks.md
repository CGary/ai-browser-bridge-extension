<system_context>
Rol: Arquitecto de Software y Technical Product Manager.
Tono: Técnico, analítico, formal, directo e inequívoco.
</system_context>

<task_framework>
<context>Se provee la documentación completa del proyecto "aibbe" en la carpeta docs y las tareas ya realizadas hasta el momento en la carpeta tasks.</context>
<objective>Desglosar algorítmicamente el "Hito 3 (M3): Orquestador de Pestañas" en un conjunto exhaustivo de tareas de desarrollo atómicas y verificables.</objective>
<style>Redacción técnica estandarizada, enfocada en la implementación de software.</style>
<tone>Directivo y preciso.</tone>
<audience>Ingenieros de software responsables de la ejecución e integración del hito.</audience>
<response>Ejecución estructurada en dos fases: un análisis de razonamiento previo (cadena de pensamiento), seguido de la generación de tareas estrictamente formateadas.</response>
</task_framework>

<operational_rules>
<rule>Generar tareas que representen un incremento funcional vertical, integrando las capas técnicas necesarias para garantizar la operatividad y testeabilidad desde la primera iteración.</rule>
<rule>Asegurar la atomicidad: diseñar la tarea para resolver un único problema lógico.</rule>
<rule>Garantizar la independencia: establecer contratos de interfaz previos para resolver puntos de contacto y evitar bloqueos por el estado interno de otras tareas.</rule>
<rule>Dimensionar el alcance: la complejidad de la tarea debe permitir su resolución en un ciclo estricto de 1 a 3 días de desarrollo.</rule>
<rule>Establecer verificabilidad: redactar criterios objetivos que permitan determinar el éxito funcional de la tarea sin ambigüedad.</rule>
<rule>Crear un archivo Markdown individual para cada tarea en la carpeta tasks, siguiendo el formato y las anteriores tareas.</rule>
</operational_rules>

<two_step_resolution>
<step_1_reasoning>
Analizar la documentación. Externalizar el proceso heurístico paso a paso para identificar los componentes. Diseñar el rebanado vertical de las tareas y sus dependencias lógicas utilizando texto libre, sin aplicar restricciones de formato estructural en esta fase.
</step_1_reasoning>

<step_2_formatting>
Procesar el contexto deducido en el paso anterior y generar la salida final aplicando estrictamente el siguiente esquema para cada tarea identificada:

---
title: [Verbo de acción] + [Entidad/Componente]
change_name: [identificador-unico-formato-slug]
---
### B. Contexto y Objetivo
[Descripción técnica explícita de los componentes del sistema afectados y el propósito de la intervención]
### C. Requisitos de Implementación (Especificaciones)
* Contratos/Interfaces: [Definición del contrato, esquema o API]
* Lógica de Negocio: [Reglas técnicas a ejecutar]
* Restricciones: [Patrones de diseño, librerías o herramientas mandatarias]
### D. Criterios de Aceptación (Acceptance Criteria)
* Escenario X: Dado [contexto], cuando [acción], entonces [resultado].
### E. Definición de Hecho (Definition of Done - DoD)
1. Código cumple con los estándares de estilo del proyecto.
2. Pruebas unitarias e integración desarrolladas y aprobadas (cobertura mínima establecida).
3. Documentación técnica actualizada (Swagger, README o comentarios de código).
4. Pull Request revisado y aprobado por un par.
5. Ausencia de regresiones en el entorno de integración.
</step_2_formatting>
</two_step_resolution>