# web/ — la UI de aire-distribuido

La SPA del TB2: React 19, Vite, TypeScript estricto, Tailwind 4, React Router, TanStack Query y D3.
El diseño completo está en [`docs/specs/2026-11-21-frontend-tb2-design.md`](../docs/specs/2026-11-21-frontend-tb2-design.md);
cómo levantarla, en el [README del repo](../README.md) (paso 9).

El gestor de paquetes es **pnpm** (la versión la fija `packageManager` en `package.json`; con Node 22,
`corepack enable` la deja lista). No se usa `npm` ni `yarn`: el único lockfile es `pnpm-lock.yaml`, y un
`npm install` instalaría otras versiones y dejaría un `package-lock.json` que no va al repo.

```bash
pnpm install
pnpm dev      # http://localhost:5173, con proxy a la API de http://localhost:8080 (otra: API_URL=…)
pnpm test     # Vitest y Testing Library
pnpm lint
pnpm build    # tsc -b y vite build, a dist/
pnpm tipos    # regenera src/api/contrato.d.ts desde ../docs/api/openapi.yaml
```

## Dónde está cada cosa

```text
src/
  api/          cliente tipado desde el contrato, errores, WebSocket y la caché en vivo; los archivos de
                /datos/ (datos.ts), las consultas de estaciones (estaciones.ts) y lo visto en la sesión
                (historia.ts y vistas.ts)
  sesion/       token y rol en sessionStorage, guardas de ruta y los dos ingresos (con clave y de invitado)
  armazon/      barra superior con sus indicadores, pie, menú, modo proyector, aviso de vencimiento, título
  componentes/  Encabezado, Tarjeta, Cifra, Dato, Frase, Banda, Etiqueta, Pildoras, Tabla, Desplegable, Estado,
                Aviso y Advertencia, Barrera; los tonos (tonos.ts) y los formatos (formato.ts)
  graficos/     los gráficos de D3 y lo que comparten (su README.md los nombra)
  pantallas/    una por archivo (Ingreso, Panel, Estacion, Simulacion, Entrenamiento, Cluster, Impacto) y, al
                lado, una carpeta con lo propio de cada una (panel/, estacion/, simulacion/…)
  estilos/      tokens de diseño, fuentes y modo proyector
  pruebas/      preparación de Vitest y dobles (fetch y WebSocket falsos, y el <dialog> modal que jsdom no trae)
pruebas/        lo que se prueba fuera del navegador: el Caddyfile y el contraste de los tokens
public/datos/   impacto.json, series.json y peru.geojson (los generan scripts/; ver su README.md)
Caddyfile       el servidor del Compose: la UI, el proxy a la API y las cabeceras de seguridad
```

## Construir una pantalla

Cada pantalla tiene su ruta, su guarda y su lugar en el menú (`src/armazon/menu.ts` y `src/App.tsx`), su archivo
en `src/pantallas/` y una carpeta al lado con lo que es solo suyo. Lo que usan dos o más pantallas va a
`src/api/` (datos), a `src/componentes/` (lo que se ve y cómo se escribe) o a `src/graficos/`. Quedan dos
excepciones, a sabiendas: `panel/situacion.ts` (cómo se presenta un estado: también lo usa la Estación) y
`simulacion/motor.ts` (el simulador: también lo lee la barra). Con la sesión iniciada,
<http://localhost:5173/muestrario> muestra los componentes base.

Toda pantalla empieza con `<Encabezado titulo="…" bajada="…">` (lo que va a la derecha del título son sus hijos)
y separa sus tarjetas con `space-y-5` y `gap-5`.

### Pedir datos

`api` conoce cada ruta del contrato y `pedir()` devuelve los datos o lanza un `ErrorApi`:

```tsx
import { useQuery } from '@tanstack/react-query'
import { api, claves, pedir } from '../api/cliente'
import { Estado, EstadoDeError } from '../componentes/indice'

const estaciones = useQuery({
  queryKey: claves.estaciones,
  queryFn: ({ signal }) => pedir(api.GET('/api/estaciones', { signal })),
})
if (estaciones.isPending) return <Estado tipo="cargando" />
if (estaciones.isError) return <EstadoDeError error={estaciones.error} alReintentar={() => void estaciones.refetch()} />
```

Los estados de toda pantalla (cargando, vacío, error con reintento y sin permiso) son `<Estado>` y
`<EstadoDeError>`; dentro de una tarjeta llevan `compacto`. El rechazo de un formulario o un error que no reemplaza
a la pantalla es un `<Aviso tono="alerta">`, y la línea de cautela junto a un control, una `<Advertencia>`. Qué se
dice de cada error se decide en un solo lugar, `src/api/errores.ts`: las pantallas no miran códigos HTTP.

