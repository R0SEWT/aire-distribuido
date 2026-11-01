# syntax=docker/dockerfile:1

# Imagen de la PC3: los comandos del cargador concurrente (parte A) y del
# K-Means paralelo (parte B), compilados estáticos sobre una imagen final sin
# shell ni gestor de paquetes. Docker Compose completo (API coordinadora, nodos
# de cómputo, MongoDB, Redis y frontend) llega con la PC4; el diagrama de qué va
# en cada contenedor está en docs/diagramas/arquitectura.png.
#
# Construir:
#
#   docker build -t aire-distribuido .
#
# Usar, una vez fusionadas en develop las ramas que crean cmd/cargar y
# cmd/entrenar. Los datos no van dentro de la imagen: data/ se monta en /datos,
# y las salidas (silver, gold, reports) quedan en el disco del anfitrión.
#
#   docker run --rm aire-distribuido cargar -h
#   docker run --rm -v "$PWD/data:/datos" aire-distribuido cargar   <flags de cargar -h>
#   docker run --rm -v "$PWD/data:/datos" aire-distribuido entrenar <flags de entrenar -h>
#
# La receta del K-Means y las reglas de limpieza (configs/*.json) sí van dentro,
# en /app/configs: la imagen es el código más la receta congelada. Para probar
# otra receta sin reconstruir: -v "$PWD/configs:/app/configs:ro".

FROM golang:1.26 AS build
WORKDIR /src

# Solo biblioteca estándar: no hay go.sum ni dependencias que descargar.
COPY go.mod ./
COPY . .

# 1. go build ./... compila todo el módulo: si algo no compila, la imagen no se
#    construye.
# 2. Los ejecutables salen de ./cmd/... (cargar, entrenar, benchmark).
#    CGO_ENABLED=0 da binarios estáticos que corren sin libc; -trimpath saca
#    las rutas de la máquina que compiló. Mientras cmd/ no exista en la rama,
#    la imagen se construye igual, sin ejecutables.
# 3. configs/ se copia si existe, por la misma razón.
RUN go build ./... \
 && mkdir -p /out/bin /out/configs \
 && if [ -d cmd ]; then CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bin/ ./cmd/...; fi \
 && if [ -d configs ]; then cp -r configs/. /out/configs/; fi

# distroless/static: sin shell, con certificados, zona horaria (tzdata, para
# America/Lima) y un usuario sin privilegios. Unos pocos MB.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/bin/ /usr/local/bin/
COPY --from=build /out/configs/ /app/configs/
WORKDIR /app
USER nonroot:nonroot

# Sin ENTRYPOINT: la imagen trae varios comandos y se elige uno al correrla.
CMD ["cargar", "-h"]
