# Calibración del K-Means sobre las lecturas de OEFA: qué tiene que decir un régimen

> **Borrador para pensar** (2026-10-31). Acompaña a la [spec de la PC3](2026-10-31-pc3-design.md) y
> reemplaza su §4.3 y §6.1 en lo que toca a features y k. Lo marcado **[abierta]** lo decide Rody; lo
> marcado **[decisión mía]** se puede revertir. Se ejecuta en Colab sobre Drive (capa silver), y la
> receta que salga se congela en `configs/kmeans.json` para que Go la reproduzca.

## 1. El problema de fondo

Un K-Means sobre lecturas horarias de calidad del aire sale "bien" con mucha facilidad y no dice
nada: con PM2.5 y PM10 crudas y estandarizadas, los clusters son rodajas de una sola dimensión
(«bajo, medio, alto») y el régimen más contaminado es un puñado de puntos de la cola. Con
meteorología, los clusters son «día y noche» o «verano e invierno». Con 34 estaciones de ciudades
distintas, los clusters son las estaciones. En los tres casos el codo y la silueta se ven decentes.

Un régimen es **informativo** si, además de agrupar bien, cumple cuatro cosas que se pueden medir:

| Criterio | Qué mide | Cómo se mide |
|---|---|---|
| **Separa frente al ECA** | Los regímenes que exceden el ECA concentran las lecturas que de verdad lo exceden | Pureza y recall de la etiqueta «PM2.5 > 50 o PM10 > 100 µg/m³» por régimen; una curva de enriquecimiento |
| **No es la estación ni la hora disfrazadas** | Un régimen que es 100 % una estación es la estación; uno que es 100 % la noche es la noche | Información mutua normalizada (NMI) entre régimen y estación, hora, mes. Alta no es malo por sí sola (los episodios *tienen* hora), pero si la NMI con estación supera a la que hay con la etiqueta ECA, el modelo aprendió geografía |
| **Estable** | Otra semilla u otra muestra dan la misma partición | ARI entre 5 semillas, y entre remuestreos por **bloques de días** (las horas consecutivas no son independientes) |
| **Interpretable** | Cada centroide se distingue del resto en al menos una variable por más de una desviación | Tabla de perfiles des-estandarizados y la distancia mínima entre centroides |

El quinto criterio, **útil para avisar** (la anticipación de §6.3 de la spec), se mide en el TB2 con
la receta ya congelada.

## 2. Decisiones de diseño que cambian lo que sale

### 2.1 Colas largas: log antes del z-score

PM2.5 y PM10 son aproximadamente log-normales. Con el z-score sobre el valor crudo, una lectura de
500 µg/m³ pesa más que mil lecturas normales y el K-Means pone un centroide ahí. Con `log1p` antes
de estandarizar (lo mismo que el TP hizo con duración y distancia), la distancia euclídea compara
razones y no diferencias: pasar de 10 a 20 pesa igual que de 100 a 200. **[decisión mía]**: log1p
en PM y gases; la meteorología se estandariza sin log. Se compara contra el crudo en la grilla.

### 2.2 PM2.5 y PM10 van juntas: usar el nivel y la fracción fina

PM2.5 está contenida en PM10 y las dos se mueven juntas (correlación alta); como features
separadas duplican el peso del particulado. La alternativa informativa es **PM10 (nivel) y la
fracción fina PM2.5/PM10**: la fracción separa **combustión** (fracción alta, tráfico, quema) de
**polvo** (fracción baja, suelo seco, minería, viento). Es la diferencia entre el aire de una
avenida en Lima de noche y el de Arequipa con viento, y es lo que un régimen debería distinguir.
La fracción no se transforma con log; se recorta a [0, 1] y se cuenta cuántas lecturas la violan
(PM2.5 > PM10 es error de medición). **[abierta]**: F1 = {PM2.5, PM10} frente a F2 = {PM10,
fracción fina}; la grilla lo decide con datos.

### 2.3 Absoluto o relativo a la estación

Con todas las estaciones juntas y en escala absoluta, la varianza **entre** estaciones (Lima frente
a La Oroya frente a Pucallpa) domina y los clusters tienden a ser estaciones. Hay dos lecturas
legítimas y no se puede elegir una sin medirlo:

