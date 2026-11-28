<!-- Generado por tablas.py desde api/rutas.go. No editar a mano. -->

::: {#lst-invitado}
```go
// POST /api/auth/invitado
//
// El ingreso de invitado: un token de la cuenta que designó quien levantó la
// API (Config.Invitado), sin presentar credenciales. El pedido no lleva cuerpo
// y, si trae uno, no se lee. La cuenta es un usuario más, de rol usuario: su
// clave no sale del servidor y el token es igual al que daría
// POST /api/auth/token. Sin cuenta de invitado responde 404.
func (s *Servidor) ingresarInvitado(w http.ResponseWriter, r *http.Request) {
    if s.invitado == "" {
        escribirError(w, http.StatusNotFound, msgSinInvitado)
        return
    }
    s.responderToken(w, r, s.invitado, s.usuarios[s.invitado].Rol)
}
```

El ingreso de invitado: un token de la cuenta que designó el servidor, sin que las credenciales pasen por el navegador (`api/rutas.go`, líneas 46–59)
:::
