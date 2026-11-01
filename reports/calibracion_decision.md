# Calibración del K-Means — tabla de decisión

> Generado por `notebooks/02_calibracion.ipynb` el 2026-11-01 14:03. Silver `oefa_ancho.csv` (sha256 `3e84071aa856327e…`), 233 707 filas anchas; submuestra sistemática cada 1.ª fila (233 707 filas). 24 variantes × 9 valores de k × 5 semillas = 1080 corridas (scikit-learn 1.9.1). Spec: `docs/specs/2026-10-31-kmeans-calibracion-design.md` §3.

## Decisión

|  | elegida | segunda | provisional (F2-M1-T2-N1, k = 5) |
|---|---|---|---|
| variante | `F2-M0-T1-N2` | `F2-M0-T2-N1` | `F2-M1-T2-N1` |
| features | pm10, fraccion_fina | pm10, fraccion_fina | pm10, fraccion_fina, viento_velocidad, humedad |
| log1p | ninguna | pm10 | pm10 |
| (1) estable | sí | sí | no |
| (2) separa el ECA | no | no | no |
| (3) no es geografía | sí | sí | no |
| (4) interpretable | sí | sí | sí |
| k | 4 | 4 | 5 |
| codo de la variante | 4 | 4 | 6 |
| filas (submuestra) | 63 821 | 63 821 | 41 991 |
| estaciones | 17 | 17 | 11 |
| ARI entre semillas | 1.000 | 0.974 | 0.633 |
| ARI por días | 0.873 | 0.967 | 0.540 |
| pureza «excede» | 0.091 | 0.000 | 0.000 |
| recall «excede» | 0.075 | 0.000 | 0.000 |
| fracción de horas en «excede» | 0.015 | 0.000 | 0.000 |
| NMI estación | 0.010 | 0.105 | 0.215 |
| NMI estación de la línea base (nivel de PM10) | 0.099 | 0.099 | 0.096 |
| NMI etiqueta ECA (24 h) | 0.003 | 0.007 | 0.003 |
| NMI hora | 0.082 | 0.071 | 0.117 |
| NMI mes | 0.005 | 0.017 | 0.036 |
| pureza del mejor régimen frente al ECA | 0.091 | 0.040 | 0.028 |
| recall del mejor régimen | 0.075 | 0.435 | 0.400 |
| tamaño del mejor régimen | 0.015 | 0.199 | 0.198 |
| cluster más chico | 0.015 | 0.193 | 0.149 |
| separación L∞ mínima | 1.39 | 1.25 | 1.27 |
| silueta | 0.479 | 0.393 | 0.268 |
| inercia | 32 644.7 | 30 633.1 | 60 013.6 |

**Variante elegida: `F2-M0-T1-N2` con k = 4.** Por qué: cumple (1) estable, (3) no es geografía, (4) interpretable (nadie cumple (2) separa el ECA: se dice y se salta); es el k más chico que lo logra desde el codo (k = 4); cubre 17 estaciones; pureza × recall = 0.007, silueta 0.479.

**Qué cede frente a la segunda**: la segunda (`F2-M0-T2-N1`, k = 4) es mejor en: ARI por días (0.967 frente a 0.873); NMI etiqueta ECA (24 h) (0.007 frente a 0.003); recall del mejor régimen (0.435 frente a 0.075); cluster más chico (0.193 frente a 0.015). Pierde porque tiene menor pureza × recall.

**Umbrales que no cumplió nadie** (desde el codo): (2) separa el ECA (umbral: pureza > 0.6 y recall > 0.5). Como nadie cumple (2) separa el ECA, ese criterio se salta y se aplican los demás en orden.

Umbrales usados: ari_semillas = 0.8, ari_dias = 0.7, pureza = 0.6, recall = 0.5, margen_geografia = 0.1, min_cluster = 0.01, sep_linf = 1.0; k desde el codo: True.

## Cuántos candidatos cumplen cada criterio

| criterio | cumplen solos | cumplen este y los anteriores aplicados | de |
|---|---|---|---|
| (1) estable | 68 | 68 | 153 |
| (2) separa el ECA | 0 | nadie: se salta | 153 |
| (3) no es geografía | 108 | 45 | 153 |
| (4) interpretable | 61 | 16 | 153 |