- **Absoluto (pooled)**: un régimen es un nivel de aire, el mismo en todo el país. Es lo que el ECA
  necesita: 50 µg/m³ son 50 en cualquier lado.
- **Relativo (por estación)**: se estandariza cada estación con su propia media y desviación; un
  régimen es un **episodio** respecto de lo normal de ese lugar. Avisa mejor («hoy está raro aquí»)
  pero pierde el nivel absoluto.

**[decisión mía]**: se corren los dos. Si el absoluto da NMI con estación mayor que con la etiqueta
ECA, el modelo de regímenes es el relativo y **la etiqueta ECA se calcula aparte, en absoluto**, sobre
cada lectura. Las dos cosas van al informe con esa explicación: el régimen dice si es un episodio, el
ECA dice si es peligroso.

### 2.4 Meteorología: feature o descriptor

Temperatura, humedad y presión tienen ciclo diario y estacional fuertes; como features arrastran el
clustering hacia «hora del día» y «época del año». Viento y humedad, en cambio, son **mecanismo**:
poco viento y alta humedad de noche es la inversión térmica que acumula PM. Opciones en la grilla:

- M0: solo contaminantes; la meteorología se describe por régimen después (media de viento, humedad
  y temperatura por cluster). Nunca puede volver al clustering «día y noche».
- M1: contaminantes + viento + humedad.
- M2: M1 + temperatura + presión.

**[decisión mía]**: la hipótesis es que M1 es el punto justo y M2 se vuelve meteorología. La hora,
la estación y el mes **no son features** en ninguna variante: se usan después para decir dónde y
cuándo aparece cada régimen, y para medir la NMI del criterio 2.

### 2.5 Gases

NO2, SO2, CO y O3 entran solo si los reporta la gran mayoría de las estaciones (tabla de completitud
del EDA). Si entran, van con log1p y el mismo tratamiento que PM. Si solo los reportan pocas
estaciones, quedan fuera del modelo y entran como descriptores donde existan.

### 2.6 Autocorrelación y muestras

Las lecturas horarias de una estación están fuertemente autocorrelacionadas. Consecuencias:

- La submuestra para el codo y la silueta es **sistemática** (cada n-ésima fila ancha ordenada por
  estación y fecha-hora), como en el TP, nunca «los primeros N».
- La estabilidad se mide con remuestreo por **bloques de días**, no por filas: remuestrear horas
  sueltas sobreestima la estabilidad.
- La silueta se calcula sobre 10 000 puntos (es O(n²)).

### 2.7 Desbalance: lo que importa es minoría

Los episodios que exceden el ECA son una fracción pequeña de las horas. El K-Means minimiza inercia,
así que parte la masa de lecturas normales y puede dejar los episodios dentro de un cluster grande.
Señales de alarma en la grilla: el régimen «excede» con menos del 1 % de las filas, o un recall de
la etiqueta ECA bajo con k pequeño. El remedio honesto es el log (§2.1), que comprime la masa y
expande la cola, y un k un poco mayor; no se recomiendan pesos ad hoc.

## 3. Grilla de calibración (Colab, sobre silver)

| Eje | Valores |
|---|---|
| Features | F1 {PM2.5, PM10} · F2 {PM10, fracción fina} |
| Meteorología | M0 ninguna · M1 viento, humedad · M2 M1 + temperatura, presión |
| Transformación | T1 crudo · T2 log1p en contaminantes |
| Nivel | N1 absoluto · N2 por estación |
| k | 2 … 10 |
| Semillas | 5 (k-means++) |

24 variantes × 9 valores de k × 5 semillas = 1 080 corridas de scikit-learn sobre una submuestra
sistemática de unas 200 000 filas anchas y ≤ 7 dimensiones: minutos en Colab. Para cada corrida se
guardan inercia, silueta (10 000 puntos), ARI entre semillas, ARI de remuestreo por días, pureza y
recall de la etiqueta ECA, NMI con estación, hora y mes, tamaño del cluster más chico y distancia
mínima entre centroides. Todo a un `calibracion.csv` en `reports/` y a Drive.

