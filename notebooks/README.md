# notebooks/ — laboratorio en Python (Colab)

**Python es laboratorio, no sistema.** La carga, la limpieza, la estandarización y el K-Means del
proyecto son Go (`cmd/cargar`, `cmd/entrenar`), como exige el enunciado. Estos notebooks solo
**exploran** el dato, **calibran** la receta que Go va a leer, **auditan** lo que Go produce y
**dibujan** las figuras del informe. Ninguna salida de aquí entra al sistema salvo la receta, y esa
la copia una persona a `configs/kmeans.json` después de leer la tabla de decisión.

## Orden

| # | Notebook | Cuándo | Qué hace | Qué produce |
|---|---|---|---|---|
| 1 | `01_eda_criba.ipynb` | lun 02/11, con el bronze en Drive | Primer MB y sha256 del bronze; mapeo de columnas y parámetros; prototipo de las reglas de limpieza 1–8 con conteos; **cada medición de la criba** de preguntas (Q1–Q9) | `reports/eda_criba.md` (veredicto por candidata y la cifra que lo sostiene), `eda_completitud.csv`, `eda_estaciones.csv`, `eda_limpieza_prototipo.json`, `eda_bronze.json`, `figuras_eda/`; `data/silver/oefa_ancho_prototipo.csv` |
| 2 | `02_calibracion.ipynb` | mar 03/11 | Grilla de la spec de calibración §3 (24 variantes × k 2–10 × 5 semillas) y regla de elección en su orden | `reports/calibracion.csv` (una fila por corrida), `calibracion_resumen.csv`, `calibracion_decision.md` (tabla de decisión), `kmeans_propuesto.json`, `centroides_sklearn.json`, `figuras_calibracion/` |
| 3 | `03_auditoria_figuras.ipynb` | jue 05/11, cuando Go ya corrió | Recalcula silver y gold desde el bronze y los compara con Go; audita centroides, asignaciones y `regimenes.json`; dibuja las figuras | `reports/auditoria.md` y `.json`; figuras PNG (150 dpi): `fig_perfiles_regimenes`, `fig_regimen_hora`, `fig_regimen_estacion`, `fig_episodio`, `fig_cobertura_departamento`, `fig_codo` |

El 03 no se cae si Go todavía no corrió: cada comprobación queda OMITIDA y las figuras usan los
regímenes de scikit-learn, con una nota que lo dice. Estados de la auditoría: **OK**; **DIFERENCIA**
(bug en Go o en Python); **AVISO** (no es un bug pero hay que mirarlo, p. ej. Go quedó en otro
mínimo local); **OMITIDO**.

Specs que implementan: `docs/specs/2026-10-31-preguntas-candidatas.md` (01),
`2026-10-31-kmeans-calibracion-design.md` §1–§4 (02 y 03) y `2026-10-31-pc3-design.md` §4 y §9.

## Abrirlos en Colab

1. **Drive**: las capas viven en `MyDrive/aire-distribuido/`:
   ```
   aire-distribuido/
     data/bronze/<CSV de OEFA>.csv     # el 01 toma el CSV más grande de la carpeta
     data/silver/  data/gold/          # los escribe Go (y el 01 el silver prototipo)
     reports/                          # Go y Colab; aquí quedan también las figuras del 03
     configs/                          # opcional: si existe, pisa a la del repo
   ```
2. **Entorno de ejecución**: *Entorno de ejecución → Cambiar tipo de entorno* → CPU con **RAM alta**.
   El 01 y el 03 leen el CSV de 1.3 GB entero (pandas con categorías: unos pocos GB); el 02 usa
   todos los núcleos con `joblib`. No hace falta GPU.
3. **Abrir**: *Archivo → Abrir cuaderno → GitHub* → `R0SEWT/aire-distribuido`, rama `develop`
   (o `feature/notebooks` mientras no se fusione) → `notebooks/0X_….ipynb`.
