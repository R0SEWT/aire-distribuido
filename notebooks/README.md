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
| 2 | `02_calibracion.ipynb` | antes de congelar la receta | Grilla de la spec de calibración §3 (24 variantes × k 2–10 × 5 semillas) y regla de elección en su orden; §8, la **decisión del equipo** (se corre sola, sin la grilla) | `reports/calibracion.csv`, `calibracion_resumen.csv`, `calibracion_decision.md` (con la sección «Decisión del equipo»), `kmeans_propuesto.json` (esquema de `configs/kmeans.json`), `centroides_sklearn.json`, `descriptores_regimen.csv`, `figuras_calibracion/` |
| 3 | `03_auditoria_figuras.ipynb` | cuando Go ya corrió | Recalcula limpieza, silver y gold desde el bronze y los compara con Go; audita `kmeans.json` (y sus semillas, desde `centroides_iniciales.json`), asignaciones, `regimenes.json`, `codo.json` y los benchmarks (que sean del gold y la receta auditados); dibuja las figuras y las muestra | `reports/auditoria.md` y `.json`; figuras del informe `perfiles.png`, `niveles.png`, `regimen-hora.png`, `regimen-estacion.png`, `episodio.png`, `codo.png`, `speedup.png` (y `cobertura.png`) |

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

## Iterar en Colab

**Abrir** desde GitHub, con la rama en la URL (la celda de entorno trae el código de esa misma rama):

- 01: <https://colab.research.google.com/github/R0SEWT/aire-distribuido/blob/feature/spec-pc3/notebooks/01_eda_criba.ipynb>
- 02: <https://colab.research.google.com/github/R0SEWT/aire-distribuido/blob/feature/spec-pc3/notebooks/02_calibracion.ipynb>
- 03: <https://colab.research.google.com/github/R0SEWT/aire-distribuido/blob/feature/spec-pc3/notebooks/03_auditoria_figuras.ipynb>

Para otra rama, se cambia `feature/spec-pc3` en la URL (`develop` cuando se fusione).

**Entorno**: CPU con **RAM alta**, sin GPU. Los notebooks ya lo piden (`machine_shape: hm`; con Colab Pro
el botón dice *Connect High-RAM*); si no, *Entorno de ejecución → Cambiar tipo de entorno*. La RAM no es
el límite (picos de unos 3 GB en el 01 y en el 03: caben en los 12.7 GB del estándar); la RAM alta
trae 8 CPU en vez de 2, y con eso el 02 corre la grilla completa.

**Ejecutar**: *Entorno de ejecución → Ejecutar todo* (Ctrl+F9); al aviso de que el notebook no es de
Google, *Ejecutar de todos modos*. La celda de entorno (§0), la misma en los tres notebooks:

1. monta Drive. Cada notebook abre su propio entorno y cada entorno nuevo pide autorizarlo una vez
   (cuadro «Permit this notebook to access your Google Drive files?» → *Connect to Google Drive* → la
   **cuenta personal**, la del `MyDrive/aire-distribuido/`); *Ejecutar todo* espera ahí hasta 15 min;
2. deja el repo en `/content/aire-distribuido` en la rama de la URL: lo clona la primera vez y en cada
   corrida hace `git fetch` + `git reset --hard origin/<rama>` (si no puede leer la URL, usa `RAMA`);
3. instala lo que falte (pandas, numpy, matplotlib, scikit-learn, scipy, joblib; Colab los trae) y pone
   el reloj en hora de Lima (Colab corre en UTC: si no, las fechas de los reportes salen +5 h);
4. imprime si es Colab, las versiones, **rama y commit**, la raíz de los datos, dónde van las figuras,
   CPU y RAM;
5. verifica los insumos en Drive y, si falta algo o las salidas de Go son de otra receta o de otro gold,
   se detiene con el comando de `rclone` que lo arregla.

**Tomar cambios del repo**: `git push` a la rama y volver a correr la celda de entorno (o *Ejecutar
todo*): trae el último commit y recarga `aire_lab`. Si cambiaron las celdas del `.ipynb`, recargar la
pestaña (Colab lee el notebook de GitHub al abrirlo). `ACTUALIZAR_CODIGO = False` congela el código
mientras se prueba algo a mano. Lo que se edite en Colab no vuelve solo a GitHub: se copia al repo (o
*Archivo → Guardar una copia en GitHub*), siempre sin salidas.

