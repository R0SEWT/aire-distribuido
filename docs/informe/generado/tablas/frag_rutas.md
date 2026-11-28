<!-- Generado por tablas.py desde api/servidor.go. No editar a mano. -->

::: {#lst-rutas}
```go
cualquiera := []string{RolUsuario, RolAdmin}
s.ruta("POST /api/auth/token", nil, s.crearToken)
s.ruta("POST /api/auth/invitado", nil, s.ingresarInvitado)
s.ruta("GET /api/salud", nil, s.salud)
s.ruta("POST /api/predicciones", cualquiera, s.predecir)
s.ruta("GET /api/estaciones", cualquiera, s.listarEstaciones)
s.ruta("GET /api/estaciones/{id}/regimen", cualquiera, s.regimenEstacion)
s.ruta("GET /api/regimenes", cualquiera, s.listarRegimenes)
s.ruta("GET /api/modelos/activo", cualquiera, s.modeloActivo)
s.ruta("GET /api/metricas", []string{RolAdmin}, s.consultarMetricas)
s.ruta("GET /api/cluster/nodos", []string{RolAdmin}, s.listarNodos)
s.ruta("GET /api/cluster/eventos", []string{RolAdmin}, s.listarEventosCluster)
s.ruta("POST /api/entrenamientos", []string{RolAdmin}, s.iniciarEntrenamiento)
s.ruta("GET /api/entrenamientos", []string{RolAdmin}, s.listarEntrenamientos)
s.ruta("GET /api/entrenamientos/{id}", []string{RolAdmin}, s.consultarEntrenamiento)
s.ruta("GET /ws", nil, s.seguirEventos) // se autentica en el handler: el navegador no manda Authorization
s.ruta("GET /openapi.yaml", nil, s.servirOpenAPI)
s.ruta("GET /docs", nil, s.servirDocs)
```

Las rutas de la API, cada una con los roles que la pueden usar (nil: libre) (`api/servidor.go`, líneas 203–220)
:::
