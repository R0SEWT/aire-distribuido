# TB2 — Diseño del frontend: panel de calidad del aire (SPA)

> **Borrador para revisar en equipo** (2026-11-21, con una revisión crítica contra el código ya
> incorporada). Lo marcado **[abierta]** es una decisión pendiente; lo marcado **[decisión mía]** es una
> elección del redactor que se puede revertir; lo marcado **[cerrada]** ya se decidió. Todo lo demás sale del
> enunciado (`../concurrente/materials/extra/CC65-TB2-1ACC0065_202620.pdf`), del contrato de la API
> (`docs/api/openapi.yaml`) o de lo ya construido en la PC4 (`docs/specs/2026-11-14-pc4-design.md`). El
> frontend es el Entregable 3 (TB2), que se entrega el **domingo 29/11 a las 23:30**.

## 1. Intención

La PC4 dejó un sistema que clasifica lecturas, avisa cuando la media de 24 horas de una estación supera el
ECA, entrena el modelo repartido en nodos y guarda todo. Nadie fuera del equipo puede verlo sin `curl`.

El frontend responde, para alguien que no programa, tres preguntas:

1. **¿Cuál es el último estado de cada estación, y cuáles están en alerta?**
2. **¿Qué significa eso para la gente que vive cerca?** (la parte D del enunciado: impacto social, con datos
   reales).
3. **¿Cómo se entrena el modelo en el clúster, y cómo se está portando el sistema?** (para quien administra).

Cada pantalla dice en una frase lo que el número significa y de dónde sale.

**No hay un «ahora».** Los datos de OEFA van de 2018 a 2022 y cada estación midió en campañas distintas. El
sistema no recibe lecturas en vivo: recibe las que alguien le manda. Por eso la UI nunca dice «ahora»: dice
**«último estado recibido»**, con su fecha, y para mostrar el sistema funcionando carga un **escenario**: un
tramo real en que muchas estaciones midieron a la vez (§5.4).

## 2. Lo que pide el curso

Del enunciado (parte D, «FrontEnd Web» y Entregable 3) y de la rúbrica («Interfaz», 5 de 20 puntos):

| Pedido | Dónde se cumple |
|---|---|
| SPA con un framework (nombra Angular o React) | React, §9 |
| Inicio de sesión / autenticación | §5.1 |
| Panel de usuario (dashboard) | §5.2 y §5.3 |
| Visualización de resultados: entrenamiento y simulación | §5.4 y §5.5 |
| Panel de administrador: métricas del clúster (uso de CPU, latencia, número de nodos, logs) | §5.6 |
| «Servir datos en tiempo real al frontend en formato JSON usando websockets» | Evento `estacion.regimen`, §6 y §9.4 |
| «Configura el aplicativo con parámetros ingresados desde la UI» | §5.4 (simulación) y §5.5 (entrenamiento) |
| Menú de opciones y experiencia de usuario adecuada | §5 y §8 |
| Impacto social «con datos reales»: qué problema resuelve y a cuántas personas podría beneficiar | §5.7 y §7 |
| Evaluación experimental que «responde a preguntas que atiende el impacto social» | §7.4 y §12 |
| Integrar el frontend con la API y configurar sus políticas de seguridad | §10 |
| Docker Compose despliega también la UI | §11 |
| Video de 6 minutos como máximo, con todos los integrantes | §13 |

## 3. Decisiones de base

- **[cerrada] Stack**: React con Vite, TypeScript y Tailwind; D3 para los gráficos y el mapa.
- **[cerrada] Impacto con población por distrito** del INEI. Cómo se presenta está en §7; la fuente exacta
  sigue **[abierta]**.
- **[cerrada] Línea visual**: la de los paneles de la carpeta de referencias de Rody: panel oscuro y denso,
  números grandes en monoespaciada, tarjetas con un borde de acento, pestañas en píldora y una frase en
  lenguaje llano por cada cifra (§8).
