<!-- Generado por tablas.py desde cluster/nodo.go. No editar a mano. -->

::: {#lst-nodo-iterar}
```go
// iterar calcula el PARCIAL de la partición con los centroides de la ronda.
func (n *Nodo) iterar(p *particion, m protocolo.Mensaje) protocolo.Mensaje {
    if p == nil {
        return n.error("ITERAR %d sin partición: falta un CARGAR que haya salido bien", m.Iter)
    }
    d := p.datos.D
    if len(m.Centroides)%d != 0 {
        return n.error("ITERAR %d: %d centroides no son k×D con D = %d", m.Iter, len(m.Centroides), d)
    }
    if p.rondas == 0 {
        n.Log.Printf("ITERAR: primera ronda (%d), k = %d", m.Iter, len(m.Centroides)/d)
    }
    inicio := time.Now()
    parcial := kmeans.ParcialesPorChunks(p.datos, m.Centroides, max(1, n.Workers), n.chunk())
    p.calculo += time.Since(inicio)
    p.rondas++
    return protocolo.Mensaje{Tipo: protocolo.Parcial, Iter: m.Iter, Sumas: parcial.Sumas, Conteos: parcial.Conteos, Inercia: parcial.Inercia}
}
```

El nodo ante un ITERAR: el parcial de su rango, con su propio pool de workers (`cluster/nodo.go`, líneas 282–299)
:::
