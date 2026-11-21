# TB2 — Diseño del frontend: panel de calidad del aire (SPA)

> **Borrador para revisar en equipo** (2026-11-21). Lo marcado **[abierta]** es una decisión pendiente; lo
> marcado **[decisión mía]** es una elección del redactor que se puede revertir; lo marcado **[cerrada]** ya
> se decidió. Todo lo demás sale del enunciado (`../concurrente/materials/extra/CC65-TB2-1ACC0065_202620.pdf`),
> del contrato de la API (`docs/api/openapi.yaml`) o de lo ya construido en la PC4
> (`docs/specs/2026-11-14-pc4-design.md`). El frontend es el Entregable 3 (TB2), que se entrega el
> **domingo 29/11 a las 23:30**.

## 1. Intención

La PC4 dejó un sistema que clasifica lecturas, avisa cuando la media de 24 horas de una estación supera el
ECA, entrena el modelo repartido en nodos y guarda todo. Nadie fuera del equipo puede verlo sin `curl`.

El frontend responde, para alguien que no programa, tres preguntas:

1. **¿Cómo está el aire ahora en cada estación, y hay alguna en alerta?**
2. **¿Qué significa eso para la gente que vive ahí?** (la parte D del enunciado: impacto social, con datos
   reales).
3. **¿Cómo se entrena el modelo en el clúster, y cómo se está portando el sistema?** (para quien administra).

No es un tablero de adorno: cada pantalla dice en una frase lo que el número significa y de dónde sale.

## 2. Lo que pide el curso

Del enunciado (parte D, «FrontEnd Web» y Entregable 3) y de la rúbrica («Interfaz», 5 de 20 puntos):

| Pedido | Dónde se cumple |
|---|---|
| SPA con un framework (nombra Angular o React) | React, §9 |
| Inicio de sesión / autenticación | §5.1 |
| Panel de usuario (dashboard) | §5.2 y §5.3 |
| Visualización de resultados: entrenamiento y simulación | §5.4 y §5.5 |
| Panel de administrador: métricas del clúster (uso de CPU, latencia, número de nodos, logs) | §5.6 |
| Predicciones en tiempo real | WebSocket, §6 y §9.4 |
| «Configura el aplicativo con parámetros ingresados desde la UI» | §5.4 (simulación) y §5.5 (entrenamiento) |
| Menú de opciones y experiencia de usuario adecuada | §5 y §8 |
| Impacto social «con datos reales»: qué problema resuelve y a cuántas personas podría beneficiar | §5.7 y §7 |
| Integrar el frontend con la API y configurar sus políticas de seguridad | §10 |
| Docker Compose despliega también la UI | §11 |

## 3. Decisiones de base

- **[cerrada] Stack**: React con Vite, TypeScript y Tailwind; D3 para los gráficos y el mapa.
- **[cerrada] Impacto**: nuestros datos más la **población por distrito** del INEI (§7).
- **[cerrada] Línea visual**: la de los paneles de la carpeta de referencias de Rody: panel oscuro y denso,
  números grandes en monoespaciada, tarjetas con un borde de acento, pestañas en píldora y una frase en
  lenguaje llano por cada cifra (§8).
- **[decisión mía] El mapa se dibuja con D3 sobre un GeoJSON versionado**, sin teselas de terceros: funciona
  sin internet en el aula y no depende de una clave de API.
- **[decisión mía] La UI se sirve desde el mismo origen que la API** (Caddy sirve los estáticos y pasa `/api`
  y `/ws`): no hace falta CORS en el despliegue y el WebSocket no cruza orígenes (§11).
- El frontend no calcula nada del modelo: la alerta, la banda INCA y el régimen los da la API. Si la API y la
  pantalla difieren, manda la API.
- En español, con las mismas palabras del informe: régimen, alerta, banda INCA, media de 24 horas.

## 4. Quién lo usa

| Rol | Qué ve | Qué puede hacer |
|---|---|---|
| Sin sesión | La pantalla de ingreso | Entrar |
| `usuario` | Panel, estación, simulación, impacto, acerca de | Consultar; correr una simulación |
| `admin` | Todo lo anterior más entrenamiento y clúster | Lanzar entrenamientos; ver nodos, métricas y eventos |

Los roles son los del JWT. La UI esconde lo que el rol no puede usar, pero quien decide es la API: una ruta
de admin responde 403 a un `usuario` aunque fuerce la URL.

