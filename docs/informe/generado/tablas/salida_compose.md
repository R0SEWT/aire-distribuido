<!-- Generado por tablas.py desde docs/informe/capturas/pc4/compose-ps.txt. No editar a mano. -->

::: {#lst-salida-compose}
```text
$ docker compose -p evidencias ps -a --format 'table \
    {{.Name}}\t{{.Service}}\t{{.Status}}\t{{.Ports}}'
NAME               SERVICE STATUS                   PORTS
evidencias-api-1   api     Up 9 seconds (healthy)   7070/tcp, 127.0.0.1:8080->8080/tcp
evidencias-mongo-1 mongo   Up 15 seconds (healthy)  127.0.0.1:27017->27017/tcp
evidencias-nodo-1  nodo    Up 4 seconds             
evidencias-nodo-2  nodo    Up 4 seconds             
evidencias-nodo-3  nodo    Up 4 seconds             
evidencias-redis-1 redis   Up 15 seconds (healthy)  127.0.0.1:6379->6379/tcp
evidencias-seed-1  seed    Exited (0) 9 seconds ago 

$ curl -s localhost:8080/api/salud
{
  "api": "ok",
  "mongo": "ok",
  "redis": "ok",
  "nodos_en_pool": 3,
  "modelo_activo": "sembrado-kmeans-json"
}
```

`docker compose up --scale nodo=3`: los servicios sanos, los tres nodos y el seed terminado (salida real, `docs/informe/capturas/pc4/compose-ps.txt`)
:::
