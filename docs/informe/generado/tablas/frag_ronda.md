<!-- Generado por tablas.py desde protocolo/coordinador.go. No editar a mano. -->

::: {#lst-ronda}
```go
parciales := make([]Mensaje, len(enlaces))
errs := make([]error, len(enlaces))
var barrera sync.WaitGroup
barrera.Add(len(enlaces))
for i, e := range enlaces {
    go func() {
        defer barrera.Done()
        // Cada goroutine escribe solo su índice: no hace falta Mutex.
        parciales[i], errs[i] = e.ronda(iter, k, centroides, limite, plazo)
    }()
}
barrera.Wait()

if err := errors.Join(errs...); err != nil {
    return nil, fmt.Errorf("ronda %d abortada: %w", iter, err)
}
return parciales, nil
```

La barrera de una ronda: una goroutine por nodo, un WaitGroup y los parciales en orden de nodo (`protocolo/coordinador.go`, líneas 192–208)
:::
