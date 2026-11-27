# Capturas y pruebas de punta a punta del TB2

Tomadas el **27/11/2026, entre las 18:32 y las 18:45 (hora de Lima)**, con Playwright (Chromium, ventana de
1440 × 900) manejando la UI real contra el **Docker Compose del repo, levantado limpio** (commit `6d91f4f`, la
punta de `develop`):

```bash
docker compose -p aire-tb2-frontend up --build -d --scale nodo=2   # UI en :8081, API en :8080
```

API, dos nodos, MongoDB, Redis y el servicio `web` (el Caddy que sirve la SPA y pasa `/api` y `/ws`). Los
usuarios y el `JWT_SECRETO` se generaron para esta corrida y no están en el repo; el invitado es la cuenta
`demo` (`API_INVITADO=demo`) y el token dura 4 horas (`JWT_DURACION=4h`). Todo lo que se ve son lecturas
reales de OEFA: el escenario y los días salen de `web/public/datos/series.json` y la lectura a mano es la de
`CA-AYAS-01` del 16/12/2021 a la 01:00 tal como está en el silver (PM2.5 2.8, PM10 3.3 µg/m³). Nada se
inventó ni se editó después.

El orden de abajo es el de la corrida: cada captura depende de lo que pasó antes. Las «completa» son la
página entera; las demás, lo que entra en la ventana. El 3 de `pruebas/` describe cómo se midió la latencia.

## 1. Lista de punta a punta (spec del frontend, §12)

| # | Paso | Resultado | Evidencia |
|---|---|---|---|
| 1 | El Compose levanta los seis servicios sanos con dos nodos | ✅ | `pruebas/compose-ps.txt`, `despliegue/compose-ps.txt` |
| 2 | Ingresar con una clave mala: dice «Usuario o clave incorrectos.», sin más | ✅ | `ingreso/02` |
| 3 | Entrar como invitado («No pienso crearme una cuenta aquí» → «Entrar igual»): rol `usuario`, menú de cuatro pantallas | ✅ | `ingreso/03`, `panel/01` |
| 4 | Con el sistema recién levantado, el Panel explica por qué está vacío y ofrece ir a Simulación | ✅ | `panel/01`, `panel/02` |
| 5 | Un `usuario` que fuerza `/cluster` ve «solo para administradores»; la API le responde 403 | ✅ | `seguridad/01`, `seguridad/politicas.txt` §7 |
| 6 | Ingresar como admin: el menú suma Entrenamiento y Clúster | ✅ | `ingreso/04`, `panel/02` |
| 7 | Cargar el escenario (48 h reales del 14 y 15/12/2021, 667 lecturas) desde la UI de admin, a ritmo lento | ✅ | `simulacion/01` a `04` |
| 8 | **Tiempo real**: mientras el admin carga el escenario, el Panel del invitado (otro navegador, otra sesión) se llena solo por el WebSocket | ✅ | `panel/03` (a mitad: 376 de 667) y `panel/04` (al terminar: 14 estaciones con estado, 2 en alerta) |
| 9 | Simular un día que termina en alerta (CA-ESP-01, 21/12/2021, «cruza el ECA»): la alerta se enciende a las 07:00 por PM10 | ✅ | `simulacion/05` a `07` |
| 10 | Mandar una lectura a mano: la API responde régimen, media, banda y alerta | ✅ | `simulacion/08` |
| 11 | La Estación muestra su último estado, la curva de lo enviado en la sesión y su ficha con la población del distrito | ✅ | `estacion/01` a `03` |
| 12 | Entrenamiento: pedir 4 nodos con 2 libres → la UI avisa y la API responde 422 con su texto | ✅ | `entrenamiento/02` |
| 13 | Entrenamiento configurado desde la UI (2 nodos, 1 réplica, semilla 42): 44 rondas reproducidas, convergió, diferencia con el secuencial 7.7e-15, el modelo nuevo queda activo | ✅ | `entrenamiento/03` a `06` |
| 14 | Medición larga (300 réplicas) en curso: las rondas llegan por el WebSocket mientras el entrenamiento sigue | ✅ | `entrenamiento/07` |
| 15 | Clúster: número de nodos, CPU del coordinador, latencia por ruta (peor p99 2.2 ms), escrituras sin pérdidas y los eventos de altas y bajas | ✅ | `cluster/01`, `cluster/02` |
| 16 | Impacto con los datos reales del gold, sin suma de población y con lo que el dato no puede decir a la vista | ✅ | `impacto/01` a `09` |
| 17 | Modo proyector | ✅ | `armazon/01` |
| 18 | La sesión avisa 5 minutos antes de vencer y, al vencer, vuelve al ingreso con «La sesión venció.» | ✅ | `seguridad/02`, `seguridad/03` |
| 19 | Ninguna violación de la CSP en la consola de la ventana principal (invitado y después admin) en toda la corrida: los únicos errores son el 401 del paso 2 y el 422 del paso 12, a propósito | ✅ | consola de Playwright |
| 20 | Latencia de la API < 100 ms medida en local, directo y a través del Caddy de la UI | ✅ | `pruebas/` (abajo) |
| 21 | Una lectura llega al panel de otro cliente por el WebSocket en milisegundos, y el último estado coincide con lo esperado de `series.json` | ✅ | `pruebas/ws_latencia.json` |

