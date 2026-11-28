<!-- Generado por tablas.py desde docs/informe/capturas/pc4/mongo.txt. No editar a mano. -->

::: {#lst-salida-mongo-modelos}
```text
$ docker compose -p evidencias exec -T mongo mongosh --quiet aire_distribuido --eval \
    'printjson(db.modelos.find({}, {origen: 1, creado: 1, activo: 1, iteraciones: 1, \
    inercia: 1}).sort({creado: 1}).toArray())'
[
  {
    _id: 'sembrado-kmeans-json',
    creado: ISODate('2026-11-02T04:59:16.218Z'),
    origen: 'sembrado',
    inercia: 50950.36689106275,
    iteraciones: 44,
    activo: false
  },
  {
    _id: 'entrenado-e-20261120-1710',
    creado: ISODate('2026-11-20T22:10:43.157Z'),
    origen: 'e-20261120-1710',
    inercia: 50950.36689106274,
    iteraciones: 44,
    activo: false
  },
  {
    _id: 'entrenado-e-20261120-1710-3',
    creado: ISODate('2026-11-20T22:10:57.520Z'),
    origen: 'e-20261120-1710-3',
    inercia: 50950.36689106274,
    iteraciones: 44,
    activo: true
  }
]
```

MongoDB: los modelos guardados y cuál está activo (salida real, `docs/informe/capturas/pc4/mongo.txt`)
:::
