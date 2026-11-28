<!-- Generado por tablas.py desde web/Caddyfile. No editar a mano. -->

::: {#lst-caddy}
```text
{
    admin off

    log default {
        format filter {
            wrap console
            fields {
                request>headers>Sec-Websocket-Protocol delete
                request>uri query {
                    replace token REDACTED
                }
            }
        }
    }
}

(politica_ui) {
    header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"
}

{$SITIO::80} {
    encode zstd gzip

    header {
        X-Content-Type-Options nosniff
        Referrer-Policy no-referrer
        -Server
    }

    @tls protocol https
    header @tls Strict-Transport-Security "max-age=31536000"

    @api path /api/* /ws /openapi.yaml
    handle @api {
        reverse_proxy api:8080
    }

    handle /docs {
        header Content-Security-Policy "default-src 'none'; script-src https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/ 'sha256-Bh/L1I8Jz3e+G6+99nT+g0tTChazroLfSG0Sz+e0ZVc='; style-src https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/ 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
        reverse_proxy api:8080
    }

    @archivos path /assets/* /datos/*
    handle @archivos {
        root * /srv
        import politica_ui

        @compilado {
            path /assets/*
            file
            not path */
        }
        header @compilado Cache-Control "public, max-age=31536000, immutable"

        @datos path /datos/*
        header @datos Cache-Control "no-cache"

        file_server
    }

    handle {
        root * /srv
        import politica_ui
        header Cache-Control "no-cache"
        try_files {path} /index.html
        file_server
    }
}
```

El Caddyfile: un solo servidor web para la interfaz y la API, con las cabeceras de seguridad (`web/Caddyfile`, completo, sin sus comentarios)
:::
