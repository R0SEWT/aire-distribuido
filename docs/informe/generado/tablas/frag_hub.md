<!-- Generado por tablas.py desde api/hub.go. No editar a mano. -->

::: {#lst-hub}
```go
repartir := func(d difusion) {
    for c := range clientes {
        if d.roles != nil && !slices.Contains(d.roles, c.rol) {
            continue
        }
        select {
        case c.envios <- d.trama:
        default:
            h.lentos.Add(1)
            h.log.Warn("websocket: cliente lento, se desconecta", "remoto", c.conn.RemoteAddr().String(), "pendientes", len(c.envios))
            soltar(c, motivoBaja{cierrePolitica, "cliente lento: no lee al ritmo de los eventos"})
        }
    }
}
```

El hub reparte un evento sin esperar a nadie: el cliente con el búfer lleno se desconecta (`api/hub.go`, líneas 126–139)
:::
