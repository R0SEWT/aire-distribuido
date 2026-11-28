<!-- Generado por tablas.py desde cmd/nodo/main.go. No editar a mano. -->

::: {#lst-dial}
```go
marcador := &net.Dialer{Timeout: 3 * time.Second}
llamar := func(ctx context.Context) (net.Conn, error) {
    return marcador.DialContext(ctx, "tcp", *coordinador)
}
```

Rol cliente: el nodo llama al coordinador con net.Dialer, en la dirección IP:puerto que recibe por parámetro (`cmd/nodo/main.go`, líneas 72–75)
:::
