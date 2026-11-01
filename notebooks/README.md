# notebooks/ — laboratorio en Python (Colab)

**Python es laboratorio, no sistema.** La carga, la limpieza, la estandarización y el K-Means del
proyecto son Go (`cmd/cargar`, `cmd/entrenar`, `cmd/benchmark`), como exige el enunciado, y **Go es
la fuente de verdad**. Estos notebooks **exploran** el dato, **calibran** la receta que Go lee,
**auditan** lo que Go produce con una segunda implementación y **dibujan** las figuras del informe.
Nada de aquí entra al sistema salvo la receta, y esa la copia una persona a `configs/kmeans.json`
después de leer la tabla de decisión.

## Orden

| # | Notebook | Cuándo | Qué hace | Qué produce |
|---|---|---|---|---|
| 1 | `01_eda_criba.ipynb` | con el bronze en Drive | Primer MB y sha256; lectura con `configs/limpieza.json` y la **cascada de reglas de Go**; valores vistos; **cada medición de la criba** (Q1–Q9) | `reports/eda_criba.md`, `eda_limpieza_prototipo.json` (esquema de `limpieza.json`), `eda_completitud.csv`, `eda_estaciones.csv`, `eda_bronze.json`, `figuras_eda/`; `data/silver/oefa_ancho_prototipo.csv`; figuras del informe `completitud.png` y `distribuciones.png` |
| 2 | `02_calibracion.ipynb` | antes de congelar la receta | Grilla de la spec de calibración §3 (24 variantes × k 2–10 × 5 semillas) y regla de elección en su orden | `reports/calibracion.csv`, `calibracion_resumen.csv`, `calibracion_decision.md`, `kmeans_propuesto.json` (esquema de `configs/kmeans.json`), `centroides_sklearn.json`, `figuras_calibracion/` |
| 3 | `03_auditoria_figuras.ipynb` | cuando Go ya corrió | Recalcula limpieza, silver y gold desde el bronze y los compara con Go; audita `kmeans.json`, asignaciones, `regimenes.json`, `codo.json` y los benchmarks; dibuja las figuras | `reports/auditoria.md` y `.json`; figuras del informe `perfiles.png`, `regimen-hora.png`, `regimen-estacion.png`, `episodio.png`, `codo.png`, `speedup.png` (y `cobertura.png`) |

Los nombres de las figuras son los que esperan `docs/informe/secciones/*.qmd`. Cada figura queda
anotada en `origen.json` (en la misma carpeta) con su fuente y si los datos son sintéticos; con datos
sintéticos (la muestra `data/sample/`, `sintetico.py` o estaciones `SINT-`/`SIN-`) sale además con la
marca de agua «DATOS SINTÉTICOS».

Estados de la auditoría: **OK**; **DIFERENCIA** (bug en Go o en Python; el detalle dice cuál
parece); **AVISO** (no es un bug pero hay que mirarlo, p. ej. Go quedó en otro mínimo local);
**OMITIDO** (falta la salida de Go). El 03 no se cae si Go todavía no corrió: las figuras usan los
regímenes de scikit-learn y lo dicen.

Specs: `docs/specs/2026-10-31-preguntas-candidatas.md` (01), `docs/specs/2026-10-31-kmeans-calibracion-design.md`
§1–§4 (02 y 03) y `docs/specs/2026-10-31-pc3-design.md` §4 y §9.

## Abrirlos en Colab

1. **Drive**: las capas viven en `MyDrive/aire-distribuido/`:
   ```
   aire-distribuido/
     data/bronze/<CSV de OEFA>.csv     # el 01 toma el CSV más grande de la carpeta
     data/silver/  data/gold/          # los escribe Go (y el 01 el silver prototipo)
     reports/                          # Go y Colab; las figuras del 03 quedan en reports/figuras/
     configs/                          # opcional: si existe, pisa a la del repo
   ```
2. **Entorno de ejecución**: *Entorno de ejecución → Cambiar tipo de entorno* → CPU con **RAM alta**.
   El 01 y el 03 leen el CSV de 1.3 GB entero; el 02 usa todos los núcleos con `joblib`. Sin GPU.
3. **Abrir**: *Archivo → Abrir cuaderno → GitHub* → `R0SEWT/aire-distribuido`, rama `feature/spec-pc3` (hasta que se fusione a `develop`)
   (o la rama donde estén) → `notebooks/0X_….ipynb`.