## 5. Pantallas

Navegación: una barra superior con el nombre del sistema, el estado de la API (un punto verde, ámbar o rojo
desde `/api/salud`), el menú en píldoras y, a la derecha, el modo claro u oscuro, el modo proyector y salir.

### 5.1 Ingreso

Usuario y clave contra `POST /api/auth/token`. Con un 401 dice «usuario o clave incorrectos», sin más. Si la
API no responde, dice eso y ofrece reintentar: no deja la pantalla en blanco.

### 5.2 Panel (inicio)

La respuesta a «¿cómo está el aire ahora?».

- **Mapa del Perú** con las 33 estaciones en su lugar. Cada punto lleva el color de su banda INCA y un anillo
  si está en alerta; las que todavía no juntan 18 lecturas van en gris, con la leyenda «sin datos
  suficientes». Al tocar una estación se abre su ficha (§5.3).
- **Tres cifras arriba**: estaciones en alerta ahora, estaciones con datos suficientes y la hora de la última
  lectura recibida.
- **Lista de estaciones** ordenada por gravedad: alerta primero, después por banda INCA. Cada fila: código,
  distrito, tipo de actividad, media de 24 h de PM2.5 y PM10, banda y régimen.
- **Frase del día**, generada con una plantilla fija sobre esos datos: «Ahora hay 2 estaciones en alerta: su
  media de 24 horas supera el ECA».
- Se actualiza sola cuando llega una lectura (§6, evento `estacion.regimen`).

### 5.3 Estación

- **Tarjeta principal**: media de 24 h de PM2.5 y de PM10 en grande, cuántas lecturas tiene la ventana, si
  hay alerta, la banda INCA y el régimen de la última lectura con su perfil.
- **La curva**: las lecturas horarias y su media móvil de 24 h, con la línea del ECA (50 y 100 µg/m³) y las
  bandas INCA como franjas de fondo. Se elige PM2.5 o PM10.
- **Frase en llano**: «La media de las últimas 24 horas de PM10 es 121 µg/m³: supera el ECA (100)».
- **«¿De dónde sale este número?»**, desplegable: la regla de la alerta, la norma (DS 003-2017-MINAM y RM
  181-2016-MINAM) y por qué el régimen no es la alerta.
- **Ficha**: objeto fiscalizado, tipo de actividad, distrito, provincia, departamento y población del
  distrito (§7).

### 5.4 Simulación («¿Y si…?»)

Es «simulación» del enunciado y la forma de ver el tiempo real sin esperar un día.

- Se elige una **estación y un día real** de un conjunto de series versionado (§7.3) y una **velocidad**. La
  UI manda las lecturas de ese día, una por una, a `POST /api/predicciones`, y la pantalla muestra cómo se
  llena la ventana, cuándo aparece la media y en qué hora se enciende la alerta.
- **Lectura a mano**: un formulario con PM2.5 y PM10 para una estación y una hora, con la respuesta completa
  de la API y su tiempo en milisegundos.
- Las lecturas de una simulación son reales pero de otra fecha: la pantalla lo dice, y el panel las muestra
  como lo que son.

### 5.5 Entrenamiento (admin)

- **Formulario**: nodos, réplicas y semilla, con los nodos libres a la vista. Los errores de la API se
  muestran con sus palabras: 409 «ya hay un entrenamiento en curso», 422 «hay 2 nodos libres y se pidieron 4».
- **En vivo**, por WebSocket: la inercia por iteración (una curva que baja) y el tiempo de cada nodo en cada
  ronda (barras por nodo). Al terminar: iteraciones, motivo de paro, diferencia con el secuencial y tiempo
  total; si abortó, el nodo que falló.
- **Resultado**: los regímenes del modelo (PM2.5 y PM10 medios, tamaño y si excede el ECA) y si el modelo
  quedó activo o fue una medición (réplicas > 1).
- **Historial** de entrenamientos con su estado.

### 5.6 Clúster (admin)

- **Nodos**: nombre, dirección, workers, latencia y si está libre.
- **«Uso de CPU»**: el enunciado lo pide. El protocolo no transporta el porcentaje de CPU de cada nodo, así
  que se muestra el **tiempo de cómputo de cada nodo por ronda** del último entrenamiento, que sí se mide, y
  la pantalla lo dice con esas palabras (es la recomendación de la spec de la PC4, §13).