- **[decisión mía] Todo funciona sin internet**: el mapa se dibuja con D3 sobre un GeoJSON versionado, sin
  teselas de terceros, y las fuentes van empaquetadas. La excepción es Swagger en `/docs`, que hoy carga de
  un CDN.
- **[decisión mía] La UI se sirve desde el mismo origen que la API** (un solo Caddy sirve los estáticos y pasa
  `/api` y `/ws`): sin CORS en el despliegue (§11).
- El frontend no calcula nada del modelo: la alerta, la banda INCA y el régimen los da la API. Si la API y la
  pantalla difieren, manda la API.
- En español, con las mismas palabras del informe: régimen, alerta, banda INCA, media de 24 horas.

## 4. Quién lo usa

| Rol | Qué ve | Qué puede hacer |
|---|---|---|
| Sin sesión | La pantalla de ingreso | Entrar |
| `usuario` | Panel, estación, simulación e impacto | Consultar; simular un día de una estación |
| `admin` | Todo lo anterior más entrenamiento y clúster | Cargar el escenario; lanzar entrenamientos; ver nodos, métricas y eventos |

Los roles son los del JWT. La UI esconde lo que el rol no puede usar, pero quien decide es la API: una ruta
de admin responde 403 a un `usuario` aunque fuerce la URL.

La simulación **escribe estado compartido** (el último estado de la estación, para todos): la pantalla lo
dice antes de empezar.

## 5. Pantallas

Navegación: una barra superior con el nombre del sistema, el estado de la API (un punto verde, ámbar o rojo
desde `/api/salud`), el menú en píldoras y, a la derecha, el modo proyector y salir.

### 5.1 Ingreso

Usuario y clave contra `POST /api/auth/token`. Con un 401 dice «usuario o clave incorrectos», sin más. Si la
API no responde, dice eso y ofrece reintentar: no deja la pantalla en blanco.

No hay registro: las cuentas las configura quien levanta el sistema. Debajo del formulario, el botón «No
pienso crearme una cuenta aquí» abre un diálogo («Perdón, era parte de la nota») que deja entrar igual, como
invitado: `POST /api/auth/invitado` responde un token de la cuenta que el servidor designó con `API_INVITADO`,
de rol `usuario`, y la sesión se abre por el mismo camino que el ingreso normal. Las credenciales de esa
cuenta no pasan por el navegador. Si el servidor no la tiene (404), el diálogo lo dice.

### 5.2 Panel (inicio)

- **Mapa del Perú** con las 33 estaciones en su lugar. Cada estación está en uno de tres estados, cada uno
  con su color y su palabra:
  - **Con estado**: el color de su banda INCA y un anillo si está en alerta.
  - **Sin datos suficientes**: recibió lecturas, pero menos de 18 en la ventana.
  - **No mide PM por hora**: 16 de las 33 estaciones son de muestreo manual de 24 horas o no tienen PM2.5 y
    PM10 horarios; no están en el gold y nunca van a tener estado. Se dibujan apagadas, con esa leyenda.
- **Tres cifras arriba**: estaciones en alerta, estaciones con estado y la fecha del último estado recibido.
- **Lista de estaciones** ordenada por gravedad: alerta primero, después por banda INCA. Cada fila: código,
  distrito, tipo de actividad, media de 24 h de PM2.5 y PM10, banda, régimen y **la fecha de ese estado**.
- **Frase del panel**, con una plantilla fija: «En el último estado recibido hay 2 estaciones en alerta: su
  media de 24 horas supera el ECA».
- **Estado vacío**: con un sistema recién levantado ninguna estación tiene estado. El panel no muestra un
  mapa gris y tres ceros: explica por qué y ofrece **«cargar el escenario»** (admin) o ir a simulación.
- Se actualiza sola con el evento `estacion.regimen` (§6).

### 5.3 Estación

