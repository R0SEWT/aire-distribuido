<!-- Generado por tablas.py desde api/rutas.go. No editar a mano. -->

::: {#lst-predecir}
```go
media := s.ventanas.Agregar(l.estacion, l.fechaHora, l.pm25, l.pm10)
alerta, inca := ventana.Alerta(media), ventana.INCA(media)
ms := float64(time.Since(inicio).Nanoseconds()) / 1e6
s.escritor.encolar(almacen.Prediccion{
    TS:        enLima(s.ahora()).Truncate(time.Millisecond),
    Estacion:  l.estacion,
    FechaHora: enLima(ventana.Hora(l.fechaHora)), // una lectura por estación y hora
    PM25:      l.pm25,
    PM10:      l.pm10,
    Regimen:   reg.Regimen,
    ExcedeECA: reg.ExcedeECA,
    Media24h:  media,
    Alerta:    alerta,
    INCA:      inca,
    ModeloID:  m.ID,
    Ms:        ms,
})
```

El final del camino de una predicción: la ventana en memoria y la escritura encolada (`api/rutas.go`, líneas 236–252)
:::
