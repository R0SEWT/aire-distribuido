<!-- Generado por tablas.py desde web/Dockerfile. No editar a mano. -->

::: {#lst-dockerfile-web}
```dockerfile
FROM node:22-alpine AS build
WORKDIR /web

RUN corepack enable

COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY . .
RUN pnpm build

FROM caddy:2.10-alpine
COPY Caddyfile /etc/caddy/Caddyfile
COPY --from=build /web/dist /srv
EXPOSE 80
```

La imagen del servicio web: Node compila la interfaz y Caddy la sirve (`web/Dockerfile`, completo, sin sus comentarios)
:::
