<!-- Generado por tablas.py desde api/entrenamientos.go. No editar a mano. -->

::: {#lst-semaforo}
```go
// El semáforo: un entrenamiento a la vez. No se espera: si está tomado, 409.
select {
case c.sem <- struct{}{}:
default:
    c.mu.Lock()
    enCurso := c.enCurso
    c.mu.Unlock()
    if enCurso == "" { // otro pedido lo tomó y todavía no tiene id
        escribirError(w, http.StatusConflict, "ya hay un entrenamiento en curso")
    } else {
        escribirError(w, http.StatusConflict, "ya hay un entrenamiento en curso: "+enCurso)
    }
    return
```

Un entrenamiento a la vez: un canal de capacidad 1 como semáforo, sin esperar (`api/entrenamientos.go`, líneas 266–278)
:::
