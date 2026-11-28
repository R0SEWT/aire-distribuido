<!-- Generado por tablas.py desde cmd/api/main.go. No editar a mano. -->

::: {#lst-http}
```go
hs := &http.Server{
    Handler:           srv,
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       10 * time.Second,
    WriteTimeout:      30 * time.Second,
    IdleTimeout:       2 * time.Minute,
    ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
}
sirviendo := make(chan error, 1)
go func() { sirviendo <- hs.Serve(ln) }()
```

El Web Server HTTP de la API: plazos de lectura y escritura y Serve sobre el socket que ya escucha (`cmd/api/main.go`, líneas 345–354)
:::
