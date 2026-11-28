<!-- Generado por tablas.py desde protocolo/coordinador.go. No editar a mano. -->

::: {#lst-enlace-ronda}
```go
func (e *Enlace) ronda(iter, k int, centroides []float64, limite time.Time, plazo time.Duration) (Mensaje, error) {
    // Se marca antes de enviar: desde este momento, cualquier PARCIAL de una
    // ronda anterior es un rezagado.
    e.ultimaIter = iter
    inicio := time.Now()
    defer func() { e.TiempoRonda = time.Since(inicio) }()
    if err := e.enviar(Mensaje{Tipo: Iterar, Iter: iter, Centroides: centroides}, limite); err != nil {
        return Mensaje{}, e.fallo("enviar ITERAR: %w", err)
    }
    m, err := e.esperar(Parcial, iter, limite)
    if errors.Is(err, ErrPlazoVencido) {
        return Mensaje{}, e.fallo("no envió su PARCIAL dentro del plazo (%v): %w", plazo, err)
    }
    if err != nil {
        return Mensaje{}, e.fallo("esperando PARCIAL: %w", err)
    }
```

Lo que hace la goroutine de cada nodo en una ronda: escribe ITERAR en su socket y bloquea leyendo el PARCIAL (`protocolo/coordinador.go`, líneas 212–228)
:::
