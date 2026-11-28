<!-- Generado por tablas.py desde cluster/reunir.go. No editar a mano. -->

::: {#lst-aceptar}
```go
func (r *recepcion) aceptar() {
    defer close(r.aceptador)
    for {
        conn, err := r.ln.Accept()
        if err != nil {
            return
        }
        e := protocolo.NuevoEnlace(conn)
        // Con el mismo Mutex que cerrar: o el saludo queda anotado antes de que
        // cerrar los recorra, o no llega a nacer.
        r.mu.Lock()
        if r.cerrada {
            r.mu.Unlock()
            e.Cerrar()
            return
        }
        r.saludando[e] = struct{}{}
        r.saludos.Add(1)
        r.mu.Unlock()
        go r.saludar(e, conn.RemoteAddr().String())
    }
}
```

El bucle de Accept del coordinador: una goroutine por conexión que llega, como el handle de los laboratorios (`cluster/reunir.go`, líneas 152–173)
:::
