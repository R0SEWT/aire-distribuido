<!-- Generado por tablas.py desde protocolo/codec.go. No editar a mano. -->

::: {#lst-escribir}
```go
func (c *Codificador) escribir(m Mensaje) error {
    if c.err != nil {
        return c.err
    }
    if err := m.Validar(); err != nil {
        return err // no se escribió nada: el flujo sigue sano
    }
    linea, err := json.Marshal(m)
    if err != nil {
        return fmt.Errorf("codificar %s: %w", m.Tipo, err)
    }
    if len(linea)+1 > MaxLinea {
        return fmt.Errorf("%w: %s ocupa %d bytes, el máximo es %d", ErrMensajeInvalido, m.Tipo, len(linea)+1, MaxLinea)
    }
    linea = append(linea, '\n')
    if _, err := c.w.Write(linea); err != nil {
        c.err = traducir("escribir "+string(m.Tipo), err)
        return c.err
    }
    if err := c.w.Flush(); err != nil {
        c.err = traducir("escribir "+string(m.Tipo), err)
        return c.err
    }
    return nil
}
```

Serializar y enviar un mensaje: JSON en una línea, con bufio, y vaciar el búfer enseguida (`protocolo/codec.go`, líneas 87–111)
:::
