<!-- Generado por tablas.py desde docs/informe/salidas-tb2/seguridad.txt. No editar a mano. -->

::: {#lst-salida-politicas}
```text
$ curl -si -X OPTIONS https://aire.rosewt.dev/api/predicciones -H 'Origin: \
    https://otro-sitio.example' -H 'Access-Control-Request-Method: POST'
HTTP/2 403
vary: Origin
{"error":"CORS: el origen https://otro-sitio.example no está permitido"}

$ curl -si https://aire.rosewt.dev/api/metricas            # sin token
HTTP/2 401
www-authenticate: Bearer realm="aire-distribuido"
{"error":"token ausente, inválido o expirado"}

$ curl -s -X POST https://aire.rosewt.dev/api/auth/invitado          # ingreso de \
    invitado
{"token": "eyJhbGciOiJI…(tapado)", "rol": "usuario", "expira": \
    "2026-11-27T19:34:28-05:00"}

$ curl -si https://aire.rosewt.dev/api/metricas -H "Authorization: Bearer \
    $TOKEN_INVITADO"
HTTP/2 403
{"error":"se requiere el rol admin"}
```

Las políticas de la API probadas contra el despliegue público: un origen ajeno, sin token, el invitado y su rol (salida real, `docs/informe/salidas-tb2/seguridad.txt`)
:::
