# Contrato de la API

[`openapi.yaml`](openapi.yaml) es el contrato de la API: las rutas de la spec de la PC4 (§6.1, con
los ejemplos de §6.5) y las que sumó el frontend del TB2 (spec del frontend, §6: `GET /api/regimenes`,
`GET /api/cluster/eventos`, el evento `estacion.regimen` y `proceso_api` en las métricas). La API lo sirve tal cual en `/openapi.yaml` y Swagger UI lo muestra en
`/docs`. Se revisa importándolo en [Swagger Editor](https://editor.swagger.io/) (**File → Import
file**).

![Contrato cargado en Swagger Editor](../informe/capturas/pc4/swagger-editor.png)

## Acceso

Libres: token, ingreso de invitado, salud, `/openapi.yaml` y `/docs`. Con rol `usuario` o `admin`: predicciones,
estaciones y su estado, modelo activo y `/ws`. Solo `admin`: nodos y eventos del clúster,
entrenamientos y métricas;
un `usuario` recibe 403. `x-roles` deja el rol escrito en cada operación. Por `/ws`, el estado de
las estaciones (`estacion.regimen`) llega a los dos roles y los eventos de un entrenamiento, solo a
los `admin`.

## Lo que el contrato fija y la spec no decía

- **Alerta e INCA**: la predicción trae el régimen (el tipo de aire, con `excede_eca` del régimen) y,
  aparte, la media de 24 h de la estación (`media_24h`, en memoria en la API). `alerta` es que esa
  media supere el ECA (PM2.5 > 50 o PM10 > 100 µg/m³) e `inca` es su banda del INCA; las dos son
  normas de 24 h y se informan por separado. Hacen falta 18 lecturas en la ventana. La ventana se
  calcula respecto de la lectura que llega (su `fecha_hora`, truncada a la hora), así que sirven los
  datos históricos y las lecturas desordenadas; por estación se conservan las 72 lecturas horarias
  más cercanas a la última recibida. La estación tiene que estar en el catálogo, y se rechazan un PM mayor
  de 10 000 µg/m³ y una fecha más de 1 hora adelante del reloj del servidor. El régimen por estación guarda la `fecha_hora` de su lectura y no
  retrocede con una lectura más antigua. En el gold, el
  régimen alto solo acierta el 35 % de las horas sobre el ECA de 24 h: por eso la alerta no sale
  del régimen.
- **Estaciones**: `id` es el `_id` de la colección (el código OEFA) y la ficha trae todos los campos
  de §7.1; un dato que falte en la fuente llega como `null`.
- **Entrenamientos**: tres estados, `en_curso`, `terminado` y `abortado`. El listado no trae las
  rondas; se piden por id. `paro` es `tolerancia` o `max_iter`, los motivos de `kmeans`.
- **Ingreso de invitado**: `POST /api/auth/invitado`, sin cuerpo y sin credenciales, responde un
  token con la misma forma que `POST /api/auth/token`. Es de la cuenta que nombra la variable de
  entorno `API_INVITADO`: un usuario de `API_USUARIOS` con el rol `usuario`; si no existe o es
  `admin`, la API no arranca y dice por qué. Sin la variable, la ruta responde 404 («el ingreso de
  invitado no está habilitado»). Con `-demo` y sin la variable, el invitado es `demo`. La clave de la
  cuenta no sale del servidor, y al arrancar el log dice si el invitado está habilitado y con qué
  usuario. Un invitado puede todo lo del rol `usuario`, también mandar lecturas
  (`POST /api/predicciones`): habilitarlo en una máquina pública es una decisión, no un descuido.
- **Errores**: siempre `{"error": "…"}`. 404 cuando no existe el recurso (para el régimen de una
  estación, también cuando todavía no tiene predicciones), 503 cuando no hay modelo activo o no
  responde el almacén de la ruta.
- **Métricas de escritura**: la predicción no espera a MongoDB ni a Redis; se encola y una
  goroutine por almacén la escribe por lotes. `/api/metricas` cuenta lo que no llegó:
  `escrituras_descartadas` (una cola llena) y `escrituras_fallidas` (el almacén rechazó el lote), y
  en `escritura` el detalle de cada almacén. `/api/salud` informa si cada base responde.
