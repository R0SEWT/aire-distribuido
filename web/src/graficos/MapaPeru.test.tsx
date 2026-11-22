// El mapa con el peru.geojson de verdad: que el país salga derecho y entero, y
// que las marcas no se pisen.

import { render, screen } from '@testing-library/react'
import { geoArea, type ExtendedFeatureCollection } from 'd3'
import { MemoryRouter } from 'react-router'
import { describe, expect, it } from 'vitest'
import crudo from '../../public/datos/peru.geojson?raw'
import { MapaPeru, type LugarEnMapa, type MarcaEnMapa } from './MapaPeru'

const PERU = JSON.parse(crudo) as ExtendedFeatureCollection

// Coordenadas de la ficha (reports/estaciones_ficha.csv).
const LUGARES: LugarEnMapa[] = [
  { id: 'CA-TA-01', lon: -81.27732, lat: -4.5758, apagada: true }, // Talara: el noroeste
  { id: 'CA-TAL-01', lon: -70.68192, lat: -17.09943, apagada: false }, // Torata: el sureste
  { id: 'CA-CH-03', lon: -78.56185, lat: -9.1033, apagada: false },
  { id: 'CA-03', lon: -78.56185, lat: -9.1033, apagada: true }, // el mismo punto que CA-CH-03
  { id: 'CA-HU-04', lon: -76.9225, lat: -11.9961, apagada: false },
  { id: 'CA-HU-09', lon: -76.9086, lat: -11.9883, apagada: false },
]

const MARCAS = new Map<string, MarcaEnMapa>(
  LUGARES.map((l) => [l.id, { banda: l.apagada ? 'no_mide' : 'mala', alerta: l.id === 'CA-HU-04', nombre: `${l.id}: de prueba` }]),
)

function montar() {
  render(
    <MemoryRouter>
      <MapaPeru geo={PERU} lugares={LUGARES} marcas={MARCAS} enlaceDe={(id) => `/estacion/${id}`} titulo="Mapa de prueba" />
    </MemoryRouter>,
  )
  const lugarDe = (id: string) => {
    const g = document.querySelector(`[data-estacion="${id}"] > g`)!
    const [, x, y] = /translate\(([\d.-]+) ([\d.-]+)\)/.exec(g.getAttribute('transform') ?? '')!
    return { x: Number(x), y: Number(y) }
  }
  return { lugarDe }
}

describe('el mapa del Perú', () => {
  it('peru.geojson trae los anillos en el sentido que espera d3-geo: ningún polígono es «todo menos él»', () => {
    expect(PERU.features).toHaveLength(26)
    // Un anillo al revés encierra el resto del planeta: casi 4π estereorradianes.
    for (const f of PERU.features) expect(geoArea(f)).toBeLessThan(0.02)
  })

  it('dibuja los 26 polígonos dentro del lienzo', () => {
    montar()

    const trazos = [...document.querySelectorAll('[data-capa="departamentos"] path')]
    expect(trazos).toHaveLength(26)
    for (const t of trazos) {
      const numeros = (t.getAttribute('d') ?? '').match(/-?\d+(\.\d+)?/g)!.map(Number)
      expect(Math.min(...numeros)).toBeGreaterThanOrEqual(0)
      expect(Math.max(...numeros)).toBeLessThanOrEqual(560)
    }
  })

  it('el norte queda arriba y el oeste a la izquierda', () => {
    const { lugarDe } = montar()

    const talara = lugarDe('CA-TA-01')
    const torata = lugarDe('CA-TAL-01')
    expect(talara.y).toBeLessThan(torata.y)
    expect(talara.x).toBeLessThan(torata.x)
  })

  it('las marcas de estaciones vecinas, o en el mismo punto, no se pisan', () => {
    const { lugarDe } = montar()

    const distancia = (a: string, b: string) => Math.hypot(lugarDe(a).x - lugarDe(b).x, lugarDe(a).y - lugarDe(b).y)
    expect(distancia('CA-CH-03', 'CA-03')).toBeGreaterThan(15)
    expect(distancia('CA-HU-04', 'CA-HU-09')).toBeGreaterThan(20)
  })

  it('cada marca es un enlace a su estación, con su nombre, y la que está en alerta lleva anillo y su código escrito', () => {
    montar()

    const enlace = screen.getByRole('link', { name: 'CA-HU-04: de prueba' })
    expect(enlace).toHaveAttribute('href', '/estacion/CA-HU-04')
    expect(enlace.querySelector('[data-anillo]')).not.toBeNull()
    expect(screen.getByRole('link', { name: 'CA-HU-09: de prueba' }).querySelector('[data-anillo]')).toBeNull()
    expect([...document.querySelectorAll('[data-capa="rotulos"] text')].map((t) => t.textContent)).toEqual(['CA-HU-04'])
  })

  it('los enlaces van en el orden de las marcas, que es el del teclado', () => {
    const orden = ['CA-HU-09', 'CA-TA-01', 'CA-HU-04']
    const marcas = new Map(orden.map((id) => [id, { banda: 'mala' as const, alerta: false, nombre: `${id}: de prueba` }]))
    render(
      <MemoryRouter>
        <MapaPeru geo={PERU} lugares={LUGARES} marcas={marcas} enlaceDe={(id) => `/estacion/${id}`} titulo="Mapa de prueba" />
      </MemoryRouter>,
    )

    expect(screen.getAllByRole('link').map((e) => e.getAttribute('data-estacion'))).toEqual(orden)
  })

  it('no usa estilos en línea: la política de contenido no los admite', () => {
    montar()

    expect(document.querySelectorAll('[style]')).toHaveLength(0)
  })
})