4. **Ejecutar todo**. La celda de entorno monta Drive y clona el repo en `/content/aire-distribuido`
   para importar `aire_lab.py` y leer `configs/`. Prueba las ramas de `RAMAS` en orden;
   `ACTUALIZAR_CODIGO = True` vuelve a clonar si cambió `aire_lab.py`.

Lo que se toca está arriba de cada notebook: `RAIZ_DRIVE`, `RAMAS`; en el 01, `AJUSTES` (para probar
un cambio de `configs/limpieza.json` antes de llevarlo al JSON) y los umbrales de la criba `U`; en
el 02, `N_SUBMUESTRA`, `UMBRALES`, `K_DESDE_CODO`, `GASES_EN_FEATURES`; en el 03, las tolerancias y
`ESTACION_EPISODIO`. Variables de entorno: `AIRE_RAIZ` (raíz de las capas), `AIRE_BRONZE` (el CSV),
`AIRE_FIGURAS` (dónde van las figuras del informe).

**Tiempos** (medidos con un sintético de 1.5 M filas largas y 210 000 anchas): el 02 hace 1 080
corridas de unos 6 s cada una (≈ 105 min de CPU: 15–20 min con 8 núcleos, más de una hora con 2;
con `N_SUBMUESTRA = 100_000`, la mitad). Con 1.5 M filas largas pasadas por Go, el 01 tardó 39 s y el
03 71 s (≈ 1.6 GB de RAM); el CSV real tiene varias veces más filas largas, por eso la RAM alta.

## Correrlos en local contra Go

```bash
# 1. Go sobre la muestra (o sobre el bronze real): salidas en una raíz aparte
R=/tmp/aire
go run ./cmd/cargar -entrada data/sample/oefa_muestra.csv -silver $R/data/silver/oefa_ancho.csv \
   -gold $R/data/gold/oefa_features.csv -reports $R/reports -workers 1,2,4 -bench 3
go run ./cmd/entrenar -gold $R/data/gold/oefa_features.csv -estandarizacion $R/reports/estandarizacion.json \
   -salida $R/reports/kmeans.json -asignaciones $R/data/gold/asignaciones.csv \
   -regimenes $R/reports/regimenes.json -codo 2:8 -codo-salida $R/reports/codo.json
go run ./cmd/benchmark -gold $R/data/gold/oefa_features.csv -workers 1,2,4 -chunk 100 -repes 3 \
   -salida $R/reports/benchmark_kmeans.json

# 2. Los notebooks sobre esas salidas (las figuras van a docs/informe/generado/figuras/)
cd notebooks
for nb in 01_eda_criba 02_calibracion 03_auditoria_figuras; do
  AIRE_RAIZ=$R AIRE_BRONZE=../data/sample/oefa_muestra.csv \
    jupyter nbconvert --to notebook --execute $nb.ipynb --output-dir $R/ejecutados
done
```

Sin `AIRE_RAIZ`, fuera de Colab la raíz es **el repo** (sus `data/` están en `.gitignore`;
`reports/` es la carpeta versionada de salidas chicas).

**A escala**, `python notebooks/sintetico.py /tmp/aire-sin --dias 1500 --go` genera un bronze
sintético en el formato de `configs/limpieza.json` (6 estaciones `SIN-` en 3 departamentos, cinco
regímenes plantados, huecos tras los episodios, suciedad de cada regla) y corre `cmd/cargar` y
`cmd/entrenar` sobre él. El script no imita a Go: con `--go`, las salidas son las de Go.

Dependencias: `pandas`, `numpy`, `matplotlib`, `scikit-learn`; `scipy` (asignación húngara; sin
ella, fuerza bruta para k chico) y `joblib` (sin ella la grilla corre en serie). Colab las trae; la
primera celda instala lo que falte. Probado de punta a punta con Python 3.13 + pandas 3.1 +
scikit-learn 1.9 y con Python 3.12 + pandas 2.2.2 + numpy 2.0.2 + scikit-learn 1.6.1 +
matplotlib 3.10.0 (lo de Colab).

## Archivos

