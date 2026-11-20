<!-- Generado por tablas.py desde docs/informe/capturas/pc4/metricas.txt. No editar a mano. -->

::: {#lst-salida-metricas}
```text
$ curl -s -H "Authorization: Bearer $TOKEN" localhost:8080/api/metricas
{
  "endpoints": {
    "GET /api/cluster/nodos": {
      "pedidos": 13,
      "p50_ms": 0.03805,
      "p95_ms": 0.09051,
      "p99_ms": 0.09051
    },
    "GET /api/entrenamientos": {
      "pedidos": 4,
      "p50_ms": 0.664,
      "p95_ms": 0.939,
      "p99_ms": 0.939
    },
    "GET /api/entrenamientos/{id}": {
      "pedidos": 13,
      "p50_ms": 0.7896,
      "p95_ms": 2.435,
      "p99_ms": 2.435
    },
    "GET /api/estaciones": {
      "pedidos": 1,
      "p50_ms": 1.069,
      "p95_ms": 1.069,
      "p99_ms": 1.069
…
```

`GET /api/metricas`: percentiles por ruta medidos dentro del servidor (primeras líneas) (salida real, `docs/informe/capturas/pc4/metricas.txt`)
:::
