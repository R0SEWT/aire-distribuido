# Auditoría de las salidas de Go

> Generado por `notebooks/03_auditoria_figuras.ipynb` el 2026-11-01 15:00. Bronze: `1a_Vigilancia y Seguimiento ambiental en la calidad del aire.csv`. Receta (`configs/kmeans.json`): pm2_5, pm10; log1p en ninguna; nivel absoluto; k = 3. Config de limpieza: `configs/limpieza.json`. Tolerancias: gold 1e-09, estandarización 1e-09, perfiles 1e-09, scikit-learn 0.001; silver byte a byte.

**37 OK, 0 con diferencias, 1 avisos, 0 omitidas.** DIFERENCIA = bug en Go o en Python; AVISO = no es un bug pero hay que mirarlo; OMITIDO = falta la salida de Go.

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
| cabecera del gold | OK | go ['estacion', 'fecha_hora', 'pm2_5', 'pm10'] |
| filas del gold (regla 8) | OK | python 64,236 · go 64,236 · 0 claves en un solo lado |
| valores del gold | OK | diferencia máxima 2.13e-14 (tolerancia 1e-09); 0 filas fuera |
| gold desde el silver de Go | OK | 0 claves en un solo lado; diferencia máxima 2.13e-14 |
| completas para el gold (total y por estación) | OK | python 64,236 · go 64236 |
| receta en estandarizacion.json | OK | go features=['pm2_5', 'pm10'] log1p=[] nivel=absoluto ddof=0 |
| n de la estandarización | OK | python 64,236 · go 64236 |
| medias de la estandarización | OK | diferencia relativa máxima 2.53e-16 |
| desviaciones de la estandarización | OK | diferencia relativa máxima 5.68e-16 |
| el modelo se entrenó con este gold | OK | gold ce1fd2a036e1… · kmeans ce1fd2a036e1…, regimenes ce1fd2a036e1…, codo ce1fd2a036e1… |
| receta de kmeans.json = configs/kmeans.json | OK | iguales |
| forma de los centroides | OK | go (3, 2) · k efectivo 3, D 2 |
| asignaciones | OK | 0 filas con otro régimen · 0 claves en un solo lado de 64,236 |
| tamaños | OK | python [47070, 15651, 1515] · go [47070, 15651, 1515] |
| inercia con los centroides finales | OK | python 49658.357931554 · go 49658.357931554 (rel 1.5e-16) |
| la inercia no crece entre iteraciones | OK | 24 iteraciones, paro «tolerancia», 0 subidas |
| punto fijo de Lloyd | OK | máx \|media de las filas − centroide\| = 1.10e-14 (umbral de paro de Go 5.2e-06, paro «tolerancia») |
| scikit-learn desde los centroides de Go | OK | se mueven 5.55e-16 en 2 iteraciones |
| centroides frente a scikit-learn | OK | máx \|Δ\| tras emparejar 5.55e-16 (tolerancia 0.001); ARI 1.0000; inercia sklearn/go − 1 = -0.00% |
| centroides des-estandarizados | OK | diferencia relativa máxima 0.00e+00 |
| arranque elegido entre las semillas | OK | 10 arranques (semillas 42–51); elegida la 42, inercia 49658.357932 = la menor; 6 semillas empatan bit a bit en ella; inercias distintas: [49658.357932, 49658.363217] |
| Lloyd desde los centroides iniciales de Go | OK | 10 de 10 arranques reproducen la inercia de Go (diferencia relativa máxima 1.5e-16); semilla elegida: máx \|Δ centroide\| 5.6e-16 |
| regimenes.json (perfil, PM medio, ECA, horas, estaciones) | OK | iguales; convención de la hora «fin» |
| codo.json frente a scikit-learn | OK | inercia de Go / mejor de scikit-learn: máximo 1.000 en k = 2 |
| cargador: mismo silver y gold para todo W | OK | identicos=True; 5 valores de W; sha del silver = el auditado |
| cargador: el benchmark es de la receta auditada | AVISO | el benchmark del cargador (2026-11-01T13:15:42-05:00) corrió con otra receta: gold ≠ el auditado, configs/kmeans.json ≠ el actual; el silver es el mismo. No es un bug: sus tiempos son de esa corrida (bronze → silver → gold con la receta de entonces); conviene repetirlo con la receta final antes de citarlo |
| K-Means: idéntico bit a bit para todo W | OK | convergencia chunk 1000: True; convergencia chunk 10000: True; fijas chunk 1000: True; fijas chunk 10000: True |
| K-Means: el benchmark es del gold y la receta auditados | OK | reports/benchmark_kmeans.json: gold = el auditado; receta igual; inercia de «convergencia» con chunk 1000 frente a kmeans.json: diferencia relativa 0.0e+00 |