**Insumos** (`MyDrive/aire-distribuido/`; los comandos van desde la raíz del repo, en la máquina donde
corrió Go; la celda de entorno imprime el que falte):

| Notebook | Lee | Se sube con |
|---|---|---|
| 01 | `data/bronze/*.csv` (el más grande) y `SHA256SUMS` | `rclone copy data/bronze gdrive:aire-distribuido/data/bronze --progress` |
| 02 | `data/silver/oefa_ancho.csv` (o el prototipo del 01); la §8, además `data/gold/asignaciones.csv` y `reports/regimenes.json` | los tres de abajo |
| 03 | silver, gold y asignaciones de Go; `reports/` de Go; el bronze (sin él, la auditoría de la limpieza queda OMITIDA) | los tres de abajo y el del bronze |

```bash
rclone copy data/silver gdrive:aire-distribuido/data/silver --include "oefa_ancho.csv"
rclone copy data/gold gdrive:aire-distribuido/data/gold --include "{oefa_features,asignaciones}.csv"
rclone copy reports gdrive:aire-distribuido/reports --include "{limpieza,estandarizacion,kmeans,regimenes,centroides_iniciales,codo,benchmark_cargador,benchmark_kmeans}.json"
```

Una carpeta `configs/` en Drive pisa a la del repo (la celda avisa si difieren); lo normal es no tenerla.

**Salidas** (en Drive; cada figura se muestra también en el notebook): el 01, `reports/eda_*`,
`reports/figuras_eda/`, `data/silver/oefa_ancho_prototipo.csv` y `completitud.png`, `distribuciones.png` en
`reports/figuras/`; el 02, `reports/calibracion*`, `kmeans_propuesto.json`, `centroides_sklearn.json`,
`descriptores_regimen.csv` y `reports/figuras_calibracion/`; el 03, `reports/auditoria.{md,json}` y las
figuras del informe en `reports/figuras/` con su `origen.json`. Al informe se traen con
`rclone copy gdrive:aire-distribuido/reports/figuras docs/informe/generado/figuras`.

**Tiempos** (CSV de OEFA de 1.3 GB, 2 883 877 filas largas; medidos el 01/11. Local: Python 3.12,
pandas 2.2.2, numpy 2.0.2, scikit-learn 1.6.1. Colab con RAM alta: 8 CPU, 51 GiB, Python 3.13.18,
pandas 2.2.3, numpy 2.1.3, scikit-learn 1.6.1):

| Notebook | Local, 12–14 CPU | Local, 2 CPU (`taskset`) | Pico de RAM | Colab |
|---|---|---|---|---|
| 01 | 1 min 53 s | — | 3.3 GB | 2 min 30 s desde la autorización (sha256 del CSV en Drive 27 s, lectura 38 s, cascada 24 s; pico 3.0 GB) |
| 02 | ≈ 9 min (grilla completa: 6 176 s de CPU) | 2 min 40 s (grilla reducida) | 0.5 GB | sin medir de punta a punta: con 8 CPU elige la grilla completa e hizo 25 de sus 216 tareas en 1 min (se interrumpió) |
| 03 | 2 min 9 s | — | 2.4 GB | 1 min 35 s desde la autorización (38 OK, 0 diferencias; pico 3.0 GB) |

En Colab los tiempos cuentan desde la autorización de Drive; leer el CSV desde Drive no pesa (27 s el
sha256 de 1.3 GB, 38 s la lectura).

**Los tres errores más probables**:

1. `FileNotFoundError: Faltan insumos…` o «Las salidas de Go no son de la receta ni del gold actuales»:
   Drive no tiene lo que el notebook lee, o tiene una corrida de Go anterior a `configs/kmeans.json`.
   Correr el `rclone copy` que imprime la celda (después de correr Go si cambió la receta) y volver a
   ejecutar.
2. `AttributeError: module 'aire_lab' has no attribute …`, `KeyError` en una columna o un campo de la
   config: el código no es el de la rama del notebook. Mirar la línea `código:` de la celda de entorno:
   abrir la URL de la rama correcta, dejar `ACTUALIZAR_CODIGO = True` o *Entorno de ejecución →
   Desconectar y borrar el entorno* y ejecutar todo de nuevo.
