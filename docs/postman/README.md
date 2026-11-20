# Postman: pruebas de la API y datos de prueba

Evidencias de la parte C de la PC4 (spec §9). La colección prueba la API real contra el contrato
`docs/api/openapi.yaml`; no hay simulador.

| Archivo | Qué es |
|---|---|
| `aire-distribuido.postman_collection.json` | La colección: 50 pedidos definidos en cuatro carpetas (34, 10, 4 y 2) |
| `local.postman_environment.json` | El entorno local, **sin credenciales**: se completan al importarlo |
| `lecturas_prueba.csv` | Datos de prueba para el Runner: 78 lecturas reales con lo que la API tiene que responder |
| `generar_lecturas.py` | Cómo se armó el CSV y se calculó lo esperado, sin consultar a la API |

## Qué comprueba cada pedido

- **El código HTTP** del caso (200, 202, 400, 401, 403, 404, 409, 422).
- **La forma de la respuesta**: el esquema del contrato, con `additionalProperties: false`. Un campo
  que la API devuelva y el contrato no nombre hace fallar la prueba, y al revés.
- **El tiempo HTTP completo** que mide Postman (`pm.response.responseTime`), menor que `limite_ms`
  (100 ms). Es decisión del equipo aplicarlo a todas las rutas, no solo a la predicción;
  `POST /api/entrenamientos` entra porque responde 202 enseguida y entrena en segundo plano.
- Lo propio del caso: el mensaje del error, el rol del token, que `media_24h`, `alerta` e `inca`
  sean coherentes entre sí, que un entrenamiento traiga una ronda por iteración con el tiempo de cada
  nodo, que `ultimo_entrenamiento` y `escritura` de las métricas digan lo que pasó.

| Carpeta | Qué cubre |
|---|---|
| 1. Base HTTP | Salud, token (200, 400, 401), predicción válida y sus rechazos (sin token, PM negativo, PM mayor de 10 000, estación vacía o desconocida, fecha inválida o futura, cuerpo que no es JSON), estaciones y su régimen (200 y los dos 404), modelo activo, `/openapi.yaml`, `/docs`, los 403 del rol usuario, y nodos, métricas e historial como admin |
| 2. Entrenamiento | 400 (cero nodos, cero réplicas, sin semilla), 422 (más nodos de los que hay), 202 y el detalle hasta `terminado`; después, el historial, las métricas y el modelo activo, que pasa a ser `entrenado-{id}` |
| 3. Conflicto controlado | Un entrenamiento largo (`replicas_conflicto` copias del gold) y, mientras corre, un segundo pedido que recibe 409 con el id del que está en curso |
| 4. Datos de prueba | El Runner con `lecturas_prueba.csv`: una fila por iteración |

**Cuántos pedidos ejecuta una corrida.** La colección define 50 pedidos, pero el resumen de una
corrida de las carpetas 1 a 3 dice 62 o 63, y no siempre el mismo número. No es un fallo:

- De los 50, se envían los 48 de las carpetas 1 a 3. Los 2 de la carpeta 4 se saltan si no hay
  archivo de datos.
- El resumen cuenta además los 2 pedidos automáticos de token (uno por rol) que la colección hace
  antes del primer pedido.
- Cuatro pedidos son sondeos que se repiten hasta que se cumple su condición: «Esperar nodos
  libres» (dos veces en la colección), «Consultar hasta terminar» y «Cerrar escenario de conflicto».
  Cuántas veces se repiten depende de cuánto tarden los nodos en volver al pool (unos 2 s después
  de cada entrenamiento) y de cuánto dure el entrenamiento largo: alrededor de una docena de
  repeticiones en total, cada una con 5 aserciones.

Por eso dos corridas correctas pueden dar, por ejemplo, 62 pedidos y 310 aserciones, y 63 y 315. Lo
que tiene que coincidir es que no haya ninguna aserción fallida. El Runner con el CSV sí es fijo:
78 iteraciones, 82 pedidos (78 predicciones, 2 consultas del estado al cerrar cada serie y los 2
tokens) y 714 aserciones.

`GET /ws` no está: una colección v2.1 no guarda pedidos WebSocket. Se prueba en `api/` y
`cmd/api/` con `go test`.

## Usarla en Postman

1. Levanta el sistema con tres nodos (`.env` con `JWT_SECRETO` y `API_USUARIOS`, ver el README de la raíz):

   ```bash
   docker compose up -d --build --scale nodo=3
   ```

2. Importa la colección y el entorno (*Import* → los dos `.json`) y elige el entorno
   «aire-distribuido - local».
