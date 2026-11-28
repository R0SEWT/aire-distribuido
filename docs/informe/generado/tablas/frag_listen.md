<!-- Generado por tablas.py desde cmd/api/main.go. No editar a mano. -->

::: {#lst-listen}
```go
var lnNodos net.Listener
dirNodos := ""
if op.nodosAddr != "" {
    if lnNodos, err = net.Listen("tcp", op.nodosAddr); err != nil {
        return fmt.Errorf("escuchar a los nodos: %w", err)
    }
    defer lnNodos.Close()
    dirNodos = lnNodos.Addr().String()
}
srv, cerrarAlmacenes, err := armar(op, getenv, log, lnNodos)
if err != nil {
    return err
}
defer cerrarAlmacenes() // al final: después del coordinador y del escritor
ln, err := net.Listen("tcp", op.addr)
```

Rol servidor: la API abre dos sockets en modo escucha, uno para los nodos y otro para HTTP (`cmd/api/main.go`, líneas 315–329)
:::
