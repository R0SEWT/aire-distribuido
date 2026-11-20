# syntax=docker/dockerfile:1

# Imagen del sistema: los comandos de la PC3 (cargar, entrenar, benchmark) y los
# de la PC4 (api, nodo, distribuido, smoke), compilados estáticos sobre una
# imagen final sin shell ni gestor de paquetes. docker-compose.yml la usa para
# los servicios api y nodo; MongoDB y Redis son sus imágenes oficiales.
#
# Construir y usar suelta:
#
#   docker build -t aire-distribuido .
#   docker run --rm aire-distribuido cargar -h
#   docker run --rm -v "$PWD/data:/datos" aire-distribuido cargar   <flags de cargar -h>
#   docker run --rm -v "$PWD/data:/datos" aire-distribuido entrenar <flags de entrenar -h>
#   docker run --rm -p 8080:8080 -e JWT_SECRETO=$(openssl rand -hex 32) aire-distribuido api -demo
#
# Qué va dentro, en /app, además de los ejecutables:
#
#   - configs/: la receta del K-Means y las reglas de limpieza. La imagen es el
#     código más la receta congelada; para probar otra sin reconstruir,
#     -v "$PWD/configs:/app/configs:ro".
#   - reports/kmeans.json, regimenes.json, estandarizacion.json y
#     estaciones_ficha.csv, y docs/api/openapi.yaml: lo que la API lee al
#     arrancar (el modelo que siembra, el catálogo de estaciones y el contrato
#     que sirve). .dockerignore deja pasar solo esos archivos de reports/ y docs/.
#
# Los datos no van dentro: data/ se monta (el gold, de solo lectura, en los
# nodos) y las salidas quedan en el disco del anfitrión.

FROM golang:1.26 AS build
WORKDIR /src

# Las dependencias primero, en su propia capa: solo se vuelven a descargar si
# cambian go.mod o go.sum. Son los drivers de MongoDB y Redis, que solo importan
# almacen/mongo y almacen/redis; el resto es biblioteca estándar.
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# 1. go build ./... compila todo el módulo: si algo no compila, la imagen no se
#    construye.
# 2. Los ejecutables salen de ./cmd/... CGO_ENABLED=0 da binarios estáticos que
#    corren sin libc; -trimpath saca las rutas de la máquina que compiló.
# 3. Lo que la API lee al arrancar se junta en /out/app con las mismas rutas
#    relativas que en el repositorio, que son las que cmd/api trae por omisión.
RUN go build ./... \
 && mkdir -p /out/bin /out/app/configs /out/app/reports /out/app/docs/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bin/ ./cmd/... \
 && cp -r configs/. /out/app/configs/ \
 && cp reports/kmeans.json reports/regimenes.json reports/estandarizacion.json reports/estaciones_ficha.csv /out/app/reports/ \
 && cp docs/api/openapi.yaml /out/app/docs/api/

# distroless/static: sin shell, con certificados, zona horaria (tzdata, para
# America/Lima) y un usuario sin privilegios. Unos pocos MB.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/bin/ /usr/local/bin/
COPY --from=build /out/app/ /app/
WORKDIR /app
USER nonroot:nonroot

# Sin ENTRYPOINT: la imagen trae varios comandos y se elige uno al correrla.
CMD ["cargar", "-h"]