- **Tarjeta principal**: media de 24 h de PM2.5 y de PM10 en grande, cuántas lecturas tiene la ventana, si
  hay alerta, la banda INCA y el régimen de la última lectura. El «excede el ECA» del régimen se toma del
  estado guardado, no del modelo activo: si se entrenó otro modelo, los índices de régimen pueden haber
  cambiado.
- **La curva**: las lecturas que la UI vio en esta sesión (del escenario o de una simulación) y su media
  móvil de 24 h, con la línea del ECA (50 y 100 µg/m³) y las bandas INCA de fondo. Se elige PM2.5 o PM10.
  No hay una serie histórica desde la base (§6).
- **Frase en llano**: «La media de las últimas 24 horas de PM10 es 121 µg/m³: supera el ECA (100)».
- **«¿De dónde sale este número?»**, desplegable: la regla de la alerta, la norma (DS 003-2017-MINAM y RM
  181-2016-MINAM) y por qué el régimen no es la alerta.
- **Ficha**: objeto fiscalizado, tipo de actividad, distrito, provincia, departamento, las fechas en que
  midió y la población del distrito (§7).

### 5.4 Simulación y escenario

Es la «simulación» del enunciado y la forma de ver el sistema en marcha.

**El escenario** (admin, desde el panel o desde aquí): carga un tramo real en que muchas estaciones midieron a
la vez. Del 13 al 31 de diciembre de 2021, entre 10 y 14 de las 17 estaciones con datos tienen al menos 18
horas válidas cada día (comprobado sobre el gold). Se cargan **48 horas seguidas** de ese tramo: con 24 la
media recién aparece en la lectura 18 y es parcial.

**Simular un día** (usuario): se elige una de las 17 estaciones con datos, un día de `series.json` (§7.3) y
una velocidad. La UI manda las lecturas una por una a `POST /api/predicciones`, y la pantalla muestra cómo se
llena la ventana, cuándo aparece la media y en qué hora se enciende la alerta.

**Lectura a mano**: PM2.5 y PM10 para una de las 17 estaciones, a la hora siguiente a su último estado. No
se elige una fecha libre.

Reglas que salen de cómo funciona la API, y que la UI respeta en vez de descubrirlas:

- **El último estado de una estación no retrocede.** Si la estación ya tiene una lectura posterior al día que
  se quiere simular, la API responde bien pero el panel no cambia. La UI consulta el estado antes y, si pasa
  eso, lo dice y ofrece los días posteriores.
- **El estado persiste** (Redis guarda en disco). El Runner de Postman y `cmd/carga` dejan estaciones con
  fechas de 2022. El escenario se carga sobre un **sistema limpio** (`docker compose down -v`), y eso es un
  paso escrito de la demo y de las capturas.
- **La media depende de lo que la API tiene en memoria** (las últimas 72 lecturas por estación). Los días de
  `series.json` de una misma estación están separados por más de 48 horas, y cada uno trae sus 48 horas.
- **Fechas**: la API rechaza una fecha más de una hora adelante del reloj; con datos de 2021 no pasa.

**[abierta]** Un botón «reiniciar las estaciones» para admin (borra el último estado y las ventanas) evitaría
el `down -v`. Es una ruta nueva de la API; se hace solo si sobra tiempo.

### 5.5 Entrenamiento (admin)

- **Formulario**: nodos, réplicas y semilla, con los nodos libres a la vista. Los errores de la API se
  muestran con sus palabras: 409 «ya hay un entrenamiento en curso», 422 «hay 2 nodos libres en el pool y se
  pidieron 4».
- **Las rondas**: un entrenamiento real dura unos 50 milisegundos (44 rondas), así que no hay nada que ver
  «avanzar». La UI abre el WebSocket antes de lanzar, recibe todas las rondas y las **reproduce a ritmo
  humano**: la inercia por iteración (una curva que baja) y el tiempo de cada nodo por ronda (barras). La
  pantalla dice el tiempo real y que lo que se ve es una reproducción.
