# Auditoría de las salidas de Go

> Generado por `notebooks/03_auditoria_figuras.ipynb` el 2026-11-01 14:05. Bronze: `1a_Vigilancia y Seguimiento ambiental en la calidad del aire.csv`. Receta: `configs/kmeans.json`. Config de limpieza: `configs/limpieza.json`. Tolerancias: gold 1e-09, estandarización 1e-09, perfiles 1e-09, scikit-learn 0.001; silver byte a byte.

**33 OK, 0 con diferencias, 1 avisos, 0 omitidas.** DIFERENCIA = bug en Go o en Python; AVISO = no es un bug pero hay que mirarlo; OMITIDO = falta la salida de Go.

| chequeo | estado | detalle |
|---|---|---|
| cabecera, BOM y delimitador | OK | go BOM=False delim=';' · python BOM=False delim=';' |
| filas largas (leídas, procesadas, descartadas) | OK | leidas py 2,883,877 go 2,883,877 · procesadas py 1,423,958 go 1,423,958 · descartadas py 1,459,919 go 1,459,919 |
| descartadas por regla | OK | las 17 reglas de Go iguales |
| marcadas por regla | OK | go {'duplicado_valor_distinto': 0, 'hora_inferida': 1323386, 'hora_no_en_punto': 0, 'unidad_convertida': 151734} |
| fuera de rango por parámetro | OK | python {'presion': 50472, 'o3': 744, 'precipitacion': 2, 'temperatura': 1} · go {'o3': 744, 'precipitacion': 2, 'presion': 50472, 'temperatura': 1} |
| filas anchas (total, PM2.5 > PM10, hora inferida) | OK | python total 233,707, pm2_5>pm10 2 · go total 233707, pm2_5>pm10 2 · hora inferida py 217248 go 217248 |
| parámetros presentes | OK | ['co', 'h2s', 'humedad', 'o3', 'pm10', 'pm2_5', 'precipitacion', 'presion', 'radiacion', 'so2', 'temperatura', 'viento_direccion', 'viento_velocidad'] |
| estaciones (leídas, procesadas, anchas, fechas, ubicación) | OK | 33 estaciones iguales |
| valores vistos (parámetros, periodos, unidades) | OK | iguales |
| silver byte a byte | OK | idéntico (sha256 3e84071aa856327e…) |
| cabecera del gold | OK | go ['estacion', 'fecha_hora', 'pm10', 'fraccion_fina', 'viento_velocidad', 'humedad'] |
| filas del gold (regla 8) | OK | python 41,991 · go 41,991 · 0 claves en un solo lado |
| valores del gold | OK | diferencia máxima 1.69e-14 (tolerancia 1e-09); 0 filas fuera |
| gold desde el silver de Go | OK | 0 claves en un solo lado; diferencia máxima 1.69e-14 |
| completas para el gold (total y por estación) | OK | python 41,991 · go 41991 |
| receta en estandarizacion.json | OK | go features=['pm10', 'fraccion_fina', 'viento_velocidad', 'humedad'] log1p=['pm10'] nivel=absoluto ddof=0 |
| n de la estandarización | OK | python 41,991 · go 41991 |
| medias de la estandarización | OK | diferencia relativa máxima 3.26e-15 |
| desviaciones de la estandarización | OK | diferencia relativa máxima 8.88e-16 |
| el modelo se entrenó con este gold | OK | gold 1a93989d9af8… · kmeans 1a93989d9af8…, regimenes 1a93989d9af8…, codo 1a93989d9af8… |
| receta de kmeans.json = configs/kmeans.json | OK | iguales |
| forma de los centroides | OK | go (5, 4) · k efectivo 5, D 4 |
| asignaciones | OK | 0 filas con otro régimen · 0 claves en un solo lado de 41,991 |
| tamaños | OK | python [6227, 7004, 7845, 7647, 13268] · go [6227, 7004, 7845, 7647, 13268] |
| inercia con los centroides finales | OK | python 59232.845147643 · go 59232.845147643 (rel 6.1e-16) |
| la inercia no crece entre iteraciones | OK | 21 iteraciones, paro «tolerancia», 0 subidas |
| punto fijo de Lloyd | OK | máx \|media de las filas − centroide\| = 7.11e-15 (umbral de paro de Go 2.6e-06, paro «tolerancia») |
| scikit-learn desde los centroides de Go | OK | se mueven 2.22e-15 en 2 iteraciones |
| centroides frente a scikit-learn | AVISO | máx \|Δ\| tras emparejar 8.17e-03 (tolerancia 0.001); ARI 0.9962; inercia sklearn/go − 1 = +0.00%. Otro mínimo local de igual calidad: no es un bug |
| centroides des-estandarizados | OK | diferencia relativa máxima 0.00e+00 |
| regimenes.json (perfil, PM medio, ECA, horas, estaciones) | OK | iguales |
| codo.json frente a scikit-learn | OK | inercia de Go / mejor de scikit-learn: máximo 1.000 en k = 3 |
| cargador: mismo silver y gold para todo W | OK | identicos=True; 5 valores de W; sha del silver = el auditado |
| K-Means: idéntico bit a bit para todo W | OK | convergencia chunk 1000: True; convergencia chunk 10000: True; fijas chunk 1000: True; fijas chunk 10000: True |
