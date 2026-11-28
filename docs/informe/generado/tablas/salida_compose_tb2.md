<!-- Generado por tablas.py desde docs/informe/capturas/tb2/despliegue/compose-ps.txt. No editar a mano. -->

::: {#lst-salida-compose-tb2}
```text
$ docker compose up --build -d --scale nodo=2 && docker compose ps -a   # 2026-11-27 \
    18:51 -05, commit 4cfc6dc
SERVICE NAME                STATUS             PORTS
api aire-tb2-frontend-api-1 Up 9 seconds (healthy) 7070/tcp, 127.0.0.1:8080->8080/tcp
mongo aire-tb2-frontend-mongo-1 Up 17 seconds (healthy) 127.0.0.1:27017->27017/tcp
nodo aire-tb2-frontend-nodo-1 Up 3 seconds       
nodo aire-tb2-frontend-nodo-2 Up 3 seconds       
redis aire-tb2-frontend-redis-1 Up 17 seconds (healthy) 127.0.0.1:6379->6379/tcp
seed aire-tb2-frontend-seed-1 Exited (0) 9 seconds ago 
web aire-tb2-frontend-web-1 Up 4 seconds       443/tcp, 2019/tcp, 443/udp, \
    127.0.0.1:8081->80/tcp
```

`docker compose ps -a` con el sistema del TB2 levantado limpio: el servicio web, la API, dos nodos, MongoDB, Redis y el seed (salida real, `docs/informe/capturas/tb2/despliegue/compose-ps.txt`)
:::