3. `ValueError: mount failed` en la celda de entorno: nadie autorizó Drive a tiempo (la celda espera
   15 min; `drive.mount` trae 2 por defecto y así falló en la prueba) o se eligió otra cuenta (la de
   trabajo no tiene `aire-distribuido/`). Volver a *Ejecutar todo* y autorizar con la cuenta personal;
   si el cuadro quedó abierto de antes, recargar la pestaña primero.

Lo que se toca está arriba de cada notebook: `RAMA`, `ACTUALIZAR_CODIGO`, `RAIZ_DRIVE`; en el 01,
`AJUSTES` (para probar un cambio de `configs/limpieza.json` antes de llevarlo al JSON) y los umbrales de la
criba `U`; en el 02, `GRILLA_COMPLETA` (por defecto, completa con más de 4 CPU y reducida con menos),
`N_SUBMUESTRA`, `UMBRALES`, `K_DESDE_CODO`, `GASES_EN_FEATURES`; en el 03, las tolerancias y
`ESTACION_EPISODIO` (por defecto, la estación con más horas en el régimen más alto). Variables de
entorno: `AIRE_RAIZ` (raíz de las capas; en Colab, además, no monta Drive), `AIRE_BRONZE` (el CSV;
vacío = sin bronze, la auditoría de la limpieza queda OMITIDA y lo demás corre), `AIRE_FIGURAS` (dónde
van las figuras del informe), `AIRE_EPISODIO` (la estación de `episodio.png`).

### Por qué fallaba antes

Diagnóstico del 01/11 sobre el código y los datos de ese día (no son hipótesis):

1. **El clon se quedaba viejo y de otra rama.** La celda vieja reusaba `/content/aire-distribuido` si
   ya tenía `aire_lab.py` (`ACTUALIZAR_CODIGO = False` por defecto) y clonaba siempre la primera rama
   de `RAMAS` (`feature/spec-pc3`), sin importar de qué rama se había abierto el notebook; los errores
   de `git` se callaban (`-q`, `check=False`). Así pasó la vez anterior: el código clonado no tenía
   todavía el esquema real del CSV (la config buscaba `FECHA`, `PARAMETRO`, `VALOR`). Ahora: rama de la URL,
   `fetch` + `reset --hard` en cada corrida y la rama y el commit impresos.
2. **Drive estaba desfasado del repo.** El silver, el gold y los `reports/` de Go en Drive eran de las
   13:21, de la receta provisional (PM10, fracción fina, viento y humedad, log1p, k = 5), y
   `configs/kmeans.json` ya congelaba PM2.5 + PM10 con k = 3 (corridas de las 14:32 y 15:05 en local). El
   03 habría auditado salidas de otra receta (diferencias en cadena) y la §8 del 02 habría descrito el
   modelo viejo sin avisar: nada lo comprobaba. Se resincronizó el 01/11 y ahora la celda de entorno
   compara la receta y el sha256 del gold de `reports/` con `configs/kmeans.json` antes de correr.
3. **Faltaban insumos y el error no decía qué hacer.** Primero no había bronze en Drive y el 01 fallaba
   recién en la §1; la §8 del 02 «se corre sola» pero lee `reports/calibracion_resumen.csv`, que no se
   versiona (`*.csv` en `.gitignore`) y no estaba en Drive. Ahora cada notebook verifica sus insumos
   al empezar y da el `rclone copy` exacto.
4. **El 02 no cabe en una sesión de Colab estándar.** La grilla son 1 080 corridas, 6 176 s de CPU
   medidos (9 min con 14 núcleos): con las 2 CPU del entorno estándar, cerca de una hora. Ahora, con 4
   CPU o menos, corre reducida (el paso coprimo con 24 salta de 1 a 5: 46 742 filas; silueta sobre
   3 000) y lo dice: la grilla tarda 2.3 min con 2 CPU (medido en local con `taskset -c 0,1`).

5. **Visto al probar en Colab el 01/11**: cada notebook abre su propio entorno y cada entorno pide
   autorizar Drive; `drive.mount` espera 2 min por defecto, así que con *Ejecutar todo* sin mirar la
   pantalla la celda de entorno moría con `ValueError: mount failed` y no corría nada más. Ahora espera
   15 min. Y el reloj de Colab está en UTC: los reportes salían fechados 5 h adelante (el `eda_criba.md`
   de la prueba dice 04:13 del 02/11 por las 23:13 del 01/11); ahora la celda pone la hora de Lima.