Las claves de la caché están todas en `claves` (`api/cliente.ts`). Lo que piden varias pantallas ya tiene su hook,
y por compartir la clave se pide una sola vez:

| Qué | Hook | Dónde |
|---|---|---|
| El catálogo de estaciones | `useEstaciones()` | `api/estaciones.ts` |
| El último estado de todas, o de una (en vivo) | `useEstados()`, `useEstado(id)` | `api/estaciones.ts` |
| `/datos/impacto.json` | `useImpacto()` | `api/datos.ts` |
| `/datos/series.json` | `useSeries()` | `api/datos.ts` |
| `/datos/peru.geojson` | `useMapaDelPeru()` | `api/datos.ts` |

`api/datos.ts` tiene también la forma de cada archivo (`Impacto`, `Series`, `MapaDelPeru`) y `pedirDatos()`, que
lanza un `ErrorApi` como `pedir()`. Un archivo que no tiene la forma esperada es un error, no una pantalla a medias.

Si la API deja de aceptar el token, la sesión se cierra y la UI vuelve al ingreso sin que la pantalla haga nada:
con cualquier pedido que responda 401, al llegar la hora de `expira`, cuando el WebSocket se cierra con 1008 por
el token, y cuando su apretón de manos falla tres veces seguidas y un pedido de comprobación responde 401.

Si una pantalla lanza al dibujarse, la barrera del armazón (`<Barrera>`) lo muestra en su lugar con «Reintentar»,
y la barra sigue a la vista. No hace falta poner otra.

### El último estado de las estaciones, en vivo

Hay un solo WebSocket por sesión y **un solo camino** del evento `estacion.regimen` a la caché: el armazón está
suscrito y actualiza, con cada evento, la lista (`claves.regimenes`) y la entrada de su estación
(`claves.regimen(id)`). Una pantalla **no** se suscribe a `estacion.regimen` para eso: lee la caché, con su
consulta marcada `EN_VIVO` y pedida con el ayudante de `src/api/enVivo.ts`.

```tsx
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, claves, pedir } from '../api/cliente'
import { EN_VIVO, pedirRegimen, pedirRegimenes } from '../api/enVivo'

const consultas = useQueryClient()

// El Panel: todas las estaciones con estado.
const estados = useQuery({
  queryKey: claves.regimenes,
  meta: EN_VIVO,
  queryFn: ({ signal }) => pedirRegimenes(consultas, () => pedir(api.GET('/api/regimenes', { signal }))),
})

// La Estación: una sola.
const estado = useQuery({
  queryKey: claves.regimen(id),
  meta: EN_VIVO,
  queryFn: ({ signal }) =>
    pedirRegimen(consultas, id, () => pedir(api.GET('/api/estaciones/{id}/regimen', { params: { path: { id } }, signal }))),
})
```

Eso es lo que hacen `useEstados()` y `useEstado(id)` de `api/estaciones.ts`: una pantalla usa esos hooks y no
repite la consulta. Con eso la pantalla se mantiene al día sola. El GET y el WebSocket viajan por conexiones distintas, así que la
respuesta de un GET puede llegar **después** de un evento más nuevo de la misma estación. La regla, que es la del
contrato y que el ayudante aplica:

> De una estación de la que ya llegó un evento desde que se abrió (o se reabrió) el WebSocket, la entrada del GET
> se ignora: vale el evento.

Si el GET traía algo más nuevo, su evento viene detrás por el WebSocket, que entrega en orden. No se compara por
`ts` (va al segundo) ni por `fecha_hora` (una corrección de la misma hora la conserva): entre dos eventos manda el
que llegó último. De ahí sale lo demás:

- **Un evento con el GET en viaje**, o anterior al GET: `pedirRegimenes` y `pedirRegimen` conservan el evento.
- **Un corte**: en cuanto la conexión deja de estar abierta se olvida de qué estaciones había eventos (sin
  WebSocket se pierden otros): mientras no vuelva, manda lo que responda un GET, y lo que la caché sabe de las
  estaciones queda marcado para volver a pedirse al abrir una pantalla. Al abrirse o reabrirse el WebSocket se
  vuelven a pedir todas las consultas marcadas `EN_VIVO` y todo el estado de las estaciones.
- **El orden al entrar**: no hay que abrir el WebSocket antes de pedir. La primera conexión también refresca.

Los tipos de `GET /api/regimenes` y de `EstadoEstacion` salen del contrato: si el contrato cambia, `pnpm tipos`.

### Lo visto en la sesión