4. **Ejecutar todo**. La celda de entorno monta Drive (pide permiso una vez) y clona el repo en
   `/content/aire-distribuido` para importar `aire_lab.py` y leer `configs/`. Prueba las ramas de
   `RAMAS` en orden; `ACTUALIZAR_CODIGO = True` vuelve a clonar si cambió `aire_lab.py`.

Lo único que se toca está arriba de cada notebook: `RAIZ_DRIVE`, `RAMAS`; en el 01, `BRONZE` y
`AJUSTES` (mapeo de columnas para probar antes de llevarlo a `configs/limpieza.json`); en el 01, los
umbrales de la criba `U`; en el 02, `N_SUBMUESTRA`, `UMBRALES`, `K_DESDE_CODO`, `GASES_EN_FEATURES`;
en el 03, las tolerancias y `ESTACION_EPISODIO`.

**Tiempos** (medidos con `sintetico.py --dias 1500`: 1.5 M filas largas, 210 000 anchas). El 02
hace 1 080 corridas de unos 6 s cada una con ~145 000 filas por variante: ≈ 105 min de CPU, es decir
unos 15–20 min con 8 núcleos y más de una hora con 2. Con `N_SUBMUESTRA = 100_000` tarda
aproximadamente la mitad. El 01 tardó 28 s y el 03 55 s (≈ 1.4 GB de RAM); el CSV real tiene varias
veces más filas largas, por eso la RAM alta.

## Correrlos en local (y probarlos con datos sintéticos)

```bash
cd notebooks
python sintetico.py /tmp/aire-lab        # bronze sintético + salidas «de Go» con semilla fija
for nb in 01_eda_criba 02_calibracion 03_auditoria_figuras; do
  AIRE_RAIZ=/tmp/aire-lab AIRE_FIGURAS=/tmp/aire-lab/figuras \
    jupyter nbconvert --to notebook --execute $nb.ipynb --output-dir /tmp/aire-lab/ejecutados
done
```

- `AIRE_RAIZ` es la raíz de las capas. Sin ella, fuera de Colab la raíz es **el repo** (sus
  `data/` están en `.gitignore`; `reports/` es la carpeta versionada de salidas chicas).
- `AIRE_FIGURAS` es dónde deja el 03 las figuras. Sin ella: `docs/informe/generado/figuras/` en
  local y `reports/figuras/` de Drive en Colab.
- Si la raíz tiene `SINTETICO.txt`, cada figura sale con la marca de agua «DATOS SINTÉTICOS» y cada
  reporte lo avisa: una figura de prueba nunca puede pasar por real. Las figuras de ejemplo que hay
  hoy en `docs/informe/generado/figuras/` son de ese tipo y se reemplazan corriendo el 03 el jueves.

`sintetico.py` genera 6 estaciones en 3 departamentos (dos a menos de 15 km en cada uno), 60 días
de horas, 8 parámetros, 5 regímenes plantados con persistencia, huecos (en una estación, justo
después de los episodios), duplicados, centinelas, fechas rotas, CO en mg/m³, PM2.5 > PM10 en
algunas horas y decimales con coma en una estación; y escribe el silver, el gold, las asignaciones y
los reports **con una implementación independiente de `aire_lab`**, para que la auditoría compare
dos implementaciones de verdad. `--dias 1500` da un tamaño parecido al real (≈ 210 000 filas anchas).

Dependencias: `pandas`, `numpy`, `matplotlib`, `scikit-learn`; `scipy` (asignación húngara; sin
ella hay fuerza bruta para k chico) y `joblib` (sin ella la grilla corre en serie). Colab las trae;
la primera celda instala lo que falte. Probado de punta a punta con Python 3.13 + pandas 3.1 +
scikit-learn 1.9 y con Python 3.12 + pandas 2.2.2 + numpy 2.0.2 + scikit-learn 1.6.1 +
matplotlib 3.10.0 (lo de Colab).

## Archivos