- **Preajuste «medición larga»**: unas 300 réplicas, que dura lo suficiente para verlo en vivo. Con réplicas
  mayores que 1 es una medición y no cambia el modelo activo; la pantalla lo dice.
- **Al terminar** pide `GET /api/entrenamientos/{id}` y muestra iteraciones, motivo de paro, diferencia con el
  secuencial, tiempo total y si el modelo quedó activo; si abortó, el error con el nodo que falló.
- Tras cada entrenamiento los nodos tardan unos 2 segundos en volver al pool: el formulario espera a que
  estén libres en vez de mostrar «0 nodos».
- **Historial** de entrenamientos con su estado.

### 5.6 Clúster (admin)

El enunciado pide «uso CPU, latencia, número de nodos, logs»:

- **Nodos**: nombre, dirección, workers, latencia y si está libre.
- **Uso de CPU**: el protocolo no transporta el CPU de cada nodo. Se muestran dos cosas que sí se miden, cada
  una con su nombre verdadero: el **CPU y la memoria del proceso coordinador** (la API los publica en
  `/api/metricas`, §6) y el **tiempo de respuesta de cada nodo por ronda**, medido desde el coordinador, que
  incluye la red.
- **Latencia de la API** por ruta: pedidos, p50, p95 y p99, con la línea de los 100 ms.
- **Escrituras**: escritas, descartadas y fallidas por almacén.
- **Eventos del clúster** (los «logs»): altas, bajas y errores de nodos, del más reciente al más antiguo. Una
  baja y un alta después de cada entrenamiento son normales, y la pantalla lo explica.
- **Salud**: API, MongoDB, Redis y modelo activo.

### 5.7 Impacto

La parte D. Una página para leer de arriba abajo, con datos reales del gold:

- **El problema**: qué es el ECA y qué es el INCA, en dos frases.
- **Cuánto tiempo**: de las horas con media de 24 h calculable, cuántas superaron el ECA y cómo se reparten
  entre las bandas INCA.
- **Dónde y junto a qué**: por estación, con su objeto fiscalizado, su tipo de actividad y **las semanas en
  que midió**: horas sobre el ECA y banda más frecuente. Es la unidad honesta: una estación vigila una
  actividad.
- **Quién vive ahí**: la población del distrito de cada estación, como contexto de esa fila. **Sin una suma
  titular de personas** (§7.2).
- **¿Sirve la alerta?**: la alerta por régimen acierta el 35 % de las veces que suena (precisión) y detecta
  el 44 % de las horas sobre el ECA (cobertura); la de la media de 24 h es la norma misma.
- **¿Con cuánta anticipación?** (§7.4).
- **Lo que este dato no puede decir**, a la vista y no en letra chica (§7.2).

Un pie de página en toda la app dice de dónde salen los datos (OEFA, licencia ODC-By) y enlaza a Swagger, al
repositorio y al informe. No hay una pantalla «Acerca de».

### 5.8 Estados que toda pantalla tiene

Cargando, vacío, error de la API con reintento, sesión vencida (vuelve al ingreso) y sin permiso.

## 6. Lo que la API tiene que agregar

Hoy la API alcanza para el ingreso, la simulación y el entrenamiento. Un `usuario` conectado al WebSocket no
recibe nada: los únicos eventos son los de entrenamiento, y son de admin. Hay que agregar:

| Qué | Para qué | Notas |
|---|---|---|
| `GET /api/regimenes` | Panel: el estado de todas las estaciones en un pedido | Hoy son 33 pedidos, casi todos 404 al arrancar |
| Evento `estacion.regimen` por WebSocket | El «tiempo real» que piden el enunciado y la rúbrica | Para `usuario` y `admin`. Lleva `estacion` (el `RegimenEstacion` no la trae). Solo se emite si la lectura es la más reciente de la estación: si no, el evento y el GET dirían cosas distintas. Agrupado por estación, para que una ráfaga no desconecte a los clientes lentos |
| `GET /api/cluster/eventos?limite` | Clúster: los «logs» | Admin. La lectura desde MongoDB ya existe; falta la interfaz, el doble en memoria, la ruta y el contrato |
| CPU y memoria del coordinador en `/api/metricas` | Clúster: «uso de CPU» | `kmeans.MedirRecursos` ya existe |

