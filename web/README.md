# web/ — la UI de aire-distribuido

La SPA del TB2: React 19, Vite, TypeScript estricto, Tailwind 4, React Router, TanStack Query y D3, con pnpm.
El diseño completo está en [`docs/specs/2026-11-21-frontend-tb2-design.md`](../docs/specs/2026-11-21-frontend-tb2-design.md);
cómo levantarla, en el [README del repo](../README.md) (paso 9).

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
  api/          cliente tipado desde el contrato, errores, WebSocket y sus hooks
  sesion/       token y rol en sessionStorage, guardas de ruta
  armazon/      barra superior, pie, menú, modo proyector, aviso de vencimiento
  componentes/  Tarjeta, Cifra, Frase, Banda, Etiqueta, Pildoras, Tabla, Desplegable, Estado
  graficos/     los gráficos de D3 (los escribe cada pantalla)
  pantallas/    una por archivo: Ingreso, Panel, Estacion, Simulacion, Entrenamiento, Cluster, Impacto
  estilos/      tokens de diseño, fuentes y modo proyector
  pruebas/      preparación de Vitest y dobles (fetch y WebSocket falsos)
public/datos/   peru.geojson, series.json, impacto.json
Caddyfile       el servidor del Compose: la UI, el proxy a la API y las cabeceras de seguridad
```

## Construir una pantalla

Cada pantalla ya tiene su ruta, su guarda y su lugar en el menú (`src/armazon/menu.ts` y `src/App.tsx`). Se
construye en su archivo de `src/pantallas/`, reemplazando el marcador `<EnConstruccion>`; el armazón no se toca.
Con la sesión iniciada, <http://localhost:5173/muestrario> muestra los componentes base.

**Pedir datos.** `api` conoce cada ruta del contrato y `pedir()` devuelve los datos o lanza un `ErrorApi`:

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
miran códigos HTTP. Un 401 cierra la sesión y vuelve al ingreso sin que la pantalla haga nada.

**Tiempo real.** Hay un solo WebSocket por sesión. Una pantalla se suscribe a un evento por su nombre y, lo
usual, actualiza la caché de TanStack Query:

```tsx
import { useQueryClient } from '@tanstack/react-query'
import { useEvento } from '../api/tiempoReal'

const consultas = useQueryClient()
useEvento('estacion.regimen', (estado) => {
  consultas.setQueryData(claves.regimen(estado.estacion), estado)
})
```

Los eventos y sus datos están en `src/api/eventos.ts`; un evento nuevo se agrega ahí.

**Roles.** `useSesion()` da la sesión (`sesion.rol`); `puede(rol, SOLO_ADMIN)` decide si se muestra un control.
La UI esconde, pero quien autoriza es la API.

## Reglas

- **El color nunca va solo**: cada estado lleva su palabra o su dibujo (`<Banda>`, `<Etiqueta>`, `<Estado>`).
- **Nada en línea ni de terceros.** La CSP de `Caddyfile` solo admite scripts, estilos y fuentes del propio
  origen: D3 con `.style()` y clases, no con `.attr('style', …)`; sin CDN; sin `dangerouslySetInnerHTML`. Vite en
  desarrollo no aplica la CSP: lo que la rompe se ve recién en el Compose.
- **Los colores salen de los tokens** (`estilos/tokens.css`, `var(--color-…)`), y todo mide en `rem`: así el modo
  proyector cambia el contraste y el tamaño solo.
- **Las fechas en hora de Lima** y los números con `numero()` (`componentes/formato.ts`).
- **El token solo vive en `sessionStorage`** (`sesion/almacen.ts`). No se copia a otro lado ni va en una URL.
- **`public/datos/` se genera** (`scripts/impacto.py`): no se edita a mano.
- **Sin `changeOrigin` en el proxy de Vite**: la API compara el origen y el WebSocket respondería 403.
