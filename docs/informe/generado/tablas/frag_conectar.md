<!-- Generado por tablas.py desde cluster/nodo.go. No editar a mano. -->

::: {#lst-conectar}
```go
// conectar reintenta cada espera hasta abrir la conexión o hasta que se cancela
// ctx. No llena la pantalla: avisa en el primer fallo y luego cada ~30 s.
func (n *Nodo) conectar(ctx context.Context, llamar func(context.Context) (net.Conn, error), espera time.Duration) (net.Conn, error) {
    var ultimoAviso time.Time
    for intento := 1; ; intento++ {
        conn, err := llamar(ctx)
        if err == nil {
            return conn, nil
        }
        if ctx.Err() != nil {
            return nil, ctx.Err()
        }
        if time.Since(ultimoAviso) >= 30*time.Second {
            n.Log.Printf("el coordinador no responde (intento %d): %v; sigo intentando", intento, err)
            ultimoAviso = time.Now()
        }
        if !dormir(ctx, espera) {
            return nil, ctx.Err()
        }
    }
}
```

El nodo llama hasta que el coordinador contesta: reintenta cada espera y avisa sin llenar la pantalla (`cluster/nodo.go`, líneas 111–131)
:::