Estas cuatro **no se recortan**: son el tiempo real y el panel de administrador que la rúbrica nombra. Cada
una entra primero al contrato, con sus pruebas, y se suma a la colección de Postman y a `tablas.py`, que
compara el contrato con el servidor.

**Recortado**: la serie histórica de una estación desde MongoDB. La colección `predicciones` no tiene clave
única (una simulación repetida duplica horas), su índice es por momento de escritura y ya guarda miles de
predicciones de las pruebas de carga. La curva de §5.3 se arma con lo que la UI vio.

**No es una ruta**: los datos de impacto van como un archivo estático de la UI (§7), no por la API.

## 7. Datos de impacto

### 7.1 Población por distrito

De las 33 estaciones, **17 tienen PM por hora** (las del gold), en **10 distritos**. Las otras 16 no entran
a ninguna cifra de impacto, y la página lo dice. `CA-HUAR-01` no trae distrito en la fuente.

Se agrega `reports/poblacion_distritos.csv`, versionado: `ubigeo, departamento, provincia, distrito,
poblacion, anio, fuente`, con los 16 distritos de la ficha (los 10 con datos se usan en impacto; los demás,
en la ficha de su estación). Se cruza por `ubigeo`, no por nombre: `LURIGANCHO` (150118) no es San Juan de
Lurigancho.

**[abierta]** La fuente: la población censada del Censo 2017 del INEI, o las proyecciones del INEI al año de
los datos. Recomendación: el censo, que es un conteo; el informe dice el año.

Un script de análisis (`scripts/impacto.py`, junto a los verificadores que ya están ahí, con sus
comprobaciones) cruza el gold, las asignaciones, la ficha y la población, y escribe `reports/impacto.json`,
que se copia a `web/public/datos/`. Depende del gold y del silver, que no se versionan: el archivo generado sí
se versiona, como los demás reports. Ninguna cifra de la pantalla se escribe a mano.

### 7.2 Lo que no se puede afirmar

- Que alguien viva en un distrito con una estación en alerta **no** dice que respiró ese aire: las estaciones
  de OEFA vigilan actividades fiscalizadas (una mina, una refinería, una ladrillera) y miden cerca de ellas.
- **No se suma la población.** Entre los 10 distritos, uno grande decide el total: Lurigancho-Chosica, por
  dos estaciones junto a ladrilleras. Un número grande al lado de «horas sobre el ECA» se lee como «personas
  afectadas», se escriba como se escriba. La población va fila por fila, como contexto.
- Varias estaciones son campañas de pocas semanas: sus horas sobre el ECA describen esas semanas.
- Un distrito sin estación, o con una estación que no mide PM por hora, no es un distrito con aire limpio.
- El enunciado pregunta «a cuántas personas podría **beneficiar**»: se responde con quiénes tendrían un aviso
  que hoy no tienen (los distritos con estación horaria), no con cuántos estuvieron expuestos.

### 7.3 Series para la simulación y el escenario

`web/public/datos/series.json`, generado desde el silver por el mismo script:

- **El escenario**: 48 horas reales seguidas de diciembre de 2021 para las estaciones que midieron a la vez.
- **Días sueltos**: unos 8, de estaciones distintas (días limpios, días que cruzan el ECA y días con huecos),
  cada uno con sus 48 horas y lo que la API debería responder al final.
- Para cada estación, las fechas son **posteriores** a las del escenario y están separadas por más de 48
  horas entre sí, con la misma regla de `docs/postman/generar_lecturas.py`.

