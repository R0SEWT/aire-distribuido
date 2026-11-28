<!-- Generado por tablas.py desde docs/informe/salidas-tb2/latencia-remota.txt. No editar a mano. -->

::: {#lst-salida-latencia-remota}
```text
$ python3 latencia_remota.py aire.rosewt.dev 50     # desde una laptop en Lima
conexión TLS abierta en 585.9 ms; 50 pedidos por ruta sobre esa misma conexión
ruta                    p50 ms  p95 ms  máx ms  códigos
GET /api/salud           161.2   252.4   276.4  [200]
GET /api/regimenes       164.1   252.2   278.2  [200]
```

La latencia vista desde una laptop en Lima contra el despliegue público: dos rutas de lectura sobre una conexión abierta (salida real, `docs/informe/salidas-tb2/latencia-remota.txt`)
:::
