<!-- Generado por tablas.py desde Dockerfile. No editar a mano. -->

::: {#lst-dockerfile}
```dockerfile
FROM golang:1.26 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go build ./... \
 && mkdir -p /out/bin /out/app/configs /out/app/reports /out/app/docs/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bin/ ./cmd/... \
 && cp -r configs/. /out/app/configs/ \
 && cp reports/kmeans.json reports/regimenes.json reports/estandarizacion.json reports/estaciones_ficha.csv \
       reports/centroides_iniciales.json /out/app/reports/ \
 && cp docs/api/openapi.yaml /out/app/docs/api/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/bin/ /usr/local/bin/
COPY --from=build /out/app/ /app/
WORKDIR /app
USER nonroot:nonroot

CMD ["cargar", "-h"]
```

La imagen de la API y de los nodos, en dos etapas (`Dockerfile`, completo, sin sus comentarios)
:::
