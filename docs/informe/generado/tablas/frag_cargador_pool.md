<!-- Generado por tablas.py desde datos/pipeline.go. No editar a mano. -->

::: {#lst-cargador-pool}
```go
for range workers {
    wg.Add(1)
    go func() {
        defer wg.Done()
        w := nuevoTrabajador(reg, cols, delim)
        for i := range trabajos {
            select {
            case resultados <- w.procesar(r, i, rangos[i]):
            case <-fin:
                return
            }
        }
    }()
}
```

El worker pool del cargador: cada goroutine procesa los bloques que le llegan por el canal (`datos/pipeline.go`, líneas 35–48)
:::
