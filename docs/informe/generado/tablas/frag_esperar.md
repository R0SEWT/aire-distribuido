<!-- Generado por tablas.py desde protocolo/coordinador.go. No editar a mano. -->

::: {#lst-esperar}
```go
switch {
case m.Tipo == tipo && (tipo != Parcial || m.Iter == iter):
    return m, nil
case m.Tipo == Pong:
    continue

case m.Tipo == Parcial && m.Iter <= e.ultimaIter:
    e.Rezagados++
    continue
```

Lo que se descarta mientras se espera un PARCIAL: los PONG sueltos y los parciales rezagados (`protocolo/coordinador.go`, líneas 269–273 y 274–276)
:::
