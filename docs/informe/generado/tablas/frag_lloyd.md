<!-- Generado por tablas.py desde cluster/lloyd.go. No editar a mano. -->

::: {#lst-lloyd}
```go
for it := 1; it <= op.MaxIter; it++ {
    ms, barrera, err := ronda(n)
    if err != nil {
        return c, err
    }
    fusion := time.Now()
    p, err := fusionar(enlaces, ms, n, k, dim)
    if err != nil {
        return c, err
    }
    // Ronda ya volvió: ninguna goroutine sigue leyendo cent mientras se actualiza.
    convergio := kmeans.CerrarIteracion(r, cent, p, it, op)
    c.Rondas = append(c.Rondas, barrera)
    c.Fusiones = append(c.Fusiones, time.Since(fusion))
    if ol.AlRonda != nil {
        porNodo := make([]time.Duration, len(enlaces))
        for i, e := range enlaces {
            porNodo[i] = e.TiempoRonda
        }
        ol.AlRonda(Ronda{Iter: it, Inercia: p.Inercia, Barrera: barrera, Fusion: c.Fusiones[it-1], PorNodo: porNodo,
            Centroides: slices.Clone(cent)})
    }
    n++
    if convergio {
        break
    }
}
```

Lloyd sobre el clúster: ronda, fusión en orden de nodo, cierre de la iteración y aviso a quien la sigue (`cluster/lloyd.go`, líneas 233–259)
:::
