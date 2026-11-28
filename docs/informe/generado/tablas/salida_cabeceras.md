<!-- Generado por tablas.py desde docs/informe/salidas-tb2/seguridad.txt. No editar a mano. -->

::: {#lst-salida-cabeceras}
```text
$ curl -sI https://aire.rosewt.dev/
HTTP/2 200
accept-ranges: bytes
cache-control: no-cache
content-security-policy: default-src 'self'; script-src 'self'; style-src 'self'; \
    font-src 'self'; img-src 'self'; connect-src 'self'; object-src 'none'; base-uri \
    'self'; form-action 'self'; frame-ancestors 'none'
content-type: text/html; charset=utf-8
referrer-policy: no-referrer
strict-transport-security: max-age=31536000
vary: Accept-Encoding
x-content-type-options: nosniff

$ curl -sI https://aire.rosewt.dev/docs
HTTP/2 200
content-security-policy: default-src 'none'; script-src \
    https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/ \
    'sha256-Bh/L1I8Jz3e+G6+99nT+g0tTChazroLfSG0Sz+e0ZVc='; style-src \
    https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/ 'unsafe-inline'; img-src \
    'self' data:; font-src 'self' data:; connect-src 'self'; base-uri 'none'; \
    form-action 'none'; frame-ancestors 'none'
referrer-policy: no-referrer
strict-transport-security: max-age=31536000
x-content-type-options: nosniff
```

Las cabeceras que pone Caddy en el despliegue público: la UI y Swagger UI (salida real, `docs/informe/salidas-tb2/seguridad.txt`)
:::