- **Latencia de la API** por ruta: pedidos, p50, p95 y p99, con la línea de los 100 ms.
- **Escrituras**: escritas, descartadas y fallidas por almacén.
- **Eventos del clúster** (los «logs»): altas, bajas y errores de nodos, del más reciente al más antiguo.
- **Salud**: API, MongoDB, Redis y modelo activo.

### 5.7 Impacto

La parte D. Una página para leer de arriba abajo, con datos reales del gold:

- **El problema**: qué es el ECA y qué es el INCA, en dos frases.
- **Cuánto tiempo**: de las horas con media de 24 h calculable, cuántas superaron el ECA y cómo se reparten
  entre las bandas INCA (`reports/alerta_24h.json`).
- **Dónde**: por estación y por tipo de actividad (minería, zona industrial, ladrilleras…), las horas sobre el
  ECA y la banda más frecuente.
- **A quiénes**: la población de los distritos con estación, y cuántas personas viven en distritos donde una
  estación registró horas sobre el ECA (§7).
- **Por qué importa el sistema**: la alerta por régimen acierta el 35 % de las veces; la de la media de 24 h
  es la norma misma. Se muestra la comparación.
- **Lo que este dato no puede decir**, a la vista y no en letra chica (§7.2).

### 5.8 Acerca de

Qué es el sistema, de dónde salen los datos (OEFA, licencia ODC-By), el equipo, y enlaces a Swagger, al
repositorio y al informe.

### 5.9 Estados que toda pantalla tiene

Cargando, vacío («todavía no hay lecturas de esta estación»), error de la API con reintento, sesión vencida
(vuelve al ingreso y conserva a dónde iba) y sin permiso.

## 6. Lo que la API tiene que agregar

Hoy la API alcanza para el ingreso, la simulación, el entrenamiento en vivo y parte del clúster. Falta:

| Qué | Para qué pantalla | Propuesta |
|---|---|---|
| El último régimen de **todas** las estaciones en un pedido | Panel | `GET /api/regimenes` → lista de `{estacion, …RegimenEstacion}`; hoy son 33 pedidos |
| La serie de una estación | Estación | `GET /api/estaciones/{id}/predicciones?desde&hasta&limite` desde MongoDB, por `fecha_hora` |
| Aviso en vivo cuando cambia una estación | Panel y estación | Evento WebSocket `estacion.regimen` con el `RegimenEstacion`, para `usuario` y `admin` |
| Eventos del clúster | Clúster | `GET /api/cluster/eventos?limite` (admin), desde `eventos_cluster` |
| Los datos de impacto | Impacto | `GET /api/impacto` sirve `reports/impacto.json` tal cual (§7) |

Cada una entra al contrato (`docs/api/openapi.yaml`) antes de implementarse, con sus pruebas, y no cambia nada
de lo que ya existe. Mientras no esté el evento `estacion.regimen`, el panel consulta cada 5 segundos.

**[abierta]** Si no alcanza el tiempo, el orden de recorte es: eventos del clúster, serie de la estación (la
curva se arma solo con lo que la UI vio en la sesión) y `GET /api/regimenes` (se queda en 33 pedidos).

## 7. Datos de impacto

### 7.1 Población por distrito

Las 33 estaciones están en **16 distritos**; una estación (`CA-HUAR-01`) no trae distrito en la fuente y
queda fuera de las cifras de población, y se dice. Se agrega `reports/poblacion_distritos.csv`, versionado:
`ubigeo, departamento, provincia, distrito, poblacion, anio, fuente`. Son 16 filas de un dato público.

**[abierta]** La fuente: el Censo 2017 del INEI (población censada por distrito) o las proyecciones del INEI
al año de los datos. Recomendación: el censo, que es un conteo y no una estimación; el informe dice el año.

Un script (`scripts/impacto.py`, con sus comprobaciones) cruza el gold, las asignaciones, la ficha y la
población, y escribe `reports/impacto.json`: por estación, por distrito y por tipo de actividad, las horas
calculables, las horas sobre el ECA y el reparto por banda INCA; y los totales. Ninguna cifra de la pantalla
se escribe a mano.

### 7.2 Lo que no se puede afirmar

