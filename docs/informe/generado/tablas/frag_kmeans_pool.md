<!-- Generado por tablas.py desde kmeans/concurrente.go. No editar a mano. -->

::: {#lst-kmeans-pool}
```go
for w := 0; w < op.Workers; w++ {
    go func() {
        defer pool.Done()
        for t := range trabajos {
            ini := t.chunk * op.Chunk
            fin := min(ini+op.Chunk, n)
            if t.asignar {
                parciales[t.chunk] = Asignar(d, cent, ini, fin, asign)
            } else {
                parciales[t.chunk] = Parciales(d, cent, ini, fin)
            }
            barrera.Done()
        }
    }()
}
```

El worker pool del K-Means: cada worker escribe solo el parcial de su chunk (`kmeans/concurrente.go`, líneas 63–77)
:::
