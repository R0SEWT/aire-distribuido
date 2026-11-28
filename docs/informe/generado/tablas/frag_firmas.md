<!-- Generado por tablas.py desde kmeans/parciales.go. No editar a mano. -->

::: {#lst-firmas}
```go
// Parciales asigna las filas [desde, hasta) de d a los centroides y devuelve
// sumas k*D, conteos k e inercia. Es lo que un nodo calcula sobre su partición
// en cada iteración. Los centroides son de solo lectura: varias goroutines
// pueden llamar a Parciales a la vez sobre tramos distintos.
//
// k sale de len(centroides)/d.D. Un tramo fuera de [0, N] o centroides que no
// son múltiplo de D son un error de programación y entran en pánico.
func Parciales(d *Datos, centroides []float64, desde, hasta int) Parcial {

// Actualizar recalcula los centroides a partir de un parcial fusionado y dice
// cuánto se movieron: el mayor desplazamiento de una coordenada,
// max_j ||mu_j(t) - mu_j(t-1)||inf, que es la norma del criterio de parada del
// TP. Un cluster vacío conserva su centroide anterior: re-sembrarlo con el punto
// más lejano haría que el resultado dependiera del orden de recorrido.
//
// Cada centroide se calcula como suma * (1/conteo), igual que en el TP: dividir
// en vez de multiplicar por el inverso cambiaría el último bit.
func Actualizar(cent []float64, p Parcial) (movimiento float64) {
```

Las dos funciones que la versión distribuida reutiliza tal cual: Parciales y Actualizar (`kmeans/parciales.go`, líneas 19–26 y 136–144)
:::
