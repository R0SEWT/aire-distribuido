<!-- Generado por tablas.py desde docs/informe/capturas/pc4/mongo.txt. No editar a mano. -->

::: {#lst-salida-mongo}
```text
$ docker compose -p evidencias exec -T mongo mongosh --quiet aire_distribuido --eval \
    'for (const c of ["estaciones","predicciones","modelos","entrenamientos","eventos_ \
    cluster"]) print(c.padEnd(16), db.getCollection(c).countDocuments({}))'
estaciones       33
predicciones     80879
modelos          3
entrenamientos   3
eventos_cluster  18

$ docker compose -p evidencias exec -T mongo mongosh --quiet aire_distribuido --eval \
    'printjson(db.predicciones.find({estacion: "CA-HU-04"}).sort({fecha_hora: -1, ts: \
    -1}).limit(1).toArray()[0])'
{
  _id: ObjectId('6b00c5714fde12c944d1a83b'),
  ts: ISODate('2026-11-20T22:10:57.362Z'),
  estacion: 'CA-HU-04',
  fecha_hora: ISODate('2021-12-11T05:00:00.000Z'),
  pm2_5: 71.6,
  pm10: 101.2,
  regimen: 1,
  excede_eca: true,
  media_24h: {
    pm2_5: 80.3625,
    pm10: 117.24166666666666,
    lecturas: 24
  },
  alerta: true,
  inca: 'mala',
  modelo_id: 'entrenado-e-20261120-1710',
  ms: 0.00479
}
```

MongoDB después de las pruebas: documentos por colección y la última predicción de una estación (salida real, `docs/informe/capturas/pc4/mongo.txt`)
:::
