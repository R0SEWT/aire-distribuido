<!-- Generado por tablas.py desde api/servidor.go. No editar a mano. -->

::: {#lst-exigir}
```go
// ruta registra un handler medido. roles nil es una ruta libre; si no, hace
// falta un JWT válido con alguno de esos roles.
func (s *Servidor) ruta(patron string, roles []string, h http.HandlerFunc) {
    var sig http.Handler = h
    if roles != nil {
        sig = s.exigir(roles, h)
    }
    s.mux.Handle(patron, s.metricas.Medir(patron, sig))
}

func (s *Servidor) exigir(roles []string, sig http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        tok, ok := tokenBearer(r)
        if !ok {
            noAutenticado(w)
            return
        }
        rec, err := s.firmador.Verificar(tok)
        if err != nil {
            noAutenticado(w)
            return
        }
        if !slices.Contains(roles, rec.Rol) {
            escribirError(w, http.StatusForbidden, "se requiere el rol "+strings.Join(roles, " o "))
            return
        }
        sig.ServeHTTP(w, r)
    })
}
```

Cada ruta se envuelve en dos capas: la que mide su latencia y la que exige un JWT con el rol (`api/servidor.go`, líneas 393–401 y 405–423)
:::
