<!-- Generado por tablas.py desde cluster/pool.go. No editar a mano. -->

::: {#lst-pool-tomar}
```go
p.latir()
p.mu.Lock()
defer p.mu.Unlock()
if len(p.libres) < n {
    return nil, &ErrorFaltanNodos{Libres: len(p.libres), Pedidos: n}
}
nombres := make([]string, 0, len(p.libres))
for nombre := range p.libres {
    nombres = append(nombres, nombre)
}
slices.Sort(nombres)
ms := make([]*Miembro, n)
for i, nombre := range nombres[:n] {
    e := p.libres[nombre]
    delete(p.libres, nombre)
    p.ocupados[e.m] = e
    ms[i] = e.m
}
return ms, nil
```

Tomar: un entrenamiento se lleva n nodos libres del pool, ordenados por nombre (`cluster/pool.go`, líneas 283–301)
:::
