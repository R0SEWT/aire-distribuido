<!-- Generado por tablas.py desde cluster/nodo.go. No editar a mano. -->

::: {#lst-atender}
```go
func (n *Nodo) Atender(conn net.Conn) error {
    cod, dec := protocolo.NuevoCodificador(conn), protocolo.NuevoDecodificador(conn)
    hola := protocolo.Mensaje{Tipo: protocolo.Hola, Version: protocolo.Version, Nodo: n.Nombre, Workers: n.Workers}
    if err := cod.Escribir(hola); err != nil {
        return fmt.Errorf("enviar HOLA: %w", err)
    }

    var part *particion
    pongs := 0
    for {
        m, err := dec.Leer()
        if err != nil {
            return fmt.Errorf("la conexión se cortó antes del FIN (%d PONG enviados): %w", pongs, err)
        }
        var resp protocolo.Mensaje
        switch m.Tipo {
        case protocolo.Ping:
            resp = protocolo.Mensaje{Tipo: protocolo.Pong}
            pongs++
        case protocolo.Cargar:
            // Un CARGAR nuevo reemplaza la partición anterior, también si falla.
            part = nil
            p, err := n.cargar(*m.Rango, m.Huella)
            if err != nil {
                resp = n.error("CARGAR [%d, %d): %v", m.Rango.Desde, m.Rango.Hasta, err)
                break
            }
            part = p
            resp = protocolo.Mensaje{Tipo: protocolo.Listo, N: p.datos.N, Huella: m.Huella}
        case protocolo.Iterar:
            resp = n.iterar(part, m)
        case protocolo.Fin:
            n.Log.Print(resumen(part, pongs))
            return nil // la partición se libera con la sesión
        case protocolo.Error:
            return fmt.Errorf("%w: %s", ErrCoordinador, m.Mensaje)
        default:
            return fmt.Errorf("%w: un nodo no espera %s", protocolo.ErrProtocolo, m.Tipo)
        }
        if err := cod.Escribir(resp); err != nil {
            return fmt.Errorf("enviar %s: %w", resp.Tipo, err)
        }
    }
}
```

El nodo sobre su conexión: manda HOLA y responde cada mensaje del coordinador hasta el FIN (`cluster/nodo.go`, líneas 140–183)
:::
