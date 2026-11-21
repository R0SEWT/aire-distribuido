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
  api/          cliente tipado desde el contrato, errores, WebSocket y la caché en vivo
  sesion/       token y rol en sessionStorage, guardas de ruta
  armazon/      barra superior, pie, menú, modo proyector, aviso de vencimiento, título
  componentes/  Tarjeta, Cifra, Frase, Banda, Etiqueta, Pildoras, Tabla, Desplegable, Estado, Barrera
  graficos/     los gráficos de D3 (los escribe cada pantalla)
  pantallas/    una por archivo: Ingreso, Panel, Estacion, Simulacion, Entrenamiento, Cluster, Impacto
  estilos/      tokens de diseño, fuentes y modo proyector
  pruebas/      preparación de Vitest y dobles (fetch y WebSocket falsos)
pruebas/        lo que se prueba fuera del navegador: el Caddyfile y el contraste de los tokens
public/datos/   impacto.json, series.json y peru.geojson (los generan scripts/; ver su README.md)
Caddyfile       el servidor del Compose: la UI, el proxy a la API y las cabeceras de seguridad
```

## Construir una pantalla

Cada pantalla ya tiene su ruta, su guarda y su lugar en el menú (`src/armazon/menu.ts` y `src/App.tsx`). Se
construye en su archivo de `src/pantallas/`, reemplazando el marcador `<EnConstruccion>`; el armazón no se toca.
Con la sesión iniciada, <http://localhost:5173/muestrario> muestra los componentes base.

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
`<EstadoDeError>`. Qué se dice de cada error se decide en un solo lugar, `src/api/errores.ts`: las pantallas no
miran códigos HTTP.

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

Con eso la pantalla se mantiene al día sola. El GET y el WebSocket viajan por conexiones distintas, así que la
respuesta de un GET puede llegar **después** de un evento más nuevo de la misma estación. La regla, que es la del
contrato y que el ayudante aplica:

> De una estación de la que ya llegó un evento desde que se abrió (o se reabrió) el WebSocket, la entrada del GET
> se ignora: vale el evento.

Si el GET traía algo más nuevo, su evento viene detrás por el WebSocket, que entrega en orden. No se compara por
`ts` (va al segundo) ni por `fecha_hora` (una corrección de la misma hora la conserva): entre dos eventos manda el
que llegó último. De ahí sale lo demás:

- **Un evento con el GET en viaje**, o anterior al GET: `pedirRegimenes` y `pedirRegimen` conservan el evento.
- **Un corte**: en el instante en que el WebSocket se abre o se reabre se olvida de qué estaciones había eventos
  (mientras estuvo caído se perdieron otros) y se vuelven a pedir todas las consultas marcadas `EN_VIVO`.
- **El orden al entrar**: no hay que abrir el WebSocket antes de pedir. La primera conexión también refresca.

Los tipos de `GET /api/regimenes` y de `EstadoEstacion` salen del contrato: si el contrato cambia, `pnpm tipos`.

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
- El armazón pone el título del documento de cada ruta («Estación · Aire distribuido») y, al navegar, lleva el
  foco al contenido. Una pantalla que quiera un título más preciso usa `useTitulo('CA-AYAS-01 · Estación')`
  (`src/armazon/titulo.ts`).

## Reglas

- **El color nunca va solo**: cada estado lleva su palabra o su dibujo (`<Banda>`, `<Etiqueta>`, `<Estado>`).
- **Nada en línea ni de terceros.** La CSP de `Caddyfile` solo admite scripts, estilos y fuentes del propio
  origen: D3 con `.style()` y clases, no con `.attr('style', …)`; sin CDN; sin `dangerouslySetInnerHTML`. Vite en
  desarrollo no aplica la CSP: lo que la rompe se ve recién en el Compose.
- **Los colores salen de los tokens** (`estilos/tokens.css`, `var(--color-…)`), y todo mide en `rem`: así el modo
  proyector cambia el contraste y el tamaño solo.
- **Las fechas en hora de Lima** y los números con `numero()` (`componentes/formato.ts`).
- **El token solo vive en `sessionStorage`** (`sesion/almacen.ts`). No se copia a otro lado ni va en una URL.
- **`public/datos/` se genera**: `impacto.json` y `series.json` con `scripts/impacto.py`, y `peru.geojson` con
  `scripts/mapa_peru.py` (`public/datos/README.md`). No se editan a mano. Se piden desde `/datos/…`; uno que no
  existe responde 404, no la página.
- **El texto da 4.5:1 de contraste o más** en los dos modos, y el borde de un campo, 3:1
  (`pruebas/contraste.test.ts` lo comprueba sobre los tokens). El botón principal usa `bg-boton text-boton-texto`,
  no el acento con blanco.
- **Sin `changeOrigin` en el proxy de Vite**: la API compara el origen y el WebSocket respondería 403.