Sirven también de datos de prueba de la UI y de la evaluación experimental (§12).

### 7.4 Anticipación **[abierta]**

El informe de la PC3 dejó escrito que «con cuánta anticipación y con qué acierto avisa» era la evaluación
experimental del TB2. El acierto ya está (§5.7). Falta la anticipación.

Recomendación: una versión acotada, calculada sobre el gold por el mismo script: para cada vez que la media
de 24 h de una estación cruza el ECA, **cuántas horas antes** la lectura horaria ya superaba el umbral y
cuántas horas antes la estación había entrado al régimen alto; se reporta la mediana y el reparto, por
estación. Se dice lo que no es: no hay partición temporal ni reentrenamiento sin fugas, que era el diseño
completo de la PC3.

La alternativa es no hacerla y decir en el informe, con esas palabras, que quedó sin responder.

## 8. Diseño visual

Referencias: las capturas de `referencias-diseno/` de Rody (no se versionan), sobre todo su app de pronóstico
y los paneles de COES y CAISO. De ahí sale la línea:

- **Oscuro**, con **modo proyector** (tipografía más grande y más contraste, para la sustentación). El modo
  claro queda fuera.
- **Tarjetas** de fondo apenas más claro que la página, con un **borde izquierdo de acento** en la que manda.
- **Números grandes en monoespaciada** (IBM Plex Mono), con su unidad chica al lado. Texto en Inter. Las dos
  fuentes van empaquetadas en la app.
- **Una frase en llano por cifra**, en un recuadro con borde de color: qué significa, comparado con qué.
- **Pestañas en píldora** y controles agrupados a la izquierda; el gráfico a la derecha.
- **«¿De dónde sale este número?»** como desplegable en las cifras que lo necesitan.

Colores, como tokens de Tailwind:

| Uso | Color |
|---|---|
| INCA buena | `#1BAF7A` |
| INCA moderada | `#E0B43A` **[decisión mía]** |
| INCA mala | `#EB6834` |
| INCA umbral de cuidado y alerta | `#D03B3B` |
| Sin datos suficientes | gris claro |
| No mide PM por hora | gris apagado, con trama |
| Acento y enlaces | `#2A78D6` |

Son los del deck de la PC3 donde había uno. El color nunca va solo: cada estado lleva su palabra o su icono.

## 9. Arquitectura del frontend

### 9.1 Stack

React 19, Vite, TypeScript estricto, Tailwind 4, React Router y D3 (escalas, formas y geografía). Para los
pedidos, TanStack Query. Sin biblioteca de componentes. Vive en `web/`.

### 9.2 Estructura

```text
web/
  src/
    api/          cliente tipado desde el contrato, WebSocket, errores
    sesion/       token, rol, guardas de ruta
    componentes/  Tarjeta, Cifra, Frase, Banda, Pildoras, Tabla, Estado…
    graficos/     Mapa, Curva, Inercia, TiemposPorNodo (D3)
    pantallas/    Ingreso, Panel, Estacion, Simulacion, Entrenamiento, Cluster, Impacto
    estilos/      tokens y modo proyector
  public/datos/   peru.geojson, series.json, impacto.json
```

### 9.3 Tipos desde el contrato

Los tipos de TypeScript se **generan** de `docs/api/openapi.yaml` (`openapi-typescript`) y se versionan. Los
eventos del WebSocket no salen solos (el contrato los describe en una extensión): se escriben a mano contra
los mismos esquemas. La comprobación automática en el CI de que los tipos están al día queda fuera.

### 9.4 Tiempo real

Un solo WebSocket por sesión, con el token en `Sec-WebSocket-Protocol` (`aire.jwt`), no en la URL. Los
eventos actualizan la caché de TanStack Query: `estacion.regimen` (todos) y `entrenamiento.ronda` y
`entrenamiento.fin` (admin). Reconecta con espera creciente y muestra el estado de la conexión, pero **no
reintenta** si el servidor cerró con 1008 (token vencido) ni pasada la hora de `expira`: vuelve al ingreso.

