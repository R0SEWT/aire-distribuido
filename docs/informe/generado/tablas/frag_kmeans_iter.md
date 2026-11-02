<!-- Generado por tablas.py desde kmeans/concurrente.go. No editar a mano. -->

::: {#lst-kmeans-iter}
```go
for it := 1; it <= op.MaxIter; it++ {
    repartir(trabajos, &barrera, cantChunks, false)
    if cerrarIteracion(r, cent, Fusionar(parciales, k, dim), it, op.Opciones) {
        break
    }
}

// repartir manda los chunks al pool y espera la barrera.
func repartir(trabajos chan<- trabajo, barrera *sync.WaitGroup, cantChunks int, asignar bool) {
    barrera.Add(cantChunks)
    for c := 0; c < cantChunks; c++ {
        trabajos <- trabajo{chunk: c, asignar: asignar}
    }
    barrera.Wait()
}
```

Una iteración: repartir los chunks, esperar la barrera y fusionar en orden de chunk (`kmeans/concurrente.go`, líneas 85–90 y 102–109)
:::