- Que alguien viva en un distrito con una estación en alerta **no** dice que respiró ese aire: las estaciones
  de OEFA vigilan actividades fiscalizadas (una mina, una refinería, una ladrillera) y miden cerca de ellas.
- Varias estaciones son campañas de pocas semanas: sus horas sobre el ECA describen esas semanas.
- Un distrito sin estación no es un distrito con aire limpio.
- La página dice «personas que viven en distritos donde una estación registró…», nunca «personas afectadas».

### 7.3 Series para la simulación

`web/public/datos/series.json`, generado desde el silver por el mismo script: unos 12 días reales de
estaciones distintas (días limpios, días que cruzan el ECA y días con huecos), cada uno con sus 24 lecturas y
lo que la API debería responder al final. Sirven también de datos de prueba de la UI.

## 8. Diseño visual

Referencias: las capturas de `referencias-diseno/` de Rody (no se versionan), sobre todo su app de pronóstico
y los paneles de COES y CAISO. De ahí sale la línea:

- **Oscuro por defecto**, con modo claro y **modo proyector** (tipografía más grande y más contraste, para la
  sustentación).
- **Tarjetas** de fondo apenas más claro que la página, con un **borde izquierdo de acento** en la que manda.
- **Números grandes en monoespaciada** (IBM Plex Mono), con su unidad chica al lado. Texto en Inter.
- **Una frase en llano por cifra**, en un recuadro con borde de color: qué significa, comparado con qué.
- **Pestañas en píldora** y controles agrupados a la izquierda; el gráfico a la derecha.
- **«¿De dónde sale este número?»** como desplegable en cada cifra que lo necesite.
- **Tabla detrás de cada gráfico** («ver los datos en una tabla»): accesible y útil para las capturas.

Colores, como tokens de Tailwind:

| Uso | Color |
|---|---|
| INCA buena | `#1BAF7A` |
| INCA moderada | `#E0B43A` **[decisión mía]** |
| INCA mala | `#EB6834` |
| INCA umbral de cuidado y alerta | `#D03B3B` |
| Sin datos suficientes | gris |
| Acento y enlaces | `#2A78D6` |

Son los del deck de la PC3 donde había uno. El color nunca va solo: cada estado lleva su palabra o su icono.
Contraste AA y navegación por teclado.

## 9. Arquitectura del frontend

### 9.1 Stack

React 19, Vite, TypeScript estricto, Tailwind 4, React Router y D3 (escalas, formas y geografía). Para los
pedidos, TanStack Query (caché, reintentos y consulta periódica). Sin biblioteca de componentes: los
componentes son pocos y propios. Vive en `web/`.

### 9.2 Estructura

```text
web/
  src/
    api/          cliente tipado desde el contrato, WebSocket, errores
    sesion/       token, rol, guardas de ruta
    componentes/  Tarjeta, Cifra, Frase, Banda, Pildoras, Tabla, Estado…
    graficos/     Mapa, Curva, Inercia, TiemposPorNodo (D3)
    pantallas/    Ingreso, Panel, Estacion, Simulacion, Entrenamiento, Cluster, Impacto, Acerca
    estilos/      tokens y temas
  public/datos/   peru.geojson, series.json
```

### 9.3 Tipos desde el contrato

Los tipos de TypeScript se **generan** de `docs/api/openapi.yaml` (`openapi-typescript`) y se versionan; el
CI falla si el contrato cambió y los tipos no. Así la UI no puede desviarse de la API sin que se note.

### 9.4 Tiempo real

Un solo WebSocket por sesión, con el token en `Sec-WebSocket-Protocol` (`aire.jwt`), no en la URL. Reconecta
con espera creciente y muestra el estado de la conexión. Los eventos actualizan la caché de TanStack Query:
`entrenamiento.ronda` y `entrenamiento.fin` (admin), y `estacion.regimen` cuando exista (§6).

### 9.5 Errores

Un solo lugar traduce los errores de la API (`{"error": "…"}`) a los estados de §5.9. Un 401 con sesión
iniciada la cierra; un 503 se muestra como «el servicio no está disponible», no como un fallo de la UI.

## 10. Seguridad

- **El token vive en memoria**, no en `localStorage`: recargar la página pide ingresar otra vez. **[decisión
  mía]**; `sessionStorage` sería más cómodo y algo menos seguro.