| Archivo | Qué es |
|---|---|
| `aire_lab.py` | Código compartido: configuración del bronze (`CONFIG_DEFECTO`, alias de columnas), lectura por categorías, reglas 1–8, contrato de silver y gold, features de la receta, UTM → lat/lon, huecos, ECA y promedio móvil, métricas, estilo de figuras. |
| `sintetico.py` | Datos sintéticos y salidas «de Go» para probar los tres notebooks. |
| `0X_*.ipynb` | Los notebooks, **guardados sin salidas**. |

## Contrato con Go que asumen

- **Mapeo del bronze**: `configs/limpieza.json` si existe (se fusiona sobre `aire_lab.CONFIG_DEFECTO`;
  `parametros` se reemplaza entero). Claves: `columnas` (`estacion, fecha, hora, parametro, valor,
  periodo, unidad, departamento, provincia, distrito, ubigeo, este, norte, zona, datum`),
  `delimitador`, `codificacion`, `decimal`, `formato_fecha`, `dia_primero`, `convencion_hora`,
  `fecha_minima`, `fecha_maxima`, `periodos_horarios`, `parametros` (real → canónico),
  `centinelas`, `negativos_permitidos`, `rangos`, `unidades`.
- **Orden de las reglas** (para que los conteos coincidan): 1 periodo → 1 parámetro → 2 fecha →
  3 valor → 6 unidad (normaliza o descarta) → 4 rango en unidad canónica → 5 duplicados (se queda el
  primero del archivo) → pivot → 8 completitud. Cada fila cuenta en la primera regla que incumple.
- **silver** `data/silver/oefa_ancho.csv`: `estacion,fecha_hora,<parámetros canónicos en orden
  alfabético>`, coma, vacío = faltante, `fecha_hora` RFC3339 con `-05:00`, ordenado por estación
  (orden de bytes) y fecha-hora.
- **gold** `data/gold/oefa_features.csv`: `estacion,fecha_hora,<features de la receta en su orden>`;
  `fraccion_fina = pm2_5 / pm10` recortada a [0, 1] (faltante si PM10 ≤ 0); log1p donde diga la
  receta; z-score con **desviación poblacional** (n, no n − 1); solo filas completas.
- **reports de Go** (claves que lee el 03; acepta algunos alias): `limpieza.json` (`filas_largas`
  {`leidas, procesadas, descartadas`}, `reglas` {`r1_periodo, r1_parametro, r2_fecha, r3_valor,
  r6_unidad, r4_rango, r5_duplicado`}, `marcas`, `filas_anchas` {`total`}); `estandarizacion.json`
  (`features, log1p, nivel, n, media, desv`; con nivel `estacion`, `media` y `desv` son
  diccionarios por estación); `kmeans.json` (`centroides_estandarizados`,
  `centroides_desestandarizados`, `tamanos`, `inercia`); `data/gold/asignaciones.csv`
  (`estacion,fecha_hora,regimen`); `regimenes.json` (`regimenes`: lista con `regimen, n, perfil,
  excede_eca, por_hora, por_estacion`; `perfil` = media del silver de las horas del régimen).

## Lo que hay que confirmar el lunes con el CSV real

Todo está en una celda del 01 (`AJUSTES`) y luego en `configs/limpieza.json`:

- nombres reales de columnas (los resuelve por alias, pero hay que mirarlos) y si fecha y hora vienen
  juntas o separadas (`columnas.hora = None` si vienen juntas);
- formato de fecha (`dia_primero`, `formato_fecha`) y convención horaria (`01:00` es inicio o fin de
  la hora; `24:00` se lee como las 00:00 del día siguiente);
- nombres reales de los parámetros (los que no tienen canónico se descartan y se listan) y los
  valores de `PERIODO` que son horarios;
- unidades por parámetro, centinelas y rangos físicos (reglas 3, 4 y 6);
- delimitador, codificación y decimal (el 01 los infiere del primer MB y avisa si no coinciden).

## Antes de commitear

Los notebooks se guardan **sin salidas** y los datos no se versionan:

```bash
jupyter nbconvert --clear-output --inplace notebooks/*.ipynb
```