## Mejor k de cada variante

Para cada variante, el k más chico (desde su codo) con el prefijo de criterios más largo.

| variante | k | codo | prefijo | n_estaciones | ari_semillas | ari_dias | pureza | recall | nmi_estacion | nmi_eca | min_cluster | sep_linf | silueta |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| F2-M0-T1-N2 | 4 | 4 | 3 | 17 | 1.000 | 0.873 | 0.091 | 0.075 | 0.010 | 0.003 | 0.015 | 1.391 | 0.479 |
| F2-M0-T2-N1 | 4 | 4 | 3 | 17 | 0.974 | 0.967 | 0.000 | 0.000 | 0.105 | 0.007 | 0.193 | 1.247 | 0.393 |
| F2-M0-T2-N2 | 4 | 4 | 3 | 17 | 0.982 | 0.957 | 0.000 | 0.000 | 0.011 | 0.002 | 0.200 | 1.277 | 0.386 |
| F1-M1-T1-N1 | 5 | 5 | 3 | 11 | 0.985 | 0.941 | 0.222 | 0.349 | 0.180 | 0.021 | 0.020 | 1.688 | 0.314 |
| F1-M1-T1-N2 | 5 | 5 | 3 | 11 | 0.984 | 0.940 | 0.081 | 0.126 | 0.011 | 0.005 | 0.020 | 1.148 | 0.291 |
| F1-M1-T2-N2 | 5 | 5 | 3 | 11 | 0.971 | 0.931 | 0.000 | 0.000 | 0.006 | 0.003 | 0.160 | 1.167 | 0.253 |
| F2-M1-T1-N2 | 5 | 5 | 3 | 11 | 0.949 | 0.931 | 0.058 | 0.150 | 0.015 | 0.002 | 0.033 | 1.184 | 0.267 |
| F2-M1-T2-N2 | 5 | 5 | 3 | 11 | 0.964 | 0.959 | 0.000 | 0.000 | 0.017 | 0.002 | 0.156 | 1.152 | 0.247 |
| F2-M2-T2-N2 | 5 | 5 | 3 | 11 | 0.965 | 0.921 | 0.000 | 0.000 | 0.009 | 0.002 | 0.145 | 1.089 | 0.196 |
| F1-M2-T1-N2 | 6 | 5 | 3 | 11 | 0.810 | 0.709 | 0.086 | 0.101 | 0.007 | 0.003 | 0.015 | 1.070 | 0.186 |
| F1-M2-T2-N2 | 6 | 4 | 3 | 11 | 0.978 | 0.925 | 0.000 | 0.000 | 0.006 | 0.003 | 0.126 | 1.240 | 0.182 |
| F1-M0-T1-N2 | 4 | 4 | 2 | 17 | 0.960 | 0.902 | 0.090 | 0.056 | 0.003 | 0.006 | 0.010 | 0.869 | 0.453 |
| F1-M0-T2-N1 | 4 | 4 | 2 | 17 | 0.905 | 0.889 | 0.000 | 0.000 | 0.124 | 0.024 | 0.108 | 0.874 | 0.400 |
| F1-M0-T2-N2 | 4 | 4 | 2 | 17 | 0.950 | 0.930 | 0.000 | 0.000 | 0.005 | 0.005 | 0.146 | 0.909 | 0.377 |
| F1-M0-T1-N1 | 5 | 5 | 2 | 17 | 0.974 | 0.867 | 0.383 | 0.375 | 0.103 | 0.048 | 0.008 | 0.753 | 0.451 |
| F2-M0-T1-N1 | 5 | 5 | 2 | 17 | 0.979 | 0.842 | 0.238 | 0.227 | 0.068 | 0.020 | 0.009 | 0.803 | 0.400 |
| F2-M1-T1-N1 | 6 | 5 | 2 | 11 | 0.836 | 0.804 | 0.136 | 0.268 | 0.172 | 0.008 | 0.003 | 1.135 | 0.290 |
| F1-M1-T2-N1 | 4 | 4 | 1 | 11 | 0.987 | 0.720 | 0.000 | 0.000 | 0.229 | 0.004 | 0.212 | 1.677 | 0.306 |
| F2-M2-T1-N1 | 4 | 4 | 1 | 11 | 0.998 | 0.991 | 0.000 | 0.000 | 0.437 | 0.001 | 0.190 | 1.433 | 0.380 |
| F2-M2-T2-N1 | 4 | 4 | 1 | 11 | 0.819 | 0.902 | 0.000 | 0.000 | 0.442 | 0.002 | 0.137 | 1.328 | 0.365 |
| F1-M2-T1-N1 | 5 | 5 | 1 | 11 | 0.998 | 0.978 | 0.176 | 0.477 | 0.427 | 0.019 | 0.036 | 1.597 | 0.346 |
| F1-M2-T2-N1 | 5 | 5 | 1 | 11 | 0.842 | 0.749 | 0.000 | 0.000 | 0.421 | 0.003 | 0.146 | 1.405 | 0.309 |
| F2-M1-T2-N1 | 6 | 6 | 1 | 11 | 0.839 | 0.884 | 0.000 | 0.000 | 0.234 | 0.004 | 0.092 | 1.289 | 0.278 |
| F2-M2-T1-N2 | 5 | 5 | 0 | 11 | 0.686 | 0.688 | 0.059 | 0.131 | 0.010 | 0.002 | 0.025 | 1.269 | 0.221 |

