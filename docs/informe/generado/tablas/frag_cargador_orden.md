<!-- Generado por tablas.py desde datos/pipeline.go. No editar a mano. -->

::: {#lst-cargador-orden}
```go
for p := range resultados {
    if primerErr != nil {
        continue // se drena el canal para que ningún worker quede colgado
    }
    pendientes[p.indice] = p
    for q, ok := pendientes[siguiente]; ok; q, ok = pendientes[siguiente] {
        delete(pendientes, siguiente)
        inicio := time.Now()
        // El error de un bloque se mira recién en su turno: si el bloque
        // anterior dejó un registro abierto, este empezó dentro de un campo
        // y su resultado (con o sin error) se descarta y se rehace.
        q, err := c.coser(q)
        if err == nil {
            a.fusionar(q)
        }
        a.tFusion += time.Since(inicio)
        siguiente++
        <-enVuelo
        if err != nil {
            primerErr = err
            close(fin)
            break
        }
    }
}
```

El recolector: los parciales llegan en cualquier orden y se fusionan en orden de bloque (`datos/pipeline.go`, líneas 76–100)
:::