| Archivo | Qué es |
|---|---|
| `aire_lab.py` | Código compartido. El prototipo de limpieza replica `datos/bloque.go` (cascada), `datos/recolector.go` (regla 5), `datos/reporte.go` (esquema de `limpieza.json`), `datos/salida.go` (silver byte a byte, `FormatFloat 'g' -1`), `datos/estandarizar.go` (gold) y `kmeans/regimenes.go` (perfil y ECA). Además: criba, UTM → lat/lon, huecos, ECA, métricas y estilo de figuras. |
| `sintetico.py` | Bronze sintético en el formato de Go y, con `--go`, las salidas del sistema real. |
| `0X_*.ipynb` | Los notebooks, **guardados sin salidas**. |

## Contrato con Go (esquema real)

- **`configs/limpieza.json`** (`datos.ConfigLimpieza`, `datos/doc.go`): `delimitador`, `codificacion`,
  `separador_decimal`, `zona_horaria`, `columnas` (`estacion, fecha, hora, parametro, valor, periodo,
  unidad` + ubicación; vacío = no existe), `parametros` (real → canónico, o `{"canonico", "factor"}`),
  `periodos_horarios`, `formatos_fecha` (layouts de Go), `fecha_desde`, `fecha_hasta` (exclusiva),
  `centinelas` (sobre el valor crudo), `admiten_negativos`, `rangos` (unidad canónica), `unidades`.
  Como en Go, el JSON es la config entera y un campo desconocido es un error. Nombres comparados con
  `datos.normalizar` (minúsculas, sin tildes, µ → u, ³ → 3, espacios colapsados).
- **Cascada** (cada fila cuenta en la primera que falla): `columnas` → `estacion` → `periodo` →
  `parametro` → `fecha` → `fecha_fuera_de_rango` → `valor_no_numerico` → `centinela` → `unidad` →
  `negativo` → (factor) → `fuera_de_rango` → `duplicado_exacto` / `duplicado_valor_distinto`.
- **`reports/limpieza.json`**: `filas_largas` {`leidas, procesadas, descartadas,
  descartadas_por_regla` (las 13), `marcadas_por_regla` {`duplicado_valor_distinto`: celdas},
  `fuera_de_rango_por_parametro`}, `filas_anchas` {`total, completas_gold, pm2_5_mayor_pm10`},
  `parametros_presentes`, `estaciones` {`largas_leidas, largas_procesadas, anchas, completas_gold,
  desde, hasta, ubicacion`}, `vistos` {`parametros, parametros_no_mapeados, periodos, unidades`},
  `cabecera, bom, delimitador, entrada`.
- **silver** `data/silver/oefa_ancho.csv`: `estacion,fecha_hora,<canónicos presentes en orden de
  bytes>`, coma, vacío = faltante, RFC3339 con `-05:00`, ordenado por estación y fecha-hora,
  `FormatFloat(x, 'g', -1, 64)`. El prototipo lo reproduce **byte a byte**.
- **gold** `data/gold/oefa_features.csv`: features de la receta en su orden; `fraccion_fina` =
  pm2_5/pm10 recortada a [0, 1] (faltante si pm10 ≤ 0); log1p; filas con todo finito; z-score con
  desviación poblacional; una feature con desviación 0 queda en 0.
- **`reports/estandarizacion.json`**: `features, log1p, nivel, n, media, desv, ddof (= 0), chunk` y,
  con nivel `estacion`, `por_estacion` {estación: {`n, media, desv`}}.
- **`reports/kmeans.json`**: `receta`, `gold` {`sha256`, `n`}, `k`, `k_efectivo`, `iteraciones`,
  `paro`, `tol_abs`, `tol_rel`, `inercia` (con los centroides finales), `inercias_por_iteracion`,
  `tamanos`, `centroides_estandarizados`, `centroides_des_estandarizados`.
- **`data/gold/asignaciones.csv`**: `estacion,fecha_hora,regimen`, con los centroides finales.
- **`reports/regimenes.json`**: `regimenes[]` con `regimen, tamano, fraccion, perfil` (media
  aritmética de cada feature en unidades originales, invirtiendo cada fila), `pm2_5_medio`
  (`pm2_5_origen`: la feature o `fraccion_fina*pm10`), `pm10_medio`, `eca` (`excede` / `no_excede`
  / `no_evaluable`), `eca_por`, `por_hora` (24), `por_estacion`.
- **`reports/codo.json`**: `puntos[]` {`k, n, inercia, iteraciones, paro`}. **Benchmarks**:
  `benchmark_cargador.json` (`mediciones[]` {`workers, tiempos_s, mediana_s, speedup, sha256_*`},
  `identicos`) y `benchmark_kmeans.json` (`resumen[]` con `speedup` {`punto, inferior, superior`},
  `determinismo[]`).

