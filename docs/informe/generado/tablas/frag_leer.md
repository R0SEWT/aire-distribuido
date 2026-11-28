<!-- Generado por tablas.py desde protocolo/codec.go. No editar a mano. -->

::: {#lst-leer}
```go
trozo, err := d.r.ReadSlice('\n')
d.pendiente = append(d.pendiente, trozo...)
if len(d.pendiente) > MaxLinea {
    d.err = fmt.Errorf("%w: línea de más de %d bytes", ErrMensajeInvalido, MaxLinea)
    return Mensaje{}, d.err
}
switch {
case err == nil:
    linea := d.pendiente
    d.pendiente = d.pendiente[:0]
    return decodificar(linea)
```

Recibir un mensaje: leer hasta el salto de línea y deserializar el JSON (`protocolo/codec.go`, líneas 169–179)
:::
