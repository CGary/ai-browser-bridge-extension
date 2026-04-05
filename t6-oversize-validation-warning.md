---
title: Aclarar y validar el escenario de payload > 1 MiB
tags: verification-warning
---

## Descripción
La verificación de `t4-validacion-sincronica` dejó un warning: el escenario “payload mayor a 1 MiB” solo se demuestra en el helper `internal/nativemessaging.WriteMessage`, pero nunca en el daemon real, ya que `ipc.MaxRequestSize` (también 1 MiB) rechaza la petición antes de que el daemon llame al helper y loguee `payload exceeds native messaging limit`. Esto impide tener evidencia de runtime sobre el límite y el `go test -cover` subreporta cobertura para `daemon` porque varias pruebas crean y ejecutan un binario externo.

## Criterios de aceptación
1. Documentar en la especificación o el diseño si el error por payload demasiado grande debe pertenecer al límite IPC, al helper de Native Messaging, o a ambos, de modo que los verificadores sepan dónde encontrar la prueba de comportamiento.  
2. Añadir o adaptar una prueba que dispare el camino `nativemessaging.WriteMessage` desde el mismo proceso del daemon para que se loguee el mensaje de error y no se ACK al CLI cuando la request excede 1 MiB.  
3. Evaluar alternativas (tests adicionales sin binario externo, instrumentación) para que la cobertura de `daemon` refleje mejor el comportamiento real y no dependa exclusivamente de procesos spawneados.