## El CSV real de OEFA (formato «largo con columnas»)

El CSV real (spec PC3 §4.1) no tiene el formato de la muestra: una columna por parámetro, pero cada
fila trae un solo parámetro con valor; la hora falta en el 94 %. Mientras `configs/limpieza.json`
describa la muestra, el 01 detecta el formato real por la cabecera y usa
`aire_lab.CONFIG_OEFA_REAL`, que deja copiada en `reports/limpieza_propuesta_real.json` para
compararla con la del cargador de Go. Claves nuevas (nombres acordados para el cargador; las marcadas
como propuesta aún no están en Go):

- `parametros_en_columnas` (columna → canónico; canónico nuevo `h2s`), `hora_prefijo_ignorar`
  (`1899-12-31`, el artefacto de Excel; la fecha sola es las 00:00), `hora_por_posicion`
  (`{"activar", "filas_por_dia": 24, "convencion": "fin"}`).
- propuestas: `valores_vacios` (`-`, `NA`, vacío), `periodos_horarios_por_grupo` (`-` es horario solo
  con hora o con su bloque de 24 completo) y `bloques_en_cero_como_faltante` (lista de canónicos).
- Reglas nuevas en la cascada: `hora` (sin hora y bloque incompleto, después de `fecha`),
  `bloque_en_cero` (**propuesta**: un bloque del día entero en 0 exacto es un equipo sin medir; después
  de `fuera_de_rango`) y `duplicado_unidad` (la misma lectura en ppb o mmHg cuando está en µg/m³ o hPa;
  antes de los duplicados). Marcas: `hora_inferida`, `unidad_convertida`.
- Silver: columna `hora_inferida` al final (1 si alguna lectura de la fila tuvo la hora inferida).
- **Convención `fin`**: la lectura de la posición *p* (0 a 23) cubre [*p*, *p*+1) y se etiqueta *p*+1;
  las horas explícitas de `HORA_INICIO` (que son de inicio) también se corren +1 h, o quedan desfasadas
  de las inferidas.
- 100 registros (CA-VMP-1, periodo 24 horas) traen un salto de línea dentro de un campo entrecomillado:
  el escaneo del prototipo los une (como pandas); el divisor de Go por líneas los rechazaría.

## Calibración (02): decisiones de la revisión

- La etiqueta ECA de pureza y recall es el **promedio móvil de 24 h** (`aire_lab.movil_24h`: ventana
  que termina en la hora, al menos 18 h), no la lectura horaria. Además se guarda el **mejor
  régimen** frente al ECA (el de mayor proporción de horas sobre el ECA), aunque su media no lo supere.
- «No es geografía»: la NMI del régimen con la estación no supera en más de 0,10 a la de una línea
  base, la partición en k niveles de PM10. La regla de la spec (NMI con estación < NMI con la etiqueta
  ECA) casi no puede cumplirse con una etiqueta binaria rara.
- La submuestra sistemática usa un paso coprimo con 24, para no fijar horas del día.
- Un criterio que no cumple nadie se dice y se salta; los demás se siguen aplicando en orden.
- `kmeans_propuesto.json` copia de `configs/kmeans.json` lo que la grilla no calibra (`semilla`,
  `semillas`, `tolerancia`, `max_iter`, `workers`, `chunk`).

`sintetico.py` todavía escribe el formato del primer borrador (parámetro y valor); el cargador de Go
ya no lo lee, así que su opción `--go` queda pendiente de actualizar. Para probar contra Go están las
dos muestras de `data/sample/` (formato real).

## Lo que hay que confirmar con el CSV real

Todo en `configs/limpieza.json` (probar antes con `AJUSTES` en el 01): nombres de columnas (el
error sugiere uno parecido), si la hora viene aparte (`columnas.hora`), `formatos_fecha` (los
ejemplos de lo que no se pudo leer salen en la tabla de reglas; Go no lee `24:00`), nombres de
parámetros sin canónico, valores de periodo horarios, unidades por parámetro, centinelas, rangos,
`fecha_desde`/`fecha_hasta`, `separador_decimal`, `codificacion` y `delimitador` (el primer MB los
sugiere).

## Antes de commitear

Los notebooks se guardan **sin salidas** y los datos no se versionan:

```bash
jupyter nbconvert --clear-output --inplace notebooks/*.ipynb
```
