<!-- Generado por tablas.py desde docs/informe/salidas-tb2/metricas-verificacion.txt. No editar a mano. -->

::: {#lst-salida-metricas-tb2}
```text
$ curl -s http://127.0.0.1:18093/api/metricas -H "Authorization: Bearer $TOKEN_ADMIN" \
    # después de la verificación
{
  "endpoints": {
    "POST /api/predicciones": {
      "pedidos": 1086,
      "p50_ms": 0.02263,
      "p95_ms": 0.0349,
      "p99_ms": 0.04525
    },
    "GET /api/estaciones/{id}/regimen": {
      "pedidos": 14,
      "p50_ms": 0.005187,
…
```

`GET /api/metricas` al terminar la verificación: la latencia de la predicción medida dentro del servidor (primeras líneas) (salida real, `docs/informe/salidas-tb2/metricas-verificacion.txt`)
:::