## 2. Las capturas

### `ingreso/`

| Archivo | Qué muestra |
|---|---|
| `01-ingreso.png` | La pantalla de ingreso, con el estado de la API en la barra («API en línea») |
| `02-clave-incorrecta.png` | El 401 de la API dicho en llano: «Usuario o clave incorrectos.» |
| `03-dialogo-invitado.png` | El diálogo de «No pienso crearme una cuenta aquí», que deja entrar como invitado (`POST /api/auth/invitado`) |
| `04-ingreso-admin.png` | El ingreso de admin, con la clave tapada |

### `panel/`

| Archivo | Qué muestra |
|---|---|
| `01-panel-vacio-invitado.png` | El panel recién levantado, visto por el invitado: 17 estaciones «sin datos suficientes», 16 «no mide PM por hora», y por qué no hay nada todavía |
| `02-panel-vacio-admin.png` | El mismo estado vacío para admin, que ofrece «Ir a cargar el escenario» |
| `03-panel-en-vivo-invitado.png` | **Tiempo real**: el panel del invitado a mitad del escenario que carga el admin (1 en alerta, 14 con estado, último estado 15/12/2021 03:00) |
| `04-panel-tras-escenario-invitado.png` | El mismo panel al terminar el escenario: 2 estaciones en alerta (CA-HU-04 y CA-HU-09, ladrilleras de Lurigancho) |
| `05-panel-admin.png`, `06-panel-admin-completa.png` | El panel del admin después del escenario, el día simulado y la lectura a mano: 3 en alerta, el mapa con sus anillos y la lista por gravedad |

### `simulacion/`

| Archivo | Qué muestra |
|---|---|
| `01-simulacion-antes.png` | El escenario sin empezar: 48 horas, 14 estaciones, 667 lecturas, «sistema limpio para este tramo» |
| `02-escenario-en-curso.png` | A mitad (376 de 667): la rejilla de horas por estación coloreada por banda INCA, CA-HU-04 ya en alerta |
| `03-escenario-terminado.png`, `04-escenario-terminado-completa.png` | El escenario terminado, con la curva de una estación y la comparación con lo esperado |
| `05-dia-configurado.png` | Un día de una estación elegido desde la UI: CA-ESP-01, 21/12/2021, ritmo rápido |
| `06-dia-cruza-eca-terminado.png`, `07-dia-cruza-eca-completa.png` | El día simulado: la media de PM10 aparece a las 18:00 del día anterior y la alerta se enciende a las 07:00 (131.9 µg/m³ al final, ECA 100) |
| `08-lectura-a-mano.png` | Una lectura a mano (la real del silver) y lo que respondió la API |
| `09-comparacion-con-lo-esperado.png` | La tabla de evaluación del escenario, recortada de `04`: las 14 estaciones coinciden con lo esperado de `series.json` (media, banda, alerta, primera alerta y régimen) |

### `estacion/`

| Archivo | Qué muestra |
|---|---|
| `01-estacion-ca-esp-01.png`, `02-estacion-ca-esp-01-completa.png` | CA-ESP-01 en alerta: medias de 24 h, banda, régimen, la curva de lo que la sesión envió y la ficha (corredor vial, Espinar, 34 861 habitantes según el censo 2017) |
| `03-de-donde-sale-este-numero.png` | El desplegable «¿De dónde sale este número?»: la regla de la alerta, las normas y por qué el régimen no es la alerta |

### `entrenamiento/`

