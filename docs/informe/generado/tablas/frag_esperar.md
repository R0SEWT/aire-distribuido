<!-- Generado por tablas.py desde protocolo/coordinador.go. No editar a mano. -->

::: {#lst-esperar}
```go
switch {
case m.Tipo == tipo && (tipo != Parcial || m.Iter == iter):
    return m, nil
case m.Tipo == Pong:
    continue
case m.Tipo == Parcial && m.Iter <= e.ultimaIter:
    e.Rezagados++
    continue
case m.Tipo == Parcial:
    return Mensaje{}, fmt.Errorf("%w: PARCIAL de la ronda %d, que no se envió (la última es la %d)",
        ErrProtocolo, m.Iter, e.ultimaIter)
```

Mientras se espera el PARCIAL de una ronda: se descartan los PONG sueltos y los parciales rezagados (`protocolo/coordinador.go`, líneas 269–279)
:::
