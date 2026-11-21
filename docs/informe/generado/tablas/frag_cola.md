<!-- Generado por tablas.py desde api/escritor.go. No editar a mano. -->

::: {#lst-cola}
```go
func (c *cola) encolar(p almacen.Prediccion) bool {
    select {
    case c.ch <- p:
        return true
    default:
        c.descartadas.Add(1)
        return false
    }
}

func (c *cola) completar(lote []almacen.Prediccion) []almacen.Prediccion {
    for len(lote) < maxLote {
        select {
        case p := <-c.ch:
            lote = append(lote, p)
        default:
            return lote
        }
    }
    return lote
}
```

La cola de escritura de un almacén: encolar nunca bloquea y el lote toma lo que ya esté esperando (`api/escritor.go`, líneas 136–144 y 177–187)
:::