**Regla de elección**, en este orden: (1) estable (ARI > 0,8 entre semillas y > 0,7 por días);
(2) separa el ECA (pureza del régimen «excede» > 0,6 y recall > 0,5); (3) no es geografía (NMI
con estación < NMI con la etiqueta ECA, o si no, se pasa a N2); (4) interpretable (ningún cluster
bajo el 1 %, perfiles distinguibles); (5) el k más chico que cumpla lo anterior, mirando el codo.
Los umbrales son puntos de partida y se reportan tal cual; si ninguna variante los cumple, se dice.

Salida del notebook: **una tabla de decisión** (variante elegida, por qué, qué perdió frente a la
segunda) y los centroides de scikit-learn como referencia para el test de equivalencia con Go.

## 4. De la calibración al sistema

1. La receta se congela en `configs/kmeans.json`: features, transformación, nivel, k, semilla,
   tolerancia, máximo de iteraciones. **Go no decide nada de esto: lo lee.**
2. `cmd/cargar` (parte A) produce **silver** (tabla ancha limpia) y **gold** (features de la receta,
   estandarizadas) y escribe en `reports/` las medias y desviaciones usadas, como hizo el TP.
3. `cmd/entrenar` (parte B) corre sobre gold y escribe centroides, asignaciones y perfiles.
4. **Auditoría en Colab** (patrón del TP: Polars contra DuckDB): el notebook recalcula gold desde
   silver con la misma receta y compara con el gold de Go (tolerancia 1e-9 tras el z-score), y
   compara los centroides de Go con los de scikit-learn con correspondencia de etiquetas y
   tolerancia. Si discrepan, hay un bug en uno de los dos.
5. Los perfiles, el heatmap régimen × hora, régimen × estación y la figura de un episodio con los
   regímenes coloreados (la figura estrella del informe) salen de Colab sobre las salidas de Go.

## 5. Capas (medallón) y dónde corre cada cosa

| Capa | Qué | Quién la produce | Dónde vive |
|---|---|---|---|
| bronze | CSV largo de OEFA tal cual, con sha256 y fecha | Descarga desde Colab o desde el navegador | Drive `aire-distribuido/data/bronze/` |
| silver | Tabla ancha limpia (estación, fecha-hora, parámetros), con los conteos por regla | **Go** (`cmd/cargar`); un prototipo en Polars durante el EDA, que luego sirve de auditoría | Drive `data/silver/`, local y gorgo |
| gold | Features de la receta, estandarizadas | **Go** (`cmd/cargar`) | Drive `data/gold/` |
| reports | Conteos de limpieza, medias y desviaciones, benchmarks, calibración, centroides | Go y Colab | `reports/` en el repo (versionado, chico) y Drive |

Colab es el **laboratorio**: EDA, calibración, auditoría y figuras. Go es el **sistema**: el enunciado
exige que la carga, la limpieza y el entrenamiento sean en Go, y así es; el informe lo dice con esas
palabras. La laptop y gorgo sincronizan con Drive por `rclone`; los notebooks montan Drive.

## 6. Calendario dentro de la PC3

| Fecha | Qué |
|---|---|
| lun 02/11 | Bronze en Drive, verificado. Notebook 01 (EDA): completitud estación × parámetro, distribuciones, prototipo de silver en Polars |
| mar 03/11 | Notebook 02 (calibración): la grilla de §3 sobre el silver prototipo; tabla de decisión; `configs/kmeans.json` |
| mié 04/11 | Go: cargador produce silver y gold oficiales; auditoría en el notebook 01 |
| jue 05/11 | Go: K-Means con la receta; auditoría de centroides; figuras en el notebook 03 |
| vie 06/11 | Informe |

Si la calibración no cierra el martes, Go arranca el miércoles con la receta provisional (F2, M1,
T2, N1, k = 5) y se re-corre el jueves con la definitiva: el código no cambia, solo el `.toml`.

## 7. Decisiones

**Cerradas el 31/10**: Colab y Drive con capas medallón; Go produce silver y gold; la calibración
va en Python sobre una submuestra y Go reproduce la receta; hora, estación y mes nunca son features.

**Abiertas**

1. F1 frente a F2 (§2.2): lo decide la grilla, pero si Rody prefiere mantener PM2.5 y PM10 como
   features por legibilidad del informe, F2 queda como ablación.
2. Umbrales de la regla de elección (§3): puntos de partida, se ajustan viendo la tabla.
3. Si el absoluto sale «geografía» (§2.3): aceptar el modelo relativo con la etiqueta ECA aparte.