- **CPU y memoria**: `/api/metricas` trae en `proceso_api` el CPU y la memoria del proceso de la
  API, que es el coordinador. No son de los nodos: el protocolo del clúster no transporta su CPU, y
  de ellos se conoce `ms_por_nodo` y `latencia_ms`. El tiempo de CPU es acumulado; el uso entre dos
  consultas es `Δ(cpu_usuario_s + cpu_sistema_s) / Δen_marcha_s`, con tope en `nucleos`.
- **Eventos del clúster**: `GET /api/cluster/eventos?limite=N` (admin) devuelve las altas, las bajas
  y los errores de los nodos, del más reciente al más antiguo: 50 por omisión y 200 como máximo.
  Después de cada entrenamiento hay una baja y un alta por nodo.
- **Reinicios**: `ultimo_entrenamiento` se recupera del almacén al arrancar, y un entrenamiento que
  estaba `en_curso` cuando la API murió queda `abortado` con `error` «la API se reinició mientras el
  entrenamiento estaba en curso». `puntos_de_control` cuenta los puntos de control por ronda.
- **Estado de las estaciones** (spec del frontend, §6): `GET /api/regimenes` trae en un pedido, y en
  una sola ida a Redis, el estado de todas las estaciones que tienen uno (una lista vacía es un 200).
  El evento `estacion.regimen` de `/ws` lleva ese mismo objeto (`EstadoEstacion`) cuando el estado de
  una estación cambia. Sale solo si la lectura pasó a ser el estado de la estación (la escritura en
  Redis reemplazó al guardado): una lectura más antigua no emite nada, así que el evento y el GET
  dicen siempre lo mismo. Va agrupado por estación, a lo sumo uno cada 250 ms con el último estado:
  una ráfaga de miles de predicciones no llena el búfer de los clientes. Si el valor guardado de una
  estación no se puede leer, el GET la salta (queda en el log) y responde 200 con las demás.
- **Juntar el GET con los eventos**: el cliente abre primero el WebSocket y después pide el GET (al
  reconectar, otra vez). Como viajan por conexiones distintas, la respuesta del GET puede llegar
  después de un evento más nuevo de la misma estación. La regla: de una estación de la que el
  cliente ya recibió un evento desde que abrió ese WebSocket, la entrada del GET se ignora (si el GET
  traía algo más nuevo, su evento llega después); de las demás, se toma la del GET. Comparar por
  `ts` no sirve: va al segundo, y una corrección de la misma hora conserva `fecha_hora`. Si Redis
  guarda un estado y su respuesta se pierde, la API lo relee cuando Redis vuelve y emite el evento
  igual; por eso un evento puede repetir un estado ya recibido.
- **WebSocket**: cada trama es `{"evento": "estacion.regimen" | "entrenamiento.ronda" |
  "entrenamiento.fin", "datos": {…}}`; los de entrenamiento, con los datos de §6.4. El JWT va en `Authorization`, en `Sec-WebSocket-Protocol`
  (`aire.jwt` seguido del token: lo recomendado para el navegador, que no deja poner encabezados,
  porque el token no queda en la URL) o en `?token=`. El `entrenamiento.fin` sale cuando el detalle
  ya tiene el estado final: quien lo recibe puede pedir `GET /api/entrenamientos/{id}` en el acto.
- **Modelo de un entrenamiento**: con `replicas = 1` el modelo nuevo se activa (`modelo_id`) solo si
  `dif_secuencial` no supera 1e-9; si la supera, el entrenamiento queda `terminado`, el modelo
  activo no cambia y `aviso` dice por qué. No hay clave de idempotencia: un segundo pedido mientras
  corre el primero recibe 409 con su id.
- **Clúster**: los nodos (`cmd/nodo -coordinador <IP>:7070`) llaman a la API por TCP en un puerto
  aparte (`-nodos-addr`, por omisión `:7070`) y quedan en su pool. `latencia_ms` es la ida y vuelta
  del último latido, que solo corre sobre los nodos libres; `ms_por_nodo` es lo que tardó el PARCIAL
  de cada nodo, medido en la API. Para entrenar, la API necesita el mismo gold que los nodos
  (`-gold`); sin él arranca igual y `POST /api/entrenamientos` responde 503. Cada nodo necesita un
  nombre único: `cmd/nodo` usa el del equipo más su PID si no se le da `-nombre`.
- **CORS**: los orígenes que pueden llamar desde un navegador se configuran con la variable de
  entorno `CORS_ORIGENES` (separados por comas, o `*`); la API responde el preflight `OPTIONS`.
