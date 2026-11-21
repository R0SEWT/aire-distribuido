<!-- Generado por tablas.py desde docs/informe/capturas/pc4/redis.txt. No editar a mano. -->

::: {#lst-salida-redis}
```text
$ docker compose -p evidencias exec -T redis redis-cli --scan --pattern modelo:*
modelo:activo

$ docker compose -p evidencias exec -T redis redis-cli --scan --pattern \
    entrenamiento:*
entrenamiento:e-20261120-1710-3:punto
entrenamiento:e-20261120-1710-2:punto
entrenamiento:e-20261120-1710:punto

$ docker compose -p evidencias exec -T redis redis-cli GET estacion:CA-HU-04:regimen
{"fecha_hora":"2021-12-11T00:00:00-05:00","regimen":1,"excede_eca":true,"media_24h":{" \
    pm2_5":80.3625,"pm10":117.24166666666666,"lecturas":24},"alerta":true,"inca":"mala \
    ","ts":"2026-11-20T17:10:57-05:00"}

$ docker compose -p evidencias exec -T redis redis-cli GET \
    entrenamiento:e-20261120-1710-3:punto
{"iter":44,"centroides":[[-0.3810318622031203,-0.3734783717613215],[3.8292692196401195 \
    ,4.165192564331026],[0.7708406277322727,0.7164537638219931]]}
```

Redis después de las pruebas: las claves, el estado de una estación y un punto de control (salida real, `docs/informe/capturas/pc4/redis.txt`)
:::