### 9.5 Errores

Un solo lugar traduce los errores de la API (`{"error": "…"}`) a los estados de §5.8. Un 401 con sesión
iniciada la cierra; un 503 se muestra como «el servicio no está disponible», no como un fallo de la UI.

## 10. Seguridad

- **El token va en `sessionStorage`**: sobrevive a recargar la página (un F5 en plena sustentación no saca
  de la sesión) y se borra al cerrar la pestaña. No va en `localStorage`.
- **El token dura una hora** y no se renueva. Para la demo se sube la duración en el Compose
  (`-jwt-duracion`), y la UI avisa antes de que venza.
- **Mismo origen** en Compose: sin CORS. `CORS_ORIGENES` queda para el desarrollo con Vite en otro puerto.
- **Cabeceras** desde Caddy, sobre la SPA: `Content-Security-Policy` (scripts, estilos y fuentes solo del
  propio origen; `connect-src 'self'`, que cubre el WebSocket con y sin TLS; `frame-ancestors 'none'`),
  `X-Content-Type-Options` y `Referrer-Policy`. `/docs` lleva una política aparte, porque Swagger carga de
  un CDN con un script en línea.
- La UI esconde lo de admin, pero la autorización es de la API (403).
- Ningún secreto en el bundle ni en el repo. Las credenciales de la demo van en el anexo del informe.
- Es el punto «configuración de políticas de seguridad de API» del Entregable 3: va al informe con lo que se
  configuró y cómo se probó.

## 11. Despliegue

- **Compose**: un servicio `web`, que es **un solo Caddy**: sirve los estáticos de `web/dist` y pasa `/api`,
  `/ws`, `/docs` y `/openapi.yaml` a `api`. `docker compose up` levanta la UI junto con todo, como pide el
  enunciado. La imagen se construye en dos etapas (Node para compilar, Caddy para servir).
- **Desarrollo**: `pnpm dev` con Vite y un proxy a la API local.
- **Máquina de Azure**: fuera del TB2. Si se hace, `web` reemplaza al `caddy` del override de
  `despliegue/azure/` y la imagen se construye en la laptop, porque esa máquina no puede compilar Node.
- El repo suma Node solo para `web/`. El CI gana un trabajo: lint, pruebas y build.

## 12. Pruebas y evidencias

- **Unitarias** (Vitest y Testing Library): las reglas de presentación (los tres estados de una estación, el
  orden por gravedad, las frases, los colores por banda) y los estados de §5.8.
- **De punta a punta, a mano**: una lista escrita que se recorre contra el Compose limpio: ingresar, cargar
  el escenario y ver el panel llenarse, simular un día que termina en alerta, lanzar un entrenamiento, ver el
  clúster, y que un `usuario` no entra a lo de admin. Cada paso deja su captura real. Playwright queda fuera.
- **Evaluación experimental** del Entregable 3: las series de §7.3 con su resultado esperado contra lo que
  mostró la pantalla; cuánto tarda una lectura en verse en el panel por el WebSocket; y la anticipación de
  §7.4 si se hace.
- **Capturas reales** para el informe y el video, contra el Compose local y limpio.

## 13. Alcance, reparto y calendario

### 13.1 Cuánto trabajo es

Una estimación honesta de todo lo que describía el primer borrador da entre 17 y 19 días de una persona, y
hay seis días hábiles (del lunes 23 al sábado 28; Rody no trabaja los domingos). Con este alcance recortado
baja a unos 11 o 12, que solo cierra si las pantallas y las rutas de la API se trabajan **en paralelo** y la
revisión no se acumula al final.