- **Mismo origen** en Compose y en la máquina desplegada: sin CORS. `CORS_ORIGENES` queda solo para el
  desarrollo con Vite en otro puerto, o si alguna vez se publica la UI aparte.
- **Cabeceras** desde Caddy: `Content-Security-Policy` (solo el propio origen y `wss:` propio), `X-Frame-Options`,
  `Referrer-Policy` y `Strict-Transport-Security` con HTTPS.
- La UI esconde lo de admin, pero la autorización es de la API (403).
- Ningún secreto en el bundle ni en el repo: las claves de demostración se dicen en la sustentación, no se
  escriben en la pantalla de ingreso.
- Es el punto «configuración de políticas de seguridad de API» del Entregable 3: va al informe con lo que se
  configuró y cómo se probó.

## 11. Despliegue

- **Compose**: un servicio `web` (Caddy con los estáticos de `web/dist`) que además pasa `/api`, `/ws`, `/docs`
  y `/openapi.yaml` a `api`. `docker compose up` levanta la UI junto con todo, como pide el enunciado. La
  imagen se construye en dos etapas (Node para compilar, Caddy para servir).
- **Máquina de Azure**: el mismo Caddy de `despliegue/azure/`, que pasa a servir también la UI.
- **Desarrollo**: `pnpm dev` con Vite y un proxy a la API local.
- El repo suma Node solo para `web/`. El CI gana un trabajo: tipos al día, lint, pruebas y build.

## 12. Pruebas y evidencias

- **Unitarias** (Vitest y Testing Library): las reglas de presentación (orden por gravedad, frases, colores
  por banda) y los estados de §5.9.
- **De punta a punta** (Playwright contra el Compose): ingresar, ver el panel, correr una simulación que
  termina en alerta, lanzar un entrenamiento como admin y verlo avanzar, y que un `usuario` no entra a
  clúster. Corren en local; **[abierta]** si entran al CI.
- **Evaluación experimental** del Entregable 3: las series de §7.3 con su resultado esperado, las capturas
  reales de cada pantalla y, para el tiempo real, cuánto tarda una lectura en verse en el panel.
- **Capturas reales** para el informe y el video, tomadas contra el Compose local.

## 13. Reparto y calendario **[abierta]**

Rody no trabaja los domingos, así que la entrega se cierra el **sábado 28**.

| Día | Hito |
|---|---|
| sáb 21 | Entrega de la PC4. Esta spec en revisión |
| lun 23 | Spec cerrada; contrato con las rutas de §6; andamiaje de `web/`, tokens, ingreso y servicio `web` en Compose |
| mar 24 | Panel con el mapa y estación; `GET /api/regimenes` y la serie |
| mié 25 | Simulación y entrenamiento en vivo; evento `estacion.regimen` |
| jue 26 | Clúster e impacto (`reports/impacto.json`, población) |
| vie 27 | Pruebas de punta a punta, capturas, informe del TB2 |
| sáb 28 | Video de 6 minutos, release y entrega |

Tareas que se pueden repartir sin frenar el resto, cada una con su PR y su criterio de aceptación:

- **Población por distrito**: `reports/poblacion_distritos.csv` con las 16 filas, la fuente y el año; se
  acepta si cada cifra se puede abrir en la página del INEI.
- **Textos de impacto y de «¿de dónde sale este número?»**: revisados contra el informe; se acepta si no
  afirman nada de §7.2.
- **Capturas y guion del video**: una captura por pantalla y el guion de 6 minutos con la parte de cada
  integrante (el enunciado pide que participen todos).

## 14. Decisiones abiertas

1. Fuente y año de la población (§7.1).
2. Orden de recorte de las rutas nuevas de la API si falta tiempo (§6).
3. Token en memoria o en `sessionStorage` (§10).
4. Si las pruebas de punta a punta entran al CI (§12).
5. Reparto de tareas (§13).
6. Si el experimento del mapa nacional (PR #52) aparece en «Acerca de» como trabajo aparte. Con datos
   sintéticos no entra a «Impacto».

## 15. Fuera de alcance

- Pronóstico de la calidad del aire: el sistema clasifica y avisa sobre lo medido, no predice el futuro.
- Alta de usuarios, cambio de clave y administración de cuentas.
- Aplicación móvil nativa; la SPA se adapta al teléfono, pero se diseña para escritorio y proyector.
- Editar el modelo o las estaciones desde la UI.
