<!-- Generado por tablas.py desde api/servidor.go. No editar a mano. -->

::: {#lst-activar}
```go
// ActivarModelo reemplaza el modelo activo. Las predicciones en curso terminan
// con el anterior; ninguna toma un lock. Después lo guarda como activo en
// MongoDB y Redis, si la API los tiene (modelo_activo.go): eso puede tardar
// hasta plazoEscritura por almacén, así que no se llama desde un handler.
func (s *Servidor) ActivarModelo(m *modelo.Modelo) {
    s.modelo.Store(m)
    s.guardarModeloActivo(m)
}
```

El modelo activo se reemplaza entero, sin candado en el camino de la predicción (`api/servidor.go`, líneas 373–380)
:::