| Archivo | Qué muestra |
|---|---|
| `01-formulario.png` | El formulario con los nodos libres a la vista |
| `02-error-mas-nodos-que-libres.png` | Se piden 4 nodos con 2 libres: la UI avisa y muestra el 422 de la API con su texto |
| `03-formulario-configurado.png` | Parámetros ingresados desde la UI: 2 nodos, 1 réplica, semilla 42 |
| `04-rondas-reproduciendose.png` | Las rondas reproducidas a ritmo humano (ronda 28 de 44): la inercia y el tiempo de cada nodo |
| `05-resultado.png`, `06-resultado-completa.png` | El resultado: 44 iteraciones, convergió, 54 ms reales, diferencia con el secuencial 7.7e-15, el modelo nuevo quedó activo; y el historial |
| `07-medicion-larga-en-curso.png` | El preajuste «medición larga» (300 réplicas) mientras corre: 0 nodos libres y las rondas llegando |

### `cluster/`

| Archivo | Qué muestra |
|---|---|
| `01-cluster.png`, `02-cluster-completa.png` | El panel de administrador que pide el enunciado: número de nodos (2, con latido y tiempo por ronda), uso de CPU y memoria del proceso de la API que coordina, latencia de la API por ruta con la línea de los 100 ms, escrituras por almacén (716 sin pérdidas), salud de cada componente y los «logs» del clúster (altas y bajas de nodos alrededor de cada entrenamiento) |
| `03-eventos-y-escrituras.png` | Recorte de `02`: los eventos del clúster y las escrituras por almacén |

### `impacto/`

| Archivo | Qué muestra |
|---|---|
| `01-impacto.png`, `02-impacto-completa.png` | La página de impacto, arriba y entera |
| `03-eca-e-inca.png` | El problema: qué es el ECA y qué es el INCA |
| `04-horas-sobre-eca.png` | 1 184 horas sobre el ECA de 64 748 con media calculable (1.8 %), y su reparto por banda INCA |
| `05-por-estacion.png` | Por estación, con su actividad fiscalizada y las semanas en que midió |
| `06-quienes-tendrian-aviso.png` | Quiénes tendrían un aviso que hoy no tienen: la población del distrito fila por fila, sin sumarla |
| `07-acierto-de-las-senales.png` | ¿Sirve la alerta?: precisión y cobertura de las señales hora a hora |
| `08-anticipacion.png` | ¿Con cuánta anticipación?: 30 episodios, contados de las dos maneras (desde el primer encendido y sin interrupción) |
| `09-lo-que-no-puede-decir.png` | Lo que este dato no puede decir, a la vista |

### `armazon/`

| Archivo | Qué muestra |
|---|---|
| `01-modo-proyector-panel.png` | El panel en modo proyector (letra más grande y más contraste, para la sustentación) |

### `despliegue/`

| Archivo | Qué muestra |
|---|---|
| `compose-ps.txt` | `docker compose ps -a` del Compose levantado otra vez limpio, sobre esta rama con develop al día (`4cfc6dc`): `web`, `api`, dos `nodo`, `mongo` y `redis` sanos y `seed` terminado con 0 |

### `seguridad/`

| Archivo | Qué muestra |
|---|---|
| `01-usuario-sin-permiso-cluster.png` | Un `usuario` que escribe `/cluster` en la barra: «Esta pantalla es solo para administradores» |
| `02-aviso-sesion-por-vencer.png` | El aviso que sale 5 minutos antes de que venza el token |
| `03-sesion-vencida-vuelve-al-ingreso.png` | Al vencer, la UI vuelve sola al ingreso y dice «La sesión venció.» |
| `politicas.txt` | Las políticas de seguridad de la API probadas con `curl` contra el mismo Compose (ver abajo) |

Para el aviso y el vencimiento se abrió una sesión con un token de vida corta (75 s) firmado con el secreto de
esta corrida: es lo mismo que pasa al final de la hora de un token normal, sin esperar una hora.

## 3. Mediciones (`pruebas/`)

**Las pruebas automáticas**, corridas sobre el mismo commit de las capturas más lo de esta rama:
`web-pruebas.txt` (lint sin errores, Vitest con 708 pruebas en 36 archivos, build) y `go-test-race.txt`
(`go vet` y `go test -race -count=1 ./...`, todos los paquetes en verde). Una primera corrida de `go test -race`
falló en `cmd/carga` por una prueba que exigía un p99 menor que 100 ms a un servidor de prueba con todos los
paquetes corriendo a la vez (dio 102 ms); se corrigió la prueba en `e84281e`, no el comando.

**Latencia de la API, con `cmd/carga`** (`POST /api/predicciones`, 5000 pedidos medidos por nivel, las 78
lecturas de prueba de Postman, 14 CPU lógicas). Se midió dos veces: directo a la API (`:8080`) y por el Caddy
de la UI (`:8081`), que es el camino del navegador. Ningún error, ninguna respuesta con otro régimen y ninguna
escritura perdida en MongoDB ni en Redis.

