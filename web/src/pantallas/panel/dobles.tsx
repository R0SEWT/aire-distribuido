// Dobles para las pruebas del Panel y de la Estación: un catálogo chico con
// los tres estados, los archivos de /datos/ y la app montada con sus rutas.

import { render } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { crearClienteDeConsultas } from '../../api/cliente'
import type { Impacto, MapaDelPeru } from '../../api/datos'
import type { EstadoEstacion } from '../../api/enVivo'
import type { Estacion as FichaEstacion } from '../../api/estaciones'
import { Proveedores } from '../../App'
import { fabricaDeEnchufes, SALUD_OK, simularApi } from '../../pruebas/dobles'
import { abrirSesion, type Rol } from '../../sesion/almacen'
import Estacion from '../Estacion'
import Panel from '../Panel'

const ficha = (id: string, distrito: string | null, tipo_actividad: string, lat: number, lon: number): FichaEstacion => ({
  id,
  objeto_fiscalizado: `Objeto de ${id}`,
  tipo_actividad,
  departamento: distrito ? 'LIMA' : null,
  provincia: distrito ? 'LIMA' : null,
  distrito,
  lat,
  lon,
})

/** Seis estaciones: cinco que miden PM por hora y una que no. */
export const ESTACIONES: FichaEstacion[] = [
  ficha('CA-01', 'CHIMBOTE', 'zona industrial', -9.12, -78.55),
  ficha('CA-AYAS-01', 'SAN MARCOS', 'minería', -9.52, -77.02),
  ficha('CA-CG-02', 'CASA GRANDE', 'agroindustria', -7.74, -79.18),
  ficha('CA-HU-04', 'LURIGANCHO', 'ladrilleras', -11.99, -76.92),
  ficha('CA-HUAR-01', null, 'minería', -10.79, -76.29),
  ficha('CA-MAC-01', 'LA HUACA', 'no lo dice el nombre', -4.91, -80.85),
]

/** Lo que el Panel y la Estación leen de impacto.json; del resto, lo justo para que tenga su forma. */
export const IMPACTO = {
  metadatos: { reglas: {} },
  totales: {},
  distritos: [],
  senales: {},
  anticipacion: {},
  alcance: { estaciones_con_pm_horario: 5, estaciones_sin_pm_horario: ['CA-01'] },
  estaciones: [{ estacion: 'CA-HU-04', primera_fecha: '2021-12-10', ultima_fecha: '2021-12-20', dias_con_datos: 11 }],
  poblacion_distritos: [
    {
      ubigeo: '150118',
      distrito: 'LURIGANCHO',
      poblacion: 240814,
      anio: 2017,
      fuente: 'https://www.inei.gob.pe/ejemplo.pdf',
      estaciones: ['CA-HU-04'],
    },
  ],
} as unknown as Impacto

/** Un solo polígono, con el anillo en sentido horario como los de peru.geojson. */
export const MAPA: MapaDelPeru = {
  type: 'FeatureCollection',
  features: [
    {
      type: 'Feature',
      properties: { departamento: 'PERU', iso: 'PE' },
      geometry: {
        type: 'Polygon',
        coordinates: [
          [
            [-81, -18],
            [-81, 0],
            [-69, 0],
            [-69, -18],
            [-81, -18],
          ],
        ],
      },
    },
  ],
}

interface Opciones {
  /** La hora del 15 de diciembre de 2021 a la que corresponde el estado. */
  hora?: number
  pm2_5?: number | null
  pm10?: number | null
  lecturas?: number
  alerta?: boolean
  inca?: EstadoEstacion['inca']
  regimen?: number
  excede_eca?: boolean
}

/** El estado de una estación como lo manda la API. Sin opciones: buena, sin alerta, con 24 lecturas. */
export function estadoDe(estacion: string, o: Opciones = {}): EstadoEstacion {
  const hora = String(o.hora ?? 10).padStart(2, '0')
  return {
    estacion,
    fecha_hora: `2021-12-15T${hora}:00:00-05:00`,
    regimen: o.regimen ?? 0,
    excede_eca: o.excede_eca ?? false,
    media_24h: { pm2_5: o.pm2_5 === undefined ? 8 : o.pm2_5, pm10: o.pm10 === undefined ? 20 : o.pm10, lecturas: o.lecturas ?? 24 },
    alerta: o.alerta ?? false,
    inca: o.inca === undefined ? 'buena' : o.inca,
    ts: `2026-11-21T17:${hora}:00-05:00`,
  }
}

export const EN_ALERTA = { pm2_5: 88.8, pm10: 120.7, alerta: true, inca: 'mala', regimen: 2 } as const

type Rutas = Parameters<typeof simularApi>[0]

/** La API con el catálogo y los archivos de /datos/; `mas` suma o reemplaza rutas. */
export function simularPanel(estados: EstadoEstacion[] | (() => EstadoEstacion[]), mas: Rutas = {}) {
  return simularApi({
    'GET /api/salud': SALUD_OK,
    'GET /api/estaciones': { estado: 200, cuerpo: ESTACIONES },
    'GET /api/regimenes': () => ({ estado: 200, cuerpo: typeof estados === 'function' ? estados() : estados }),
    'GET /datos/impacto.json': { estado: 200, cuerpo: IMPACTO },
    'GET /datos/peru.geojson': { estado: 200, cuerpo: MAPA },
    ...mas,
  })
}

/** Monta el Panel y la Estación con sus rutas, la caché y el WebSocket falso, con la sesión de `rol`. */
export function montar(ruta: string, rol: Rol = 'usuario') {
  abrirSesion({ usuario: rol === 'admin' ? 'ana' : 'demo', token: `token-de-${rol}`, rol, expira: Date.now() + 3_600_000 })
  const enchufes = fabricaDeEnchufes()
  const consultas = crearClienteDeConsultas()
  const vista = render(
    <Proveedores consultas={consultas} tiempoReal={{ crearEnchufe: enchufes.crearEnchufe }}>
      <MemoryRouter initialEntries={[ruta]}>
        <Routes>
          <Route path="/" element={<Panel />} />
          <Route path="/estacion" element={<Estacion />} />
          <Route path="/estacion/:id" element={<Estacion />} />
          <Route path="/simulacion" element={<h1>Simulación de prueba</h1>} />
        </Routes>
      </MemoryRouter>
    </Proveedores>,
  )
  return { ...vista, enchufes, consultas }
}
