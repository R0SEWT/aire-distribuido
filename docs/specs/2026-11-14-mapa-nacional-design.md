# Mapa nacional de PM2.5 y PM10 — Diseño (experimento de laboratorio para el TB2)

> Borrador del 2026-11-14. **[decisión mía]** es una elección del redactor que se puede revertir; **[abierta]**
> está pendiente; **[estimación]** es una cuenta sin medir. Lo demás sale de datos ya vistos: el silver de OEFA
> (`data/silver/oefa_ancho.csv`), los catálogos de Copernicus consultados el 14/11 y las mediciones del
> laboratorio (PR #48, #49, #50).

## 1. Intención

Las 34 estaciones de OEFA (22 con PM) no cubren el país, y una zona sin estación no es una zona con aire limpio
(el ángulo ético del proyecto). Este experimento estima **PM2.5 y PM10 en cada celda de 0.1° (~11 km) del Perú,
cada hora, de 2018 a 2022**, con meteorología de reanálisis y aerosoles de un modelo físico como entrada y las
estaciones como verdad. Cruzado con la población, responde la pregunta de impacto a escala nacional:

> **¿Cuántas personas respiraron aire sobre el ECA, cuántos días al año y dónde, incluso donde no hay estación?**

Es también la carga que justifica el clúster: unos 32 GB de entrada, una red convolucional de ~8 M de parámetros
y el entrenamiento repartido en las GPUs del laboratorio (T1000 u RTX A1000, 8 GB).

**Lugar en el proyecto.** El sistema del curso sigue en Go. El entrenamiento y la inferencia en GPU son PyTorch
en `lab/mapa/`, como `lab/gpu/` (PR #49): un laboratorio que produce datos (mapas, totales por distrito) que el
sistema en Go sirve desde MongoDB y la SPA muestra. El **coordinador del entrenamiento distribuido es Go**
(`cmd/mapa-coord`, sobre `protocolo/` v2): reparte, sincroniza y promedia; los nodos en Python lo llaman como
`lab/gpu/nodo_gpu.py` llama al de la PC4.

## 2. Metas y no-metas

**Metas**

- Datos de Copernicus y estáticos descargados y llevados a una grilla común, en fragmentos que un nodo baja por HTTP.
- Tres modelos comparados en las mismas particiones: CAMS crudo, un modelo tabular por píxel (XGBoost en GPU) y la
  red espacial. Gana el que mejor estima en **estaciones que no vio**.
- Entrenamiento distribuido con **local SGD** en N GPUs del laboratorio con el coordinador en Go, y su medición:
  tiempo por época, costo de sincronizar, convergencia según cada cuántos pasos se promedia.
- Inferencia de 2018–2022 completo (~1 100 millones de celda-horas) repartida en las GPUs.
- Productos: PM2.5 y PM10 diarios por celda, días sobre el ECA por celda y año, personas-día sobre el ECA por
  distrito, y la incertidumbre de cada uno.

**No-metas**

- Pronóstico (t+24 h): se puede agregar con los pronósticos de CAMS como entrada; no en esta versión.
- 2023 en adelante: OEFA no tiene PM después de 2022, y sin verdad no hay validación.
- Alta resolución real de PM: la red reparte información de 22 puntos y de CAMS a 0.75°. El mapa dice esa
  resolución efectiva (§7).

## 3. Datos

### 3.1 Verdad: estaciones

| Fuente | Qué hay | Uso |
|---|---|---|
| OEFA, silver (`data/silver/oefa_ancho.csv`) | 22 estaciones con PM, 2018-01 → 2022-12; PM10 ~126 mil horas, PM2.5 ~66 mil (solo desde 2020). Hora con convención `fin` (`reports/limpieza.json`), en −05:00 | Etiquetas |
| SENAMHI Lima (datosabiertos.gob.pe, 577 794 registros) | Red automática de Lima, PM2.5 y PM10 horarios | **[abierta]** Etiquetas extra donde vive un tercio del país. El portal no deja bajarlo por terminal: Rody lo baja desde el navegador a `data/raw/senamhi/` |

Coordenadas: `reports/estaciones_ficha.csv` (lat, lon). Todo se pasa a **UTC** y a la hora de fin del intervalo.

### 3.2 Entradas

| Fuente (licencia CC-BY-4.0) | Variables | Resolución | Tamaño 2018–2022 [estimación] |
|---|---|---|---|
| ERA5-Land, CDS `reanalysis-era5-land` | temperatura y punto de rocío a 2 m, viento u/v a 10 m, presión en superficie, precipitación, radiación solar | 0.1°, horaria | ~25–30 GB |
| ERA5, CDS `reanalysis-era5-single-levels` | altura de la capa límite, nubosidad total | 0.25°, horaria | ~1 GB |
| CAMS EAC4, ADS `cams-global-reanalysis-eac4` | PM2.5, PM10, AOD total a 550 nm, AOD de polvo a 550 nm | 0.75°, cada 3 h | < 1 GB |
| Invariantes de ERA5-Land | orografía, máscara de tierra | 0.1° | — |
| WorldPop 2020, 1 km (CC-BY-4.0) | población | 1 km → suma a 0.1° | ~100 MB |
| geoBoundaries PER ADM3 **[abierta: licencia del ADM3]** | límites distritales | vectorial | ~50 MB |

CAMS y ERA5 se interpolan **bilinealmente** a la grilla de 0.1°; CAMS además **linealmente en el tiempo** de 3 h a
1 h. La precipitación y la radiación de ERA5-Land vienen acumuladas desde las 00 UTC: se pasan a valores horarios.

**Cuentas.** CDS y ADS usan la misma cuenta gratuita de ECMWF; hay que aceptar la licencia de cada dataset en su
página. Las credenciales van en `~/.cdsapirc` (CDS) y `~/.adsapirc` (ADS, con `CDSAPI_RC=~/.adsapirc`), nunca en
el repo. Las descargas pasan por la cola de Copernicus: horas.

### 3.3 Grilla común

**192 × 136 celdas de 0.1°**: latitud `0.6 − 0.1·i` (i = 0…191, de 0.6° a −18.5°), longitud `−81.6 + 0.1·j`
(j = 0…135, de −81.6° a −68.1°). Cubre el Perú con margen y es divisible por 8 (tres reducciones de la red). Los
puntos coinciden con los de ERA5-Land. La máscara del Perú sale de geoBoundaries ADM0.

### 3.4 Formato en disco (el contrato entre `datos/` y `modelo/`)

En `data/mapa/` (fuera de git; en Drive bajo `aire-distribuido/data/mapa/` con `rclone`, remoto `gdrive:`):

- `dias/AAAA-MM-DD.npy`: `float16`, forma `[24, C, 192, 136]`, horas 00–23 UTC de ese día, canales ya
  **normalizados** (z-score con la media y la desviación de 2018–2021). 1 826 días, ~17.5 MB cada uno.
- `estaticos.npy`: `float32`, `[S, 192, 136]`: orografía, máscara de tierra, máscara del Perú, latitud y longitud
  normalizadas, `log1p` de la población.
- `etiquetas.csv`: `hora_utc` (ISO, la hora de fin), `estacion`, `fuente`, `i`, `j`, `pm2_5`, `pm10` (µg/m³; vacío
  si falta). Una fila por estación-hora con al menos un valor.
- `manifiesto.json`: canales dinámicos en orden con su media y desviación, estáticos en orden, grilla, periodo,
  `sha256` de cada archivo, fuentes y versión. Un nodo verifica cada día que baja contra este archivo.

El nodo solo necesita **numpy y torch** (lo que ya trae el paquete portable de `lab/gpu/armar-paquete.sh`) y la
biblioteca estándar de Python: nada de xarray ni netCDF en los PC del laboratorio.

## 4. Modelos

Entrada de una muestra para la hora t: las 24 horas que terminan en t (`24·C` canales) + los `S` estáticos + 4 de
calendario (seno y coseno de la hora y del día del año). Salida: `log1p` de PM2.5 y de PM10 en las 192 × 136
celdas. **Pérdida**: error cuadrático en espacio `log1p`, solo en las celdas con estación y valor en t, por
contaminante; las celdas sin verdad no aportan.

| Modelo | Qué es | Para qué |
|---|---|---|
| M0 CAMS crudo | PM2.5 y PM10 de CAMS interpolados a la celda | La línea base: ¿cuánto se equivoca el modelo físico en el Perú? |
| M1 Tabular | XGBoost (GPU) por celda-hora con los mismos canales en t, medias de 6 y 24 h y los estáticos | El método habitual en la literatura de *downscaling* de PM; si la red no le gana, se dice |
| M2 Red espacial | U-Net 2D de 3 niveles (32-64-128-256 canales), GroupNorm (no BatchNorm: el promedio entre nodos no tiene estadísticas que sincronizar), ~8 M de parámetros | Usa el contexto espacial y temporal; es la carga del clúster |

**Validación**: **5 particiones agrupadas por estación** (ninguna estación aparece en entrenamiento y validación a
la vez), las mismas para los tres modelos. Métricas en µg/m³ y en `log1p`: RMSE, sesgo, R², y la detección de
días sobre el ECA de 24 h (50 µg/m³ PM2.5, 100 µg/m³ PM10): sensibilidad y precisión. Se reportan por estación:
con 22 estaciones, el promedio esconde mucho.

## 5. Entrenamiento distribuido

### 5.1 Por qué local SGD

Medido el 14/11 (`mlpbench`, PR futura): sincronizar el gradiente en cada lote por 1 GbE cuesta ~0.25 s por paso
con 20 nodos y un modelo de 34 mil parámetros en JSON, y un PC entrena solo más rápido. Con ~8 M de parámetros es
peor. Entonces **cada nodo entrena H pasos con su parte de los datos y el coordinador promedia los pesos**
(local SGD / FedAvg). Con 8 M de parámetros en `float32` son 32 MB por nodo; 20 nodos suben y bajan ~1.3 GB por
sincronización, ~12 s por el cable de la laptop **[estimación]**; con una sincronización cada ~5 min de cómputo,
~4 % de sobrecosto. Se mide con H = 50, 200 y 1 000.

### 5.2 Reparto de los datos

Los días se agrupan en **bloques de 7 días** que se barajan con la semilla y se reparten en ronda entre los nodos,
para que cada nodo vea todas las estaciones del año (con bloques contiguos por nodo, cada uno vería otra estación
del año y el promedio se desviaría). Un nodo baja sus días y el día anterior a cada bloque (la ventana de 24 h)
desde el servidor HTTP de la laptop (`lab/gpu/servidor.py`), los verifica con el manifiesto y los deja en RAM o en
la GPU si caben (con 20 nodos son ~1.7 GB por nodo).

### 5.3 Protocolo v2

Extiende la v1 sin romperla: los mensajes de control siguen siendo **JSON en una línea**; un mensaje con carga
binaria lleva en el JSON el campo `"bytes": N` y **le siguen exactamente N bytes** crudos después del `'\n'`. Un
mensaje sin `bytes` es uno de la v1. Los vectores son `float32` *little-endian*.

| Mensaje | Dirección | Campos | Carga binaria |
|---|---|---|---|
| `HOLA` | nodo → coord | `version: 2`, `nodo`, `workers`, `gpu`, `vram_gb` | — |
| `TAREA` | coord → nodo | `tarea` (id), `manifiesto` (URL), `huella` (sha256 del manifiesto), `dias` (lista de `AAAA-MM-DD`), `validacion` (estaciones excluidas de la pérdida), `hiper` (lote, tasa de aprendizaje, semilla…) | — |
| `LISTO` | nodo → coord | `tarea`, `muestras` (horas con etiqueta que tiene) | — |
| `PESOS` | coord → nodo | `ronda`, `pasos` (H), `n` (parámetros), `bytes` | `n` × `float32`: pesos globales |
| `LOCAL` | nodo → coord | `ronda`, `n`, `bytes`, `muestras` (vistas en la ronda), `perdida`, `perdida_val`, `ms_computo` | `n` × `float32`: pesos tras H pasos |
| `FIN`, `PING`, `PONG`, `ERROR` | | como en la v1 | — |

El coordinador **trata los pesos como un vector opaco**: no sabe de capas. El orden y las formas los fija
`modelo/` en `pesos.json` junto al vector inicial `pesos_0.bin`. El promedio es **ponderado por `muestras`**, se
acumula en `float64` **en orden de nodo** (determinista, como la fusión del K-Means) y se baja a `float32`. El
optimizador (AdamW) vive en cada nodo y conserva su estado entre rondas; solo viajan los pesos.

`MaxLinea` (1 MB) sigue rigiendo la línea JSON; la carga binaria tiene su propio tope, `MaxCarga` = 256 MB.

### 5.4 `cmd/mapa-coord`

```
mapa-coord -escuchar 10.11.x.y:7071 -nodos N -manifiesto http://…/manifiesto.json \
           -pesos-iniciales pesos_0.bin -rondas R -pasos H -hiper hiper.json -validacion CA-SB-01,… \
           -salida reports/mapa_entrenamiento.json -pesos-finales pesos_final.bin
```

Espera N nodos (los que llaman, como `cmd/distribuido`), reparte los bloques, corre R rondas `PESOS → LOCAL`
con plazo, promedia, y escribe por ronda: pérdida de entrenamiento y de validación (promedio ponderado),
ms de cómputo por nodo, ms de transferencia y ms de promedio. Si un nodo cae, la ronda aborta y lo nombra; los
pesos de la última ronda completa quedan escritos (punto de control).

### 5.5 Modo granja

Las 5 particiones de validación, las semillas y los hiperparámetros son trabajos independientes: con 20 GPUs, 5
trabajos de 4 nodos a la vez. **[abierta]** En esta versión, un coordinador por trabajo en puertos distintos; el
reparto de nodos entre trabajos desde un solo coordinador queda para después.

## 6. Inferencia nacional

Con los pesos finales (y los de las 5 particiones, para la incertidumbre), cada nodo predice sus días completos:
192 × 136 × 24 horas por día. Devuelve por día los promedios diarios por celda (`float16`, ~100 KB por día y
contaminante) por HTTP (`PUT` a `resultados/`). La laptop arma:

- `mapa/pm25_diario.npy`, `mapa/pm10_diario.npy` (`[1826, 192, 136]`), con la desviación entre las 5 particiones.
- Días sobre el ECA por celda y año.
- Por distrito (ADM3): población, días sobre el ECA ponderados por población, y **personas-día sobre el ECA**,
  en `reports/mapa_distritos.json` (chico, versionado), que el sistema en Go carga a MongoDB.

## 7. Honestidad del mapa

- Cada celda lleva su **distancia a la estación más cercana** y la **desviación entre particiones**. La SPA pinta
  con menos opacidad lo que está lejos de toda verdad.
- La resolución efectiva es la de CAMS (0.75°) corregida con 22 puntos; los 0.1° son la grilla de cálculo, no la
  precisión. El informe lo dice con esas palabras.
- Si M2 no le gana a M1 en estaciones no vistas, el mapa sale de M1 y el informe lo dice.

## 8. Código

| Ruta | Qué | Dependencias |
|---|---|---|
| `lab/mapa/datos/` | Descarga (CDS, ADS, WorldPop, geoBoundaries), regrillado, normalización, `dias/`, `estaticos.npy`, `etiquetas.csv`, `manifiesto.json` | Su propio `pyproject.toml` con uv: cdsapi, xarray, netCDF4, scipy, pandas, rasterio, geopandas |
| `lab/mapa/modelo/` | M0, M1, M2; entrenamiento en un solo dispositivo; `nodo_mapa.py` (nodo del protocolo v2); inferencia; agregación por distrito | `pyproject.toml` propio: torch, numpy, xgboost, pandas. **El nodo y la inferencia por nodo usan solo numpy, torch y la biblioteca estándar** |
| `protocolo/` | Carga binaria (`bytes`), `HOLA` v2, `TAREA`, `PESOS`, `LOCAL`, `MaxCarga`; la v1 no cambia | Biblioteca estándar |
| `cluster/` + `cmd/mapa-coord` | Espera de nodos, reparto de bloques, rondas de local SGD, promedio en orden de nodo | Biblioteca estándar |

Todo se prueba primero en la laptop, en CPU, con una **grilla y un periodo reducidos** (por ejemplo 24 × 16 celdas
y 30 días sintéticos): el mismo código, otros tamaños en el manifiesto. Las pruebas de Go con `-race`.

## 9. Calendario

| Cuándo | Qué |
|---|---|
| sáb 14/11 | Spec; cuentas de Copernicus; las descargas empiezan; código de las tres partes contra datos sintéticos |
| 16–21/11 | **La PC4 tiene prioridad.** Las descargas y el preprocesamiento corren solos; M0 y M1 en la laptop |
| Primera clase con laboratorio después del 21 | Entrenamiento distribuido (H, nodos), granja de particiones e inferencia nacional |
| 29/11 (TB2) | Mapa en la SPA, sección en el informe |

## 10. Decisiones abiertas

1. SENAMHI Lima como verdad adicional (depende de que se baje a mano).
2. Licencia de geoBoundaries ADM3 para el Perú; si no sirve, los límites del INEI.
3. Modo granja con un solo coordinador.
4. Si el laboratorio tiene RAM para tener la parte de cada nodo en memoria (no medido).