## ¿Algún régimen aísla los episodios?

Etiqueta: el promedio móvil de 24 h sobre el ECA (1.19 % de las horas de la submuestra). «Excede» por la media del régimen: 117 de 216 candidatos tienen algún régimen cuya media supera el ECA. El mejor régimen de cada candidato (el de mayor proporción de horas sobre el ECA), los diez de mayor pureza:

| variante | k | pureza_mejor | recall_mejor | frac_mejor | pureza | recall | frac_excede | ari_semillas |
|---|---|---|---|---|---|---|---|---|
| F1-M0-T1-N1 | 10 | 0.804 | 0.028 | 0.001 | 0.226 | 0.513 | 0.044 | 0.713 |
| F1-M0-T1-N1 | 9 | 0.795 | 0.030 | 0.001 | 0.264 | 0.418 | 0.030 | 0.919 |
| F1-M0-T1-N1 | 8 | 0.753 | 0.124 | 0.003 | 0.221 | 0.540 | 0.047 | 0.867 |
| F1-M0-T1-N1 | 7 | 0.705 | 0.190 | 0.005 | 0.254 | 0.443 | 0.036 | 0.715 |
| F1-M0-T1-N1 | 6 | 0.691 | 0.188 | 0.005 | 0.215 | 0.457 | 0.041 | 0.717 |
| F1-M1-T1-N1 | 10 | 0.691 | 0.113 | 0.002 | 0.184 | 0.352 | 0.028 | 0.764 |
| F1-M1-T1-N1 | 9 | 0.645 | 0.086 | 0.002 | 0.173 | 0.483 | 0.044 | 0.703 |
| F1-M0-T1-N1 | 5 | 0.604 | 0.278 | 0.008 | 0.383 | 0.375 | 0.018 | 0.974 |
| F1-M2-T1-N1 | 10 | 0.591 | 0.124 | 0.003 | 0.160 | 0.401 | 0.040 | 0.731 |
| F1-M0-T1-N1 | 4 | 0.526 | 0.268 | 0.009 | 0.226 | 0.555 | 0.049 | 0.757 |

## Receta propuesta (`reports/kmeans_propuesto.json`)

Mismo esquema que `configs/kmeans.json`. `semilla`, `tolerancia`, `max_iter`, `workers` y `chunk` no los calibra la grilla: se copian del `configs/kmeans.json` actual. Copiar la propuesta allí es decisión de Rody.

```json
{
  "features": [
    "pm10",
    "fraccion_fina"
  ],
  "log1p": [],
  "nivel": "estacion",
  "k": 4,
  "semilla": 42,
  "semillas": 10,
  "tolerancia": 1e-06,
  "max_iter": 100,
  "workers": 0,
  "chunk": 10000
}
```

Centroides de scikit-learn sobre todo el silver (63 821 filas, semilla 23, inercia 32644.5196) en `reports/centroides_sklearn.json`, para el test de equivalencia con Go.
