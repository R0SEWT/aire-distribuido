<!-- Generado por tablas.py desde api/puntos.go. No editar a mano. -->

::: {#lst-buzon}
```go
// dejar entrega el punto de una ronda sin esperar. Lo llama una sola
// goroutine, la de la corrida: por eso, después de sacar el punto viejo (o de
// que la otra goroutine se lo haya llevado), el lugar está libre.
func (b *buzonPuntos) dejar(p almacen.PuntoControl) {
    if b == nil {
        return
    }
    select {
    case b.ch <- p:
        return
    default:
    }
    select {
    case <-b.ch:
        b.cuenta.omitidos.Add(1)
    default:
    }
    b.ch <- p
}
```

El punto de control de cada ronda: un buzón de un lugar donde el punto nuevo reemplaza al que no se escribió (`api/puntos.go`, líneas 78–96)
:::