| Clientes | p50 directo | p99 directo | p50 por el Caddy | p99 por el Caddy |
|---:|---:|---:|---:|---:|
| 1 | 0.16 ms | 0.32 ms | 0.35 ms | 0.91 ms |
| 4 | 0.14 ms | 0.49 ms | 0.38 ms | 1.14 ms |
| 16 | 0.41 ms | 2.01 ms | 0.91 ms | 2.50 ms |
| 64 | 1.19 ms | 4.32 ms | 3.39 ms | 12.95 ms |

Archivos: `api_latencia_directa.json`, `api_latencia_caddy.json` (el formato de `reports/api_latencia.json`) y
lo que imprimió cada corrida, `carga_directa.txt` y `carga_caddy.txt`.

**Del pedido al panel por el WebSocket, con `medir_ws.mjs`** (Node 22, sin dependencias). Entra como invitado
por el Caddy, abre `/ws` igual que la UI y manda una por una las 132 lecturas reales de tres días de
`series.json` (CA-JUPR-01, CA-CG-02 y CA-MAC-01). Por cada una mide hasta la respuesta y hasta que llega su
`estacion.regimen`:

| | p50 | p95 | p99 | máx |
|---|---:|---:|---:|---:|
| Respuesta de `POST /api/predicciones` | 1.91 ms | 2.96 ms | 3.74 ms | 5.41 ms |
| Llegada del evento `estacion.regimen` | 2.19 ms | 3.96 ms | 4.44 ms | 4.70 ms |

Las 132 lecturas tuvieron su evento, y el último estado de cada día coincidió con lo `esperado` del archivo
(régimen, alerta, banda INCA y medias), que `scripts/impacto.py` calcula sin la API. Archivo: `ws_latencia.json`.

**Políticas de seguridad de la API** (`seguridad/politicas.txt`, todo por el Caddy de la UI):

- Cabeceras de la SPA: `Content-Security-Policy` estricta (solo el propio origen; `connect-src 'self'` cubre
  `/api` y `/ws`; `frame-ancestors 'none'`), `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`,
  sin `Server`. `/docs` lleva una política aparte para Swagger, con su script por hash.
- HSTS: el Caddy lo manda solo por TLS. En el Compose local (HTTP) no sale, a propósito; en el despliegue de
  Azure sí: `curl -I https://aire.rosewt.dev/` responde `strict-transport-security: max-age=31536000` (comprobado
  el mismo 27/11), y `http://` redirige con 308 a `https://`.
- JWT HS256 con roles: sin token, 401; un `usuario` en una ruta de admin, 403; un token vencido, uno con
  `alg=none` y uno con el rol cambiado a mano, 401.
- CORS: en el Compose la UI y la API comparten origen y `CORS_ORIGENES` está vacía, así que la API no manda
  `Access-Control-Allow-Origin` a nadie; un preflight desde otro origen recibe 403.
- WebSocket: el token viaja en `Sec-WebSocket-Protocol` (`aire.jwt`), no en la URL; desde otro origen, 403; sin
  token o con uno vencido, 401.
- Ingreso de invitado: `POST /api/auth/invitado` da un token de rol `usuario` de la cuenta que nombra
  `API_INVITADO`, sin que su clave pase por el navegador.

## 4. Cómo repetirlo

1. `cp .env.example .env`, completar `JWT_SECRETO` y `API_USUARIOS`, y poner `API_INVITADO=demo` y
   `JWT_DURACION=4h`. Con `GOLD_DIR` apuntando a la carpeta del gold.
2. `docker compose down -v` (sistema limpio: el escenario no se puede cargar sobre estados más nuevos) y
   `docker compose up --build -d --scale nodo=2`.
3. Recorrer la lista de la sección 1 en <http://localhost:8081>, en ese orden. El invitado, en otra ventana
   privada.
4. Las mediciones, **después** de las capturas (`cmd/carga` deja estados de 2022 en las estaciones):

   ```bash
   node docs/informe/capturas/tb2/pruebas/medir_ws.mjs http://localhost:8081 > docs/informe/capturas/tb2/pruebas/ws_latencia.json
   CARGA_CLAVE=… CARGA_CLAVE_ADMIN=… go run ./cmd/carga -url http://localhost:8081 -pedidos 5000 -salida docs/informe/capturas/tb2/pruebas/api_latencia_caddy.json
   ```

Las capturas de `andamiaje/` son anteriores (21/11), del armazón con las pantallas todavía en construcción.
