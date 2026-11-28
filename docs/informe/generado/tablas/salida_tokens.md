<!-- Generado por tablas.py desde docs/informe/capturas/tb2/seguridad/politicas.txt. No editar a mano. -->

::: {#lst-salida-tokens}
```text
## 7. Roles: el JWT decide qué rutas responde la API
GET /api/regimenes sin token ............................ 401 {"error":"token \
    ausente, inválido o expirado"}
GET /api/regimenes con token de usuario (invitado) ...... 200 []
GET /api/cluster/nodos con token de usuario .............. 403 {"error":"se requiere \
    el rol admin"}
GET /api/metricas con token de usuario ................... 403 {"error":"se requiere \
    el rol admin"}
POST /api/entrenamientos con token de usuario ............ 403 {"error":"se requiere \
    el rol admin"}
GET /api/cluster/nodos con token de admin ................ 200 \
    [{"nombre":"6621ecd33694-1","direccion":"172.20.0.6:6027

## 8. Tokens que no valen
token vencido (firmado bien, exp hace una hora) .......... 401 {"error":"token \
    ausente, inválido o expirado"}
token con alg=none ....................................... 401 {"error":"token \
    ausente, inválido o expirado"}
token de usuario con el rol cambiado a admin a mano ...... 401 {"error":"token \
    ausente, inválido o expirado"}

## 10. WebSocket /ws: origen, token en Sec-WebSocket-Protocol y rol
Mismo origen, token de usuario:
HTTP/1.1 101 Switching Protocols
Sec-WebSocket-Protocol: aire.jwt
Origen ajeno, token válido:
HTTP/1.1 403 Forbidden
{"error":"el origen https://otro-sitio.example no puede abrir este WebSocket"}
Mismo origen, sin token:
HTTP/1.1 401 Unauthorized
{"error":"token ausente, inválido o expirado"}
Mismo origen, token vencido:
HTTP/1.1 401 Unauthorized
{"error":"token ausente, inválido o expirado"}
```

Roles, tokens alterados y el WebSocket, probados con `curl` contra el Compose local por el Caddy de la interfaz (salida real, `docs/informe/capturas/tb2/seguridad/politicas.txt`)
:::
