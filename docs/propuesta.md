# Propuesta técnica — caso 10: calidad del aire

Borrador para la PC3. Sale de la propuesta SENAMHI de setiembre (`../concurrente`, rama
`feat/senamhi-ods13`, `docs/PR_SENAMHI_ODS13.md`): se conservan el worker pool, la reducción de
resultados parciales y el diseño de experimentos; cambian el modelo (de árbol de decisión a K-Means
paralelo, como pide el caso 10) y la fuente principal (OEFA, para pasar el millón de filas).

## Objeto de estudio

Lecturas continuas de las estaciones de calidad del aire de OEFA y SENAMHI. Se buscan regímenes de
contaminación (combinaciones típicas de PM2.5, PM10, gases y meteorología por estación y hora) y se
usan para avisar con anticipación cuando una estación entra en un régimen que supera el ECA de aire.

Pregunta de impacto social (borrador): ¿en qué estaciones y a qué horas se concentran los episodios
de PM2.5/PM10 que superan el ECA, y con cuánta anticipación se podría avisar a la población cercana?

## Datos

| Fuente | Volumen | Papel |
|--------|---------|-------|
| OEFA, Vigilancia y Seguimiento ambiental en la calidad del aire | CSV de 1.3 GB, 34 estaciones | Principal: entrenamiento y la carga de >1M filas |
| SENAMHI, Monitoreo de los contaminantes del aire en Lima Metropolitana | 577,794 registros | Complemento en Lima y contraste entre redes |

Variables de entrada previstas (los nombres exactos salen del diccionario de OEFA): estación,
coordenadas, fecha, hora, PM2.5, PM10, NO2 y las variables meteorológicas que traiga cada estación
(temperatura, humedad, viento). Se estandarizan antes de entrar al K-Means.

Limpieza: valores negativos o fuera de rango del equipo, lecturas duplicadas por estación y hora,
huecos de la serie y unidades inconsistentes entre las dos redes.

## Modelo: K-Means paralelo

1. El coordinador elige k centroides iniciales (k-means++ sobre una muestra).
2. Cada nodo recibe los centroides, asigna cada lectura de su partición al más cercano y devuelve,
   por cluster, la suma de los vectores y el conteo.
3. El coordinador suma los parciales de todos los nodos, recalcula los centroides y repite hasta que
   se muevan menos que una tolerancia o se llegue a un máximo de iteraciones.
4. Cada cluster se etiqueta por su perfil medio frente al ECA (DS 003-2017-MINAM): PM2.5 de 24 h en
   50 µg/m³ y PM10 de 24 h en 100 µg/m³.
5. En tiempo real, una lectura nueva se clasifica por su centroide más cercano; si cae en un régimen
   que excede el ECA, se emite la alerta para esa estación.

k se elige con el método del codo y la silueta sobre una muestra, y se justifica en el informe.

## Arquitectura

Concurrente dentro de cada nodo (heredado de la propuesta SENAMHI):

```
CSV ─▶ Reader ─▶ channel ─▶ Worker 1..N (limpieza, transformación, asignación) ─▶ Reducer
```

Distribuida entre nodos:

```
Coordinador (API) ──TCP──▶ Nodo 1..M: asignación + sumas parciales
        ▲                              │
        └──────── parciales ───────────┘
```

## Sincronización

Goroutines, channels, `sync.WaitGroup` y `sync.Mutex`. Cada worker acumula en su propia estructura y
el reducer fusiona al final, de modo que no hay escritura compartida en el camino caliente.

La fusión de K-Means es una reducción exacta: sumar las sumas y los conteos parciales da lo mismo
que calcularlos de una vez. Es la propiedad que se prueba.

## Experimentos

- Workers por nodo: 1, 2, 4, 8, 16. Nodos: 1, 2, 4.
- Métricas: tiempo de carga, tiempo por iteración del K-Means, speedup, uso de CPU y memoria.
- Speedup = tiempo secuencial / tiempo concurrente.

## Tests

- Correctitud de la carga: registros leídos = registros procesados + descartados, sin duplicados.
- Concurrencia: todo con `go test -race`.
- Fusión: los centroides de la versión distribuida coinciden con los de la secuencial, con la misma
  semilla y los mismos datos.

## Pendiente para la PC3 (07/11)

- Bajar el CSV de OEFA, contar las filas y leer su diccionario.
- Validar la pregunta de impacto social con el equipo.
- Cargador concurrente con sus tests y la medición de speedup.
- Diseño del K-Means distribuido y su explicación en el informe.
