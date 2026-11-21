<!-- Generado por tablas.py desde cluster/reparto.go. No editar a mano. -->

::: {#lst-repartir}
```go
base, resto := n/k, n%k
rangos := make([]protocolo.Rango, k)
desde := 0
for i := range rangos {
    tam := base
    if i < resto {
        tam++
    }
    rangos[i] = protocolo.Rango{Desde: desde, Hasta: desde + tam}
    desde += tam
}
return rangos, nil
```

Repartir: rangos contiguos que difieren a lo sumo en una fila, en orden de nodo (`cluster/reparto.go`, líneas 27–38)
:::
