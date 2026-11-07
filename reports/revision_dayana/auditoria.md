# Auditoría de las salidas de Go

> Generado por `notebooks/03_auditoria_figuras.ipynb` el 2026-11-07 10:30. Bronze: `1a_Vigilancia y Seguimiento ambiental en la calidad del aire.csv`. Receta (`configs/kmeans.json`): pm2_5, pm10; log1p en ninguna; nivel absoluto; k = 3. Config de limpieza: `configs/limpieza.json`. Tolerancias: gold 1e-09, estandarización 1e-09, perfiles 1e-09, scikit-learn 0.001; silver byte a byte.

**40 OK, 0 con diferencias, 0 avisos, 0 omitidas.** DIFERENCIA = bug en Go o en Python; AVISO = no es un bug pero hay que mirarlo; OMITIDO = falta la salida de Go.

| chequeo | estado | detalle |
|---|---|---|
| cabecera, BOM y delimitador | OK | go BOM=False delim=';' · python BOM=False delim=';' |
| filas largas (leídas, procesadas, descartadas) | OK | leidas py 2,883,877 go 2,883,877 · procesadas py 1,434,498 go 1,434,498 · descartadas py 1,449,379 go 1,449,379 |
| descartadas por regla | OK | las 19 reglas de Go iguales |
| marcadas por regla | OK | go {'duplicado_valor_distinto': 0, 'hora_inferida': 1333932, 'hora_no_en_punto': 0, 'ubicacion_corregida': 5296, 'unidad_convertida': 151524} |
| fuera de rango por parámetro | OK | python {'presion': 50472, 'o3': 744, 'precipitacion': 2, 'temperatura': 1} · go {'o3': 744, 'precipitacion': 2, 'presion': 50472, 'temperatura': 1} |
| día en cero, hora en cero y unidad convertida, por parámetro | OK | iguales: bloque_en_cero {'co': 107448, 'h2s': 137088, 'humedad': 42918, 'o3': 100944, 'pm10': 62400, 'pm2_5': 22368, 'radiacion': 480, 'so2': 168912, 'temperatura': 39822, 'viento_direccion': 40182, 'viento_velocidad': 40182}; cero_como_faltante {'co': 256, 'h2s': 1237, 'o3': 567, 'pm10': 3136, 'pm2_5': 415, 'so2': 3083} |
| horas explícitas, inferidas y del bloque físico | OK | hora_explicita py 100566 go 100566 · hora_inferida py 1333932 go 1333932 · hora_por_bloque_fisico py 19710 go 19710 |
| filas anchas (total, PM2.5 > PM10, hora inferida) | OK | python total 236,001, pm2_5>pm10 0 · go total 236001, pm2_5>pm10 0 · hora inferida py 219542 go 219542 |
| parámetros presentes | OK | ['co', 'h2s', 'humedad', 'o3', 'pm10', 'pm2_5', 'precipitacion', 'presion', 'radiacion', 'so2', 'temperatura', 'viento_direccion', 'viento_velocidad'] |
| estaciones (leídas, procesadas, anchas, fechas, ubicación con lat/lon y la corregida) | OK | 33 estaciones iguales; corregidas (valor del CSV): {'CA-HUAR-01': {'zona': '19'}} |
| valores vistos (parámetros, periodos, unidades) | OK | iguales |
| silver byte a byte | OK | idéntico (sha256 63b707f469a63baa…) |
| cabecera del gold | OK | go ['estacion', 'fecha_hora', 'pm2_5', 'pm10'] |
| filas del gold (regla 8) | OK | python 65,873 · go 65,873 · 0 claves en un solo lado |
| valores del gold | OK | diferencia máxima 1.42e-14 (tolerancia 1e-09); 0 filas fuera |
| gold desde el silver de Go | OK | 0 claves en un solo lado; diferencia máxima 1.42e-14 |
| completas para el gold (total y por estación) | OK | python 65,873 · go 65873 |
| receta en estandarizacion.json | OK | go features=['pm2_5', 'pm10'] log1p=[] nivel=absoluto ddof=0 |
| n de la estandarización | OK | python 65,873 · go 65873 |
| medias de la estandarización | OK | diferencia relativa máxima 2.52e-16 |
| desviaciones de la estandarización | OK | diferencia relativa máxima 2.67e-16 |
| el modelo se entrenó con este gold | OK | gold 2fdace5ad859… · kmeans 2fdace5ad859…, regimenes 2fdace5ad859…, codo 2fdace5ad859… |
| receta de kmeans.json = configs/kmeans.json | OK | iguales |
| forma de los centroides | OK | go (3, 2) · k efectivo 3, D 2 |
| asignaciones | OK | 0 filas con otro régimen · 0 claves en un solo lado de 65,873 |
| tamaños | OK | python [48161, 1536, 16176] · go [48161, 1536, 16176] |
| inercia con los centroides finales | OK | python 50950.366891063 · go 50950.366891063 (rel 1.4e-16) |
| la inercia no crece entre iteraciones | OK | 44 iteraciones, paro «tolerancia», 0 subidas |
| punto fijo de Lloyd | OK | máx \|media de las filas − centroide\| = 7.55e-15 (umbral de paro de Go 5.2e-06, paro «tolerancia») |
| scikit-learn desde los centroides de Go | OK | se mueven 1.78e-15 en 2 iteraciones |
| centroides frente a scikit-learn | OK | máx \|Δ\| tras emparejar 1.78e-15 (tolerancia 0.001); ARI 1.0000; inercia sklearn/go − 1 = -0.00% |
| centroides des-estandarizados | OK | diferencia relativa máxima 0.00e+00 |
| arranque elegido entre las semillas | OK | 10 arranques (semillas 42–51); elegida la 42, inercia 50950.366891 = la menor; 4 semillas empatan bit a bit en ella; inercias distintas: [50950.366891, 50950.36712] |
| Lloyd desde los centroides iniciales de Go | OK | 10 de 10 arranques reproducen la inercia de Go (diferencia relativa máxima 2.9e-16); semilla elegida: máx \|Δ centroide\| 1.8e-15 |
| regimenes.json (perfil, PM medio, ECA, horas, estaciones) | OK | iguales; convención de la hora «fin» |
| codo.json frente a scikit-learn | OK | inercia de Go / mejor de scikit-learn: máximo 1.000 en k = 3 |
| cargador: mismo silver y gold para todo W | OK | identicos=True; 5 valores de W |
| cargador: el benchmark es de la receta auditada | OK | gold y configs/kmeans.json del benchmark = los auditados |
| K-Means: idéntico bit a bit para todo W | OK | convergencia chunk 1000: True; convergencia chunk 10000: True; fijas chunk 1000: True; fijas chunk 10000: True |
| K-Means: el benchmark es del gold y la receta auditados | OK | reports/benchmark_kmeans.json: gold = el auditado; receta igual; inercia de «convergencia» con chunk 1000 frente a kmeans.json: diferencia relativa 0.0e+00 |