| Bloque | Días |
|---|---|
| API: `GET /api/regimenes`, evento `estacion.regimen`, eventos del clúster, CPU en métricas | 2 |
| Andamiaje, tokens, ingreso, guardas, errores, WebSocket, tipos | 1,5 |
| Servicio `web` en Compose con sus cabeceras | 0,5 |
| Panel con el mapa y los tres estados | 1,5 |
| Estación y curva | 0,75 |
| Simulación, escenario y `series.json` | 1,25 |
| Entrenamiento | 1 |
| Clúster | 0,5 |
| Impacto: script, población y página | 1,5 |
| Pruebas unitarias y lista de punta a punta | 0,5 |
| Informe del TB2 | 1,5 |
| Video | 1 |

### 13.2 Qué queda fuera, en este orden si aun así falta tiempo

Ya fuera: Playwright, la serie histórica desde MongoDB, el modo claro, la UI en Azure, la comprobación de
tipos en el CI, la pantalla «Acerca de» y la suma de población.

Lo siguiente en salir, sin perder puntos de la rúbrica: la anticipación de §7.4 (se declara sin responder),
el preajuste de medición larga, el modo proyector y los días sueltos de la simulación (queda el escenario).

No sale: el ingreso, el panel con el mapa, el tiempo real por WebSocket, el entrenamiento, el clúster con sus
cuatro métricas, el impacto y la UI en Compose.

### 13.3 Calendario **[abierta]**

| Día | Hito |
|---|---|
| sáb 21 | Entrega de la PC4. Esta spec en revisión |
| lun 23 | Decisiones de §14 cerradas. Contrato con lo de §6. Andamiaje, ingreso y servicio `web`. Guion del video repartido |
| mar 24 | `GET /api/regimenes` y `estacion.regimen`. Panel con el mapa |
| mié 25 | Estación, simulación y escenario |
| jue 26 | Entrenamiento y clúster. Los compañeros graban su parte del video |
| vie 27 | Impacto. Lista de punta a punta, capturas e informe del TB2 |
| sáb 28 | Montaje del video, release y entrega |

### 13.4 Tareas que se pueden repartir **[abierta]**

Cada una con su PR y su criterio de aceptación, y ninguna frena al resto:

- **Población por distrito**: `reports/poblacion_distritos.csv` con los 16 distritos, la fuente y el año; se
  acepta si cada cifra se puede abrir en la página del INEI y el `ubigeo` coincide con la ficha.
- **Textos de impacto y de «¿de dónde sale este número?»**: revisados contra el informe; se aceptan si no
  afirman nada de §7.2.
- **Guion del video y la parte de cada uno**: 6 minutos como máximo y participan todos, como pide el
  enunciado. Sin video, la rúbrica castiga todo lo demás.

## 14. Decisiones abiertas

1. Fuente y año de la población (§7.1).
2. La anticipación: versión acotada, o declararla sin responder (§7.4).
3. El botón «reiniciar las estaciones» (§5.4).
4. Reparto de tareas y calendario (§13).

## 15. Notas para quien implemente

- El texto real del 422 es «hay 2 nodos libres en el pool y se pidieron 4»; el nodo que falló llega dentro
  del texto de `error`.
- Con el proxy de Vite no usar `changeOrigin`: la API compara el origen y el WebSocket respondería 403.
- D3 con `.style()` y clases, no con `style` como atributo, para respetar la CSP.
- Las rutas nuevas tocan también la colección de Postman y `docs/informe/tablas.py`.
- El GeoJSON: solo departamentos, simplificado, con su licencia anotada.
- La colección de Postman espera un 404 de una estación sin lecturas (`CA-01`): por eso la lectura a mano no
  admite las 16 estaciones que no miden PM por hora.

## 16. Fuera de alcance

- Pronóstico de la calidad del aire: el sistema clasifica y avisa sobre lo medido.
- Alta de usuarios, cambio de clave y administración de cuentas.
- Aplicación móvil nativa; la SPA se diseña para escritorio y proyector.
- Editar el modelo o las estaciones desde la UI.
- El experimento del mapa nacional (PR #52): con datos sintéticos no es impacto con datos reales.
