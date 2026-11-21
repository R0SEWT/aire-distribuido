# syntax=docker/dockerfile:1

# La imagen del servicio web (spec del frontend, §11): la UI compilada y el
# Caddy que la sirve y pasa /api, /ws, /docs y /openapi.yaml a la API.
# docker-compose.yml la construye con esta carpeta (web/) como contexto.
#
# Dos etapas: Node compila y no llega a la imagen final, que es Caddy con los
# archivos estáticos. Los tipos de la API (src/api/contrato.d.ts) se versionan,
# así que compilar no necesita el contrato ni nada de fuera de web/.

FROM node:22-alpine AS build
WORKDIR /web

# pnpm, en la versión que fija package.json (packageManager).
RUN corepack enable

# Las dependencias primero, en su propia capa: solo se vuelven a instalar si
# cambia el lockfile.
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY . .
RUN pnpm build

FROM caddy:2.10-alpine
COPY Caddyfile /etc/caddy/Caddyfile
COPY --from=build /web/dist /srv
EXPOSE 80
