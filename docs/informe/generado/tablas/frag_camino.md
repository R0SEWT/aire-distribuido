<!-- Generado por tablas.py desde api/entrenamientos.go. No editar a mano. -->

::: {#lst-camino}
```go
var miembros []*cluster.Miembro
if c.pool == nil {
    err = &cluster.ErrorFaltanNodos{Pedidos: par.Nodos}
} else {
    miembros, err = c.pool.Tomar(par.Nodos)
}
if err != nil {
    escribirError(w, http.StatusUnprocessableEntity, err.Error())
    return
}
devolver := func(motivo string) {
    c.pool.Soltar(miembros, motivo)
    cluster.Abortar(cluster.Enlaces(miembros), motivo, 2*time.Second)
}
rangos, err := cluster.Repartir(par.Replicas*n, len(miembros))

// …
lanzado = true
go c.correr(e, miembros, rangos, inicio)
s.responder(w, r, http.StatusAccepted, struct {
    ID     string `json:"id"`
    Estado string `json:"estado"`
}{e.ID, almacen.EstadoEnCurso})

// …
_, err := cluster.CargarCtx(c.ctx, enlaces, rangos, cluster.HuellaDeSha256(c.ent.Sha256), t0.Add(c.ent.plazoCarga()))
if err == nil {
    corrida, err = cluster.LloydCtx(c.ctx, enlaces, inicio.Centroides, op,
        cluster.OpcionesLloyd{Plazo: c.ent.plazoRonda(), Primera: 1, AlRonda: alRonda})
}
close(paraAlmacen)
```

El camino de un entrenamiento dentro de la API: toma los nodos, responde 202 y corre en su goroutine (`api/entrenamientos.go`, líneas 302–316 y 340–345 y 411–416)
:::
