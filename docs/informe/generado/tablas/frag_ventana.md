<!-- Generado por tablas.py desde ventana/ventana.go. No editar a mano. -->

::: {#lst-ventana}
```go
func (w *ventana) agregar(p punto) almacen.Media24h {
    i, existe := slices.BinarySearchFunc(w.puntos, p.h, compararH)
    if existe {
        w.puntos[i] = p
    } else {
        w.puntos = slices.Insert(w.puntos, i, p)
        if len(w.puntos) > Guardadas {
            // Sale la más lejana a la que acaba de llegar, que está en un extremo
            // porque están ordenadas; a igual distancia, la más vieja.
            primera, ultima := w.puntos[0].h, w.puntos[len(w.puntos)-1].h
            if p.h-primera >= ultima-p.h {
                w.puntos = slices.Delete(w.puntos, 0, 1)
            } else {
                w.puntos = w.puntos[:len(w.puntos)-1]
            }
        }
    }
    desde, _ := slices.BinarySearchFunc(w.puntos, p.h-horasVentana+1, compararH)
    hasta, _ := slices.BinarySearchFunc(w.puntos, p.h+1, compararH)
    return calcular(w.puntos[desde:hasta])
}
```

La ventana de 24 horas de una estación: una lectura por hora, 72 guardadas y la media de las de la ventana (`ventana/ventana.go`, líneas 143–163)
:::
