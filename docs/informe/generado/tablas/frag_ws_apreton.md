<!-- Generado por tablas.py desde api/hub.go. No editar a mano. -->

::: {#lst-ws-apreton}
```go
clave, msg := validarApreton(r)
if msg != "" {
    w.Header().Set("Sec-WebSocket-Version", "13") // la que este servidor habla (RFC 6455, §4.4)
    escribirError(w, http.StatusBadRequest, msg)
    return
}
if !s.cors.permiteWebSocket(r) {
    escribirError(w, http.StatusForbidden, "el origen "+recortar(r.Header.Get("Origin"), 128)+" no puede abrir este WebSocket")
    return
}
tok, subprotocolo, ok := tokenWS(r)
if !ok {
    noAutenticado(w)
    return
}
rec, err := s.firmador.Verificar(tok)
if err != nil {
    noAutenticado(w)
    return
}
if !slices.Contains(rolesWS, rec.Rol) {
    escribirError(w, http.StatusForbidden, "se requiere el rol "+strings.Join(rolesWS, " o "))
    return
}
if s.apagando.Load() || !s.hub.admite() {
    escribirError(w, http.StatusServiceUnavailable, errApagando.Error())
    return
}
conn, lector, err := tomarConexion(w)
if err != nil {
    s.log.Error("websocket: no se pudo tomar la conexión", "err", err)
    return
}
s.hub.Atender(conn, lector, rec.Rol, time.Unix(rec.Expira, 0), respuesta101(clave, subprotocolo))
```

El apretón de manos del WebSocket: el origen, el token y el rol se validan antes de responder 101 (`api/hub.go`, líneas 472–505)
:::
