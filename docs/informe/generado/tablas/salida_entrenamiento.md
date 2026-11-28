<!-- Generado por tablas.py desde docs/informe/capturas/pc4/entrenamiento.txt. No editar a mano. -->

::: {#lst-salida-entrenamiento}
```text
$ curl -s -w '\nHTTP %{http_code} en %{time_total} s\n' -X POST \
    localhost:8080/api/entrenamientos -H "Authorization: Bearer $TOKEN" -d '{"nodos": \
    3, "replicas": 1, "semilla": 42}'
{"id":"e-20261120-1710-3","estado":"en_curso"}
HTTP 202 en 0.002034 s

$ curl -s -H "Authorization: Bearer $TOKEN" localhost:8080/api/entrenamientos/e-202611 \
    20-1710-3
{
  "id": "e-20261120-1710-3",
  "estado": "terminado",
  "nodos": [
    "738df9caa875-1",
    "9546f5d8e1b6-1",
    "c4d2e5f18af9-1"
  ],
  "replicas": 1,
  "iteraciones": 44,
  "paro": "tolerancia",
  "dif_secuencial": 7.549516567451064e-15,
  "ms_total": 51.193906,
  "modelo_id": "entrenado-e-20261120-1710-3",
  "rondas": [
    {
      "iter": 1,
      "inercia": 73075.49583192292,
      "ms_ronda": 0.506603,
      "ms_fusion": 0.002467,
      "ms_por_nodo": {
        "738df9caa875-1": 0.482305,
        "9546f5d8e1b6-1": 0.362702,
        "c4d2e5f18af9-1": 0.36035
      }
    },
    {
…
```

Un entrenamiento distribuido por la API: el 202 con el id y el detalle al terminar (primeras líneas) (salida real, `docs/informe/capturas/pc4/entrenamiento.txt`)
:::
