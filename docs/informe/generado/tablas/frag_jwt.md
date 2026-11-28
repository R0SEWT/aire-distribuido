<!-- Generado por tablas.py desde api/jwt.go. No editar a mano. -->

::: {#lst-jwt}
```go
func (f *Firmador) Verificar(token string) (Reclamos, error) {
    partes := strings.Split(token, ".")
    if len(partes) != 3 {
        return Reclamos{}, errTokenMalo
    }
    var cab struct {
        Alg string `json:"alg"`
    }
    if err := decodificarParte(partes[0], &cab); err != nil || cab.Alg != "HS256" {
        // Solo HS256: un token con alg = none o con otro algoritmo no se acepta.
        return Reclamos{}, errTokenMalo
    }
    firma, err := b64.DecodeString(partes[2])
    if err != nil || !hmac.Equal(firma, f.firmar(partes[0]+"."+partes[1])) {
        return Reclamos{}, errTokenMalo
    }
    var r Reclamos
    if err := decodificarParte(partes[1], &r); err != nil || r.Usuario == "" || r.Expira == 0 {
        return Reclamos{}, errTokenMalo
    }
    if !f.ahora().Before(time.Unix(r.Expira, 0)) {
        return Reclamos{}, errTokenVencido
    }
    if r.Rol != RolUsuario && r.Rol != RolAdmin {
        return Reclamos{}, errTokenMalo
    }
    return r, nil
}
```

Verificación de un JWT HS256 con la biblioteca estándar (`api/jwt.go`, líneas 77–104)
:::
