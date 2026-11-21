<!-- Generado por tablas.py desde api/entrenamientos.go. No editar a mano. -->

::: {#lst-al-ronda}
```go
alRonda := func(r cluster.Ronda) {
    ronda := almacen.Ronda{Iter: r.Iter, Inercia: r.Inercia, MsRonda: ms(r.Barrera), MsFusion: ms(r.Fusion),
        MsPorNodo: make(map[string]float64, len(enlaces))}
    for i, en := range enlaces {
        ronda.MsPorNodo[en.Nodo] = ms(r.PorNodo[i])
    }
    paraAlmacen <- ronda                                                 // al almacén, por su canal
    c.difundir(eventoRonda, eventoDeRonda{ID: e.ID, Ronda: ronda}, e.ID) // al hub, por el suyo
    puntos.dejar(puntoDe(r.Iter, op.K, r.Centroides))                    // a Redis, por su buzón
}
```

Cada ronda va a sus tres consumidores, cada uno por su canal (`api/entrenamientos.go`, líneas 394–403)
:::