La API no guarda una serie histórica: la curva de la Estación se arma con lo que esta pestaña vio. Son dos
registros, los dos fuera de React:

- **`api/historia.ts`**, los estados recibidos. Mira la caché en vivo y anota cada estado que entra, por un GET
  o por un evento. Lo enciende el armazón (`useRegistroDeSesion()` en `armazon/Armazon.tsx`), así que anota desde
  cualquier pantalla. Se guarda en `sessionStorage`: sobrevive a un F5. Las escrituras se agrupan (a lo sumo una
  cada medio segundo, y al ocultarse la página). Se lee con `useHistoria(estacion)`.
- **`api/vistas.ts`**, las lecturas enviadas. El evento `estacion.regimen` no trae la lectura horaria: solo se
  conocen las que esta sesión mandó desde Simulación, y aquí quedan con lo que la API respondió a cada una
  (`anotarVista()`). Vive en memoria: se pierde al recargar. Se lee con `useLecturasVistas(estacion)`.

El simulador (`pantallas/simulacion/motor.ts`) también vive fuera de React: una corrida sigue aunque se cambie de
pantalla, y mientras no termina la barra lo dice con `<SimulacionEnCurso>` (`armazon/Indicadores.tsx`). Vive en la
pestaña: mientras está en curso, el navegador pregunta antes de recargarla o cerrarla (`beforeunload`).

Lo mismo el entrenamiento a la vista (`pantallas/entrenamiento/almacen.ts`): el que se lanzó o se abrió del
historial sigue ahí al volver a la pantalla, y sus rondas le llegan aunque no esté montada (`App.tsx`).

**Todo lo que una sesión deja en la pestaña se limpia en un solo lugar**, `limpiarLoDeLaSesion()` en `App.tsx`,
cuando el token deja de estar («Salir», vencimiento o 401): la caché de consultas, los eventos de la caché en
vivo, los dos registros de arriba, el entrenamiento a la vista y el simulador, que se detiene. Lo que una pantalla
guarde fuera de sus componentes se suma ahí; no se suscribe por su cuenta al fin de la sesión. La limpieza
escucha al almacén de la sesión, no al desmontaje de un componente: con StrictMode (`pnpm dev`) React desmonta y
vuelve a montar todo al cargar, y eso no es que la sesión haya terminado.

**La respuesta de un pedido puede llegar cuando su sesión ya terminó.** Quien escriba en uno de esos almacenes
con la respuesta de un pedido toma la marca de la sesión antes de pedir y la comprueba al responder
(`marcarSesion()` en `sesion/epoca.ts`); `limpiarLoDeLaSesion()` la avanza. Si no, el siguiente que entre en la
pestaña hereda lo que respondió tarde.

### Otros eventos

Para lo que no es el estado de las estaciones (las rondas de un entrenamiento), la pantalla se suscribe por nombre:

```tsx
import { useEvento } from '../api/tiempoReal'

useEvento('entrenamiento.ronda', (ronda) => { /* … */ })
useEvento('entrenamiento.fin', (fin) => { /* … */ })
```

Los eventos y sus datos están en `src/api/eventos.ts`; un evento nuevo se agrega ahí. `useEstadoWS()` da el
estado de la conexión. Los eventos que se emiten con el WebSocket caído se pierden: una pantalla que dependa de
ellos marca su consulta `EN_VIVO` para volver a pedirla al reconectar.

### Roles, título y foco

- `useSesion()` da la sesión (`sesion.rol`); `puede(rol, SOLO_ADMIN)` decide si se muestra un control. La UI
  esconde, pero quien autoriza es la API.
- No hay registro. Se entra con usuario y clave (`ingresar`) o, desde «No pienso crearme una cuenta aquí», como
  invitado (`entrarComoInvitado`, contra `POST /api/auth/invitado`): los dos abren la sesión por el mismo camino,
  en `sesion/useSesion.ts`. Las credenciales del invitado no están en la UI: la API responde el token de la
  cuenta que nombra `API_INVITADO`, y si no hay ninguna (404) el diálogo lo dice. A quien entra así la barra lo
  llama «invitado»: la API no dice el usuario de la cuenta.
- El armazón pone el título del documento de cada ruta («Estación · Aire distribuido») y, al navegar, lleva el
  foco al contenido. Una pantalla que quiera un título más preciso usa `useTitulo('CA-AYAS-01 · Estación')`
  (`src/armazon/titulo.ts`).

## Reglas

- **El color nunca va solo**: cada estado lleva su palabra o su dibujo (`<Banda>`, `<Etiqueta>`, `<Estado>`).
  Los tonos están en `componentes/tonos.ts`: `aviso` es para lo que conviene saber antes de seguir, y `moderada`,
  que tiene su mismo color, es solo la banda INCA.
