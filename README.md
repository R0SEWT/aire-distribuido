# aire-distribuido

Monitoreo de la calidad del aire en el Perú con las estaciones de OEFA: carga concurrente de millones
de lecturas horarias, K-Means paralelo en Go que agrupa las lecturas en regímenes de contaminación
por PM2.5/PM10 y meteorología, un clúster de nodos que lo distribuye, una API coordinadora y un
panel web de alertas tempranas de salud respiratoria.

Trabajo Final del curso Programación Concurrente y Distribuida (1ACC0065), UPC, 2026-20. Caso 10 del
enunciado: Monitoreo Calidad del Aire y Sensores Ambientales.

> Las 34 estaciones no cubren el territorio de forma pareja. Una zona sin estación no es una zona con
> aire limpio; el panel y el informe lo dicen explícitamente.

## Equipo

- Rody Vilchez
- Dayana Gómez
- Julio Meza

## Qué hay

| Carpeta | Qué es |
|---|---|
| `datos/`, `cmd/cargar` | Parte A: cargador concurrente del CSV largo de OEFA. Bloques por bytes, workers con goroutines y channels, pivot y estandarización por reducción. Escribe silver, gold y `reports/`. |
| `kmeans/`, `cmd/entrenar`, `cmd/benchmark` | Parte B: K-Means paralelo (worker pool, parciales por chunk, fusión exacta), regímenes con etiqueta frente al ECA, codo y benchmark de speedup. |
| `protocolo/`, `docs/diseno.md`, `docs/diagramas/` | Diseño del K-Means distribuido y el protocolo coordinador ↔ nodos (JSON por línea sobre TCP), con tests. El clúster se implementa en la PC4. |
| `configs/` | `limpieza.json` (columnas, parámetros, unidades, reglas) y `kmeans.json` (receta: features, k, semilla). |
| `notebooks/` | Laboratorio en Colab: criba de preguntas y EDA, calibración del K-Means, auditoría y figuras. |
| `docs/informe/` | Informe en Quarto renderizado a Word, con las cifras tomadas de `reports/`. |
| `docs/superpowers/specs/` | Specs de diseño de cada parte. |

## Cómo correrlo

```bash
go test -race ./...                                              # siempre con -race
go run ./cmd/cargar -entrada data/sample/oefa_muestra.csv        # silver, gold y reports/ de la muestra
go run ./cmd/entrenar -codo 2:10                                 # regímenes y codo
go run ./cmd/benchmark -workers 1,2,4,8 -chunk 1000,10000 -repes 5
docker build -t aire-distribuido . && docker run --rm aire-distribuido cargar -h
```

El despliegue con Docker Compose (API, nodos ML, MongoDB, Redis, frontend) llega con la PC4.

## Datos

Los datos no se versionan: el CSV de OEFA pesa 1.3 GB y vive en Google Drive en capas bronze, silver y
gold. Ver `data/README.md` para obtenerlo. `data/sample/` trae una muestra sintética con el mismo
formato para las pruebas y el CI.

## Flujo de trabajo

Git Flow: `develop` es la rama por defecto. Cada cambio va en una rama `feature/<tema>` con PR a
`develop`. Las entregas salen de `release/<pc3|pc4|tb2>` hacia `main`.
