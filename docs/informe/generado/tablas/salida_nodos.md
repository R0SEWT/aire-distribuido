<!-- Generado por tablas.py desde docs/informe/capturas/pc4/entrenamiento.txt. No editar a mano. -->

::: {#lst-salida-nodos}
```text
$ curl -s -H "Authorization: Bearer $TOKEN" localhost:8080/api/cluster/nodos
[
  {
    "nombre": "738df9caa875-1",
    "direccion": "172.19.0.7:53286",
    "workers": 14,
    "latencia_ms": 0.112333,
    "libre": true
  },
…
```

`GET /api/cluster/nodos`: los nodos del pool tal como los ve el coordinador (primeras líneas) (salida real, `docs/informe/capturas/pc4/entrenamiento.txt`)
:::