- **Nada en línea ni de terceros.** La CSP de `Caddyfile` solo admite scripts, estilos y fuentes del propio
  origen: D3 con `.style()` y clases, no con `.attr('style', …)`; sin CDN; sin `dangerouslySetInnerHTML`. Vite en
  desarrollo no aplica la CSP: lo que la rompe se ve recién en el Compose.
- **Los colores salen de los tokens** (`estilos/tokens.css`, `var(--color-…)`), y todo mide en `rem`: así el modo
  proyector cambia el contraste y el tamaño solo.
- **Las fechas en hora de Lima** y los números con `numero()`. Todos los formatos están en
  `componentes/formato.ts`, una sola vez: `fechaHora`, `hora`, `horaConSegundos`, `fechaConSegundos`, `diaCorto` y
  `diaDeInstante` para un instante; `fechaDia` y `diasEnLlano` para un día sin hora (`2021-12-14`); `lapso` y
  `duracion`; `numero`, `milisegundos`, `compacto`, `porciento` y `proporcion`; `enumerar` y `nombrePropio`. Si
  falta uno, se agrega ahí con su prueba (`formato.test.ts`), no en la carpeta de la pantalla.
- **Un valor junto a su umbral se escribe de modo que se vea de qué lado cae**: `decimalesFrenteA(valor, umbral)`
  da los decimales que hacen falta. 50.03 no se escribe «50.0» al lado de «supera el ECA (50)».
- **Las mismas palabras para lo mismo**: «último estado recibido» (no hay un «ahora»), «media de 24 h» en un
  rótulo y «media de 24 horas» en una frase, «en alerta» y «sin alerta», «ECA de 24 h». El «excede el ECA» de un
  régimen se dice del grupo de lecturas, nunca de la estación.
- **El token solo vive en `sessionStorage`** (`sesion/almacen.ts`). No se copia a otro lado ni va en una URL.
- **Una imagen se importa desde el código** (`import gato from './gato.webp'`), con su ancho y su alto en el
  `<img>`: Vite la emite en `/assets/` con hash y la CSP solo admite imágenes del propio origen.
- **Un diálogo modal es un `<dialog>` con `showModal()`** (`pantallas/ingreso/DialogoInvitado.tsx`): el navegador
  deja inerte lo de atrás y lo cierra con Escape. Lo que no trae se pone a mano: a dónde va el foco al abrir, que
  Tab dé la vuelta dentro y cerrar con un clic fuera.
- **`public/datos/` se genera**: `impacto.json` y `series.json` con `scripts/impacto.py`, y `peru.geojson` con
  `scripts/mapa_peru.py` (`public/datos/README.md`). No se editan a mano. Se piden desde `/datos/…`; uno que no
  existe responde 404, no la página.
- **El texto da 4.5:1 de contraste o más** en los dos modos, y el borde de un campo, 3:1
  (`pruebas/contraste.test.ts` lo comprueba sobre los tokens). El botón principal usa `bg-boton text-boton-texto`,
  no el acento con blanco.
- **Sin `changeOrigin` en el proxy de Vite**: la API compara el origen y el WebSocket respondería 403.

## Limitaciones conocidas

- **El uso de CPU justo después de reiniciar la API.** `/api/metricas` publica el CPU acumulado del proceso y el
  uso sale de dos consultas seguidas (`pantallas/cluster/cpu.ts`). Si la API se reinicia entre dos consultas y el
  proceso nuevo ya lleva más tiempo y más CPU que la muestra anterior, las dos muestras parecen del mismo proceso
  y ese punto de la curva sale mal. No hay con qué distinguirlo: las métricas no traen un identificador de
  arranque. Se corrige solo con la consulta siguiente.
- **«Sistema limpio» no mira la ventana de 24 horas.** Antes de simular, Simulación dice que el sistema está
  limpio para un tramo si ninguna de sus estaciones tiene un estado de esas horas o posterior
  (`pantallas/simulacion/limpieza.ts`). Un estado anterior al tramo por menos de 24 horas no impide simular, pero
  sus lecturas siguen en la ventana de la API: las primeras medias del tramo las incluyen y pueden no coincidir
  con lo esperado del archivo.
- **Las lecturas horarias de la curva de la Estación no sobreviven a un F5** (`api/vistas.ts` vive en memoria);
  los estados recibidos, sí.
- **El formulario de Entrenamiento vuelve a sus valores por omisión** al salir de la pantalla y volver; el
  entrenamiento a la vista, no.