3. En el entorno, completa `usuario_usuario`, `usuario_clave`, `admin_usuario` y `admin_clave` con un
   usuario de rol `usuario` y uno de rol `admin` de tu `API_USUARIOS`. Escríbelos en *Current value*:
   no se sincronizan ni se exportan. Si guardas una copia del entorno con claves, que se llame
   `*.local.postman_environment.json`: está en `.gitignore`.
4. Envía cualquier pedido. Los tokens se piden solos antes del pedido y se renuevan cuando les queda
   menos de un minuto; «Token usuario» y «Token admin» los piden a la vista. Si la API se reinició con
   otro `JWT_SECRETO` y un pedido da 401, envía esos dos.
5. Las carpetas 1 a 3 se corren con *Run folder*, en ese orden.

Variables del entorno que conviene conocer: `nodos_entrenamiento` (3: los nodos que tiene que haber
libres), `replicas_conflicto` (300: cuánto dura el entrenamiento largo; en una máquina lenta se
puede bajar, en una rápida, subir, mientras el 409 llegue con la corrida en curso), `limite_ms` (100)
y `estacion_sin_lecturas` (`CA-01`, una estación del catálogo que no mide PM por hora).

## El Runner con los datos de prueba

*Run folder* sobre «4. Datos de prueba», *Select file* → `lecturas_prueba.csv`. Son 78 iteraciones.

| `caso` | Filas | Qué se espera |
|---|---|---|
| `regimen_0`, `regimen_1`, `regimen_2` | 10 cada uno | El régimen de `data/gold/asignaciones.csv`. Cada lectura está sola en su ventana: 1 lectura, sin media, sin alerta, sin INCA |
| `serie_alerta` | 24 horas seguidas de `CA-HU-04` | `lecturas` sube de 1 a 24; desde la 18 hay media; termina con 80,36 y 117,24 µg/m³: `alerta: true`, INCA `mala` |
| `serie_sin_alerta` | 24 horas seguidas de `CA-ROM-01` | Termina con 38,60 y 58,54 µg/m³: `alerta: false` e INCA `mala` por el PM2.5 (el INCA y el ECA se informan aparte) |

Cada iteración comprueba el régimen, las lecturas en la ventana, la media de 24 h (con tolerancia
de 1e-9), la alerta y el INCA. En la última fila de cada serie consulta además
`GET /api/estaciones/{id}/regimen`: lo que quedó en Redis tiene que ser lo del CSV.

Lo esperado **no se le preguntó a la API**: `generar_lecturas.py` lo calcula desde el gold y el
silver con la regla del contrato. Para regenerarlo (con `data/gold` y `data/silver` en su sitio):

```bash
python3 docs/postman/generar_lecturas.py
```

Repetir el archivo deja la API en el mismo estado (la misma estación y hora reemplaza a la
anterior), así que el Runner se puede correr varias veces y `cmd/carga` usa estas mismas lecturas.
Lo que sí cambia lo esperado es mandar a mano otras horas de `CA-HU-04` o `CA-ROM-01` dentro de las
24 horas de su serie.

## Correrla con newman

`newman` se usa con `npx`, como herramienta: el repositorio no tiene código Node.

```bash
# Carpetas 1 a 3 (los pedidos de la 4 se saltan si no hay archivo de datos)
npx newman@6 run docs/postman/aire-distribuido.postman_collection.json \
  -e docs/postman/local.postman_environment.json \
  --env-var usuario_usuario=demo --env-var usuario_clave="$CLAVE_DEMO" \
  --env-var admin_usuario=admin --env-var admin_clave="$CLAVE_ADMIN"

# El Runner con el CSV
npx newman@6 run docs/postman/aire-distribuido.postman_collection.json \
  -e docs/postman/local.postman_environment.json \
  --env-var usuario_usuario=demo --env-var usuario_clave="$CLAVE_DEMO" \
  --env-var admin_usuario=admin --env-var admin_clave="$CLAVE_ADMIN" \
  --folder "4. Datos de prueba (Runner con lecturas_prueba.csv)" -d docs/postman/lecturas_prueba.csv
```

La salida de una corrida real está en `docs/informe/capturas/pc4/`.

## Procedencia

La colección parte de la que Julio Meza hizo para J2 contra un simulador (`feature/postman-pc4`).
De ahí se conservan las carpetas, el recorrido de los casos y el sondeo del entrenamiento; se
quitaron el simulador y su modo, y se puso al día con el contrato actual.