Descartadas al medir: **dependencias** (Colab trae todo lo que se importa; `pyproj` no se usa, la
conversión UTM está en `aire_lab`), **memoria** (pico de 3.3 GB en el 01 y 2.4 GB en el 03 con pandas
2.2.2, como en Colab) y **figuras ocultas** (todas pasan por `guardar()`, que las guarda y hace
`plt.show()`). El 03 sí escribía y borraba en Drive un silver temporal de 17 MB para el sha256; ahora
usa el disco local.

## Correrlos en local contra Go

```bash
# 1. Go sobre la muestra (o sobre el bronze real): salidas en una raíz aparte
R=/tmp/aire
go run ./cmd/cargar -entrada data/sample/oefa_muestra.csv -silver $R/data/silver/oefa_ancho.csv \
   -gold $R/data/gold/oefa_features.csv -reports $R/reports -workers 1,2,4 -bench 3
go run ./cmd/entrenar -gold $R/data/gold/oefa_features.csv -estandarizacion $R/reports/estandarizacion.json \
   -salida $R/reports/kmeans.json -asignaciones $R/data/gold/asignaciones.csv \
   -regimenes $R/reports/regimenes.json -centroides $R/reports/centroides_iniciales.json \
   -codo 2:8 -codo-salida $R/reports/codo.json
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
sintético en el **formato real de OEFA** que describe `configs/limpieza.json` (36 columnas, `;`, CRLF,
una columna por parámetro; días completos en bloques de 24 filas sin hora y días con huecos con la hora
explícita; 6 estaciones `SIN-` en 3 departamentos, cinco regímenes plantados, huecos tras los episodios,
suciedad de cada regla de la cascada) y corre `cmd/cargar` y `cmd/entrenar` sobre él, con todas las
salidas (también `centroides_iniciales.json`) dentro de la raíz que se le pasa. El script no imita a
Go: con `--go`, las salidas son las de Go (la variable `GO` elige el binario).

Dependencias: `pandas`, `numpy`, `matplotlib`, `scikit-learn`; `scipy` (asignación húngara; sin
ella, fuerza bruta para k chico) y `joblib` (sin ella la grilla corre en serie). Colab las trae; la
celda de entorno instala lo que falte. Probado de punta a punta con Python 3.13 + pandas 3.1 +
scikit-learn 1.9, con Python 3.12 + pandas 2.2.2 + numpy 2.0.2 + scikit-learn 1.6.1 +
matplotlib 3.10.0 y, en Colab el 01/11, con Python 3.13.18 + pandas 2.2.3 + numpy 2.1.3 +
scikit-learn 1.6.1 + matplotlib 3.10.0.

## Archivos

| Archivo | Qué es |
|---|---|
| `aire_lab.py` | Código compartido. El prototipo de limpieza replica `datos/bloque.go` (cascada), `datos/recolector.go` (regla 5), `datos/reporte.go` (esquema de `limpieza.json`), `datos/salida.go` (silver byte a byte, `FormatFloat 'g' -1`), `datos/estandarizar.go` (gold) y `kmeans/regimenes.go` (perfil y ECA). Además: criba, UTM → lat/lon, huecos, ECA, métricas, estilo de figuras y lo que usa la celda de entorno (`describir_entorno`, `verificar_insumos`, los `rclone copy` de cada insumo). |
| `sintetico.py` | Bronze sintético en el formato real de OEFA y, con `--go`, las salidas del sistema real. |
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

- **Decisión del equipo (§8 del 02)**: la regla automática eligió `F2-M0-T1-N2` con k = 4 (PM10 y
  fracción fina por estación); el equipo congeló `F1-M0-T1-N1` con k = 3 (PM2.5 y PM10 absolutos, sin
  log), que está bajo el codo de su variante pero cumple los otros criterios aplicados y es un solo
  K-Means de niveles de aire. La §8 lo compara con las cifras de `calibracion_resumen.csv`, describe
  los regímenes de Go con la fracción fina y la meteorología (medias y anomalía dentro de cada estación)
  y deja `kmeans_propuesto.json` y `centroides_sklearn.json` con la receta final. Se corre sola: celda
  de entorno, importaciones y las celdas de la §8.

`sintetico.py` escribe el formato real (el que lee el cargador de Go). Con `--dias 60 --go`, los
descartes por regla de Go coinciden con lo que el script siembra y cada celda del silver coincide con
la verdad del generador, en valor y en hora (convención `fin`). Para probar contra Go están además las
dos muestras de `data/sample/`.

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
