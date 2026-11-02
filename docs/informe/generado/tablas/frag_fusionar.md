<!-- Generado por tablas.py desde kmeans/parciales.go. No editar a mano. -->

::: {#lst-fusionar}
```go
func Fusionar(ps []Parcial, k, dim int) Parcial {
    f := nuevoParcial(k, dim)
    inercia := 0.0
    for c := range ps {
        p := &ps[c]
        if len(p.Sumas) != k*dim || len(p.Conteos) != k {
            panic(fmt.Sprintf("kmeans.Fusionar: el parcial %d tiene %d sumas y %d conteos, se esperaban %d y %d",
                c, len(p.Sumas), len(p.Conteos), k*dim, k))
        }
        for i := range f.Sumas {
            f.Sumas[i] += p.Sumas[i]
        }
        for j := range f.Conteos {
            f.Conteos[j] += p.Conteos[j]
        }
        inercia += p.Inercia
    }
    f.Inercia = inercia
    return f
}
```

Fusionar: la reducción exacta, en el orden dado (`kmeans/parciales.go`, líneas 78–97)
:::
