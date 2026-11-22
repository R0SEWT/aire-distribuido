// El comportamiento del Panel (spec §5.2 y §12): sus estados, los tres estados
// de una estación, el orden por gravedad y la llegada de un evento.

import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { fechaHora } from '../../componentes/indice'
import { cerrarSesion, recargarSesion } from '../../sesion/almacen'
import { EN_ALERTA, estadoDe, montar, simularPanel } from './dobles'

const SIN_MEDIA = { pm2_5: null, pm10: null, inca: null, lecturas: 3 } as const

/** Los códigos de la lista, en el orden en que se ven. */
async function codigosDeLaLista() {
  const tabla = await screen.findByRole('table', { name: /por gravedad/ })
  return within(tabla)
    .getAllByRole('row')
    .slice(1)
    .map((fila) => within(fila).getAllByRole('link')[0]!.textContent)
}

const marca = (id: string) => document.querySelector(`[data-estacion="${id}"]`)!
const pedidosDe = (fetch: ReturnType<typeof simularPanel>, ruta: string) =>
  fetch.mock.calls.filter((c) => new URL(c[0].url).pathname === ruta).length

describe('el Panel', () => {
  beforeEach(() => recargarSesion())
  afterEach(() => cerrarSesion('salir'))

  describe('los estados de la pantalla', () => {
    it('mientras carga lo dice, con el título a la vista', () => {
      simularPanel([])

      montar('/')

      expect(screen.getByRole('heading', { name: 'Panel' })).toBeInTheDocument()
      expect(screen.getByRole('status')).toHaveTextContent('Cargando las estaciones…')
    })

    it('si la API falla lo dice con sus palabras y ofrece reintentar', async () => {
      let caida = true
      simularPanel([estadoDe('CA-HU-04', EN_ALERTA)], {
        'GET /api/estaciones': () =>
          caida ? { estado: 503, cuerpo: { error: 'MongoDB no disponible' } } : { estado: 200, cuerpo: [] },
      })
      montar('/')

      const error = await screen.findByRole('alert', {}, { timeout: 5000 })
      expect(error).toHaveTextContent('El servicio no está disponible')
      expect(error).toHaveTextContent('MongoDB no disponible')

      caida = false
      await userEvent.click(within(error).getByRole('button', { name: 'Reintentar' }))

      expect(await screen.findByText('El catálogo de estaciones está vacío')).toBeInTheDocument()
    })

    it('si no se puede leer /datos/impacto.json no adivina qué estaciones miden: es un error', async () => {
      simularPanel([], { 'GET /datos/impacto.json': { estado: 404 } })

      montar('/')

      expect(await screen.findByRole('alert')).toHaveTextContent('No se encontró lo que se pidió')
    })

    it('si falla solo el mapa, la lista sigue ahí', async () => {
      simularPanel([estadoDe('CA-HU-04', EN_ALERTA)], { 'GET /datos/peru.geojson': { estado: 404 } })

      montar('/')

      expect(await codigosDeLaLista()).toContain('CA-HU-04')
      expect(await screen.findByRole('alert')).toHaveTextContent('No se encontró lo que se pidió')
    })
  })

  describe('con un sistema recién levantado', () => {
    it('no muestra tres ceros: explica por qué no hay nada y manda a simulación', async () => {
      simularPanel([])
      montar('/')

      const vacio = await screen.findByText('Ninguna estación tiene estado todavía')
      expect(screen.getByText(/El sistema no recibe lecturas en vivo/)).toBeInTheDocument()
      expect(screen.queryByText('Estaciones en alerta')).not.toBeInTheDocument()
      expect(screen.queryByText(/En el último estado recibido/)).not.toBeInTheDocument()
      expect(vacio.closest('[data-estado]')).toHaveAttribute('data-estado', 'vacio')

      await userEvent.click(screen.getByRole('button', { name: 'Ir a simulación' }))

      expect(await screen.findByRole('heading', { name: 'Simulación de prueba' })).toBeInTheDocument()
    })

    it('a un admin le ofrece cargar el escenario', async () => {
      simularPanel([])
      montar('/', 'admin')

      expect(await screen.findByRole('button', { name: 'Ir a cargar el escenario' })).toBeInTheDocument()
      expect(screen.getByText(/48 horas reales de diciembre de 2021/)).toBeInTheDocument()
    })

    it('el mapa ya ubica a las estaciones, apagadas o sin datos, con su palabra', async () => {
      simularPanel([])
      montar('/')

      const mapa = await screen.findByRole('group', { name: /Mapa del Perú con las 6 estaciones/ })
      expect(within(mapa).getAllByRole('link')).toHaveLength(6)
      expect(within(mapa).getByRole('link', { name: 'CA-01, Chimbote: no mide pm por hora' })).toHaveAttribute('data-banda', 'no_mide')
      expect(within(mapa).getByRole('link', { name: 'CA-HU-04, Lurigancho: sin datos suficientes' })).toHaveAttribute(
        'href',
        '/estacion/CA-HU-04',
      )
    })

    it('el primer evento que llega lo llena sin recargar', async () => {
      simularPanel([])
      const { enchufes } = montar('/')
      await screen.findByText('Ninguna estación tiene estado todavía')
      act(() => enchufes.ultimo().abrir())

      act(() => enchufes.ultimo().recibir({ evento: 'estacion.regimen', datos: estadoDe('CA-HU-04', EN_ALERTA) }))

      expect(await screen.findByText(/hay 1 estación en alerta/)).toBeInTheDocument()
      expect(screen.queryByText('Ninguna estación tiene estado todavía')).not.toBeInTheDocument()
    })
  })

  describe('con estados', () => {
    const ESTADOS = [
      estadoDe('CA-AYAS-01', { inca: 'moderada', pm2_5: 16.7, pm10: 55.7 }),
      estadoDe('CA-HU-04', { ...EN_ALERTA, hora: 12 }),
      estadoDe('CA-MAC-01', SIN_MEDIA),
    ]

    it('cada estación está en uno de los tres estados, con su palabra además de su color', async () => {
      simularPanel(ESTADOS)
      montar('/')
      const tabla = await screen.findByRole('table', { name: /por gravedad/ })
      await screen.findByRole('group', { name: /Mapa del Perú/ })

      // Con estado: la banda con su palabra y, si hay alerta, la etiqueta y el anillo.
      const enAlerta = within(tabla).getByRole('row', { name: /CA-HU-04/ })
      expect(within(enAlerta).getByText('Mala')).toBeInTheDocument()
      expect(within(enAlerta).getByText('En alerta')).toBeInTheDocument()
      expect(marca('CA-HU-04')).toHaveAttribute('data-banda', 'mala')
      expect(marca('CA-HU-04').querySelector('[data-anillo]')).not.toBeNull()
      expect(marca('CA-HU-04')).toHaveAccessibleName('CA-HU-04, Lurigancho: INCA mala, en alerta')
      expect(marca('CA-AYAS-01').querySelector('[data-anillo]')).toBeNull()

      // Sin datos suficientes: recibió lecturas, pero menos de 18; o todavía ninguna.
      const pocas = within(tabla).getByRole('row', { name: /CA-MAC-01/ })
      expect(within(pocas).getByText('Sin datos suficientes')).toBeInTheDocument()
      expect(within(pocas).getByText('3 de 18 lecturas en la ventana')).toBeInTheDocument()
      expect(within(within(tabla).getByRole('row', { name: /CA-CG-02/ })).getByText('todavía no recibió lecturas')).toBeInTheDocument()
      expect(marca('CA-MAC-01').querySelector('[data-glifo="hueco"]')).not.toBeNull()

      // No mide PM por hora: fuera de la lista, apagada en el mapa y nombrada aparte.
      expect(within(tabla).queryByRole('row', { name: /CA-01/ })).not.toBeInTheDocument()
      expect(marca('CA-01').querySelector('[data-glifo="rombo"]')).not.toBeNull()
      expect(screen.getByText('1 estaciones no miden PM por hora y nunca van a tener estado')).toBeInTheDocument()
      expect(within(screen.getByRole('list', { name: 'Estaciones que no miden PM por hora' })).getByRole('link', { name: 'CA-01' })).toBeInTheDocument()
    })

    it('la leyenda del mapa nombra cada marca y dice cuántas hay', async () => {
      simularPanel(ESTADOS)
      montar('/')

      const leyenda = within(await screen.findByRole('list', { name: 'Leyenda del mapa' }))
      const textos = leyenda.getAllByRole('listitem').map((e) => e.textContent)
      expect(textos).toEqual([
        'INCA buena0',
        'INCA moderada1',
        'INCA mala1',
        'INCA umbral de cuidado0',
        'Sin datos suficientes3',
        'No mide PM por hora1',
        'Con anillo: en alerta1',
      ])
    })

    it('la lista va de la más grave a la menos', async () => {
      simularPanel(ESTADOS)
      montar('/')

      // En alerta, moderada, sin datos con lecturas, y las que no recibieron nada por código.
      expect(await codigosDeLaLista()).toEqual(['CA-HU-04', 'CA-AYAS-01', 'CA-MAC-01', 'CA-CG-02', 'CA-HUAR-01'])
    })

    it('cada fila trae lo que pide el panel, con la fecha de ese estado', async () => {
      simularPanel(ESTADOS)
      montar('/')
      const tabla = await screen.findByRole('table', { name: /por gravedad/ })

      const fila = within(tabla).getByRole('row', { name: /CA-HU-04/ })
      expect(within(fila).getByRole('link', { name: 'CA-HU-04' })).toHaveAttribute('href', '/estacion/CA-HU-04')
      expect(fila).toHaveTextContent('Lurigancho · ladrilleras')
      expect(fila).toHaveTextContent('88.8')
      expect(fila).toHaveTextContent('120.7')
      const [dia, hora] = fechaHora('2021-12-15T12:00:00-05:00').split(', ')
      expect(fila).toHaveTextContent(dia!)
      expect(fila).toHaveTextContent(hora!)
      expect(within(tabla).getByRole('row', { name: /CA-HUAR-01/ })).toHaveTextContent('distrito sin dato · minería')
      expect(within(tabla).getByRole('row', { name: /CA-MAC-01/ })).toHaveTextContent('La Huaca · actividad sin clasificar')
    })

    it('una media apenas sobre el ECA lleva en la tabla los decimales que hagan falta para verlo', async () => {
      simularPanel([estadoDe('CA-HU-04', { pm2_5: 50.03, pm10: 100.04, alerta: true, inca: 'mala' }), estadoDe('CA-AYAS-01', { pm2_5: 8.26, pm10: 20 })])
      montar('/')
      const tabla = await screen.findByRole('table', { name: /por gravedad/ })

      const enAlerta = within(tabla).getByRole('row', { name: /CA-HU-04/ })
      expect(within(enAlerta).getByText('50.03')).toBeInTheDocument()
      expect(within(enAlerta).getByText('100.04')).toBeInTheDocument()
      // Lejos del umbral, un decimal como siempre.
      const limpia = within(tabla).getByRole('row', { name: /CA-AYAS-01/ })
      expect(within(limpia).getByText('8.3')).toBeInTheDocument()
      expect(within(limpia).getByText('20.0')).toBeInTheDocument()
    })

    it('el régimen y su «excede el ECA» son los del estado guardado', async () => {
      const fetch = simularPanel([estadoDe('CA-AYAS-01', { regimen: 1, excede_eca: true }), estadoDe('CA-HU-04', { regimen: 2 })])
      montar('/')
      const tabla = await screen.findByRole('table', { name: /por gravedad/ })

      expect(within(tabla).getByRole('row', { name: /CA-AYAS-01/ })).toHaveTextContent('excede el ECA')
      expect(within(tabla).getByRole('row', { name: /CA-HU-04/ })).not.toHaveTextContent('excede el ECA')
      expect(pedidosDe(fetch, '/api/modelos/activo')).toBe(0)
    })

    it('arriba van las tres cifras y la frase: es el último estado recibido, con su fecha', async () => {
      simularPanel(ESTADOS)
      montar('/')
      await screen.findByText('Estaciones en alerta')

      const cifras = document.querySelector('[data-cifras]')!
      expect(cifras).toHaveTextContent('Estaciones en alerta1de 2 con estado')
      expect(cifras).toHaveTextContent('Estaciones con estado2de 5 que miden PM por hora')
      expect(cifras).toHaveTextContent(`Último estado recibido${fechaHora('2021-12-15T12:00:00-05:00')}`)
      expect(cifras).toHaveTextContent(`CA-HU-04 · llegó el ${fechaHora('2026-11-21T17:12:00-05:00')}`)
      expect(screen.getByText('En el último estado recibido hay 1 estación en alerta: su media de 24 horas supera el ECA.')).toBeInTheDocument()
      expect(document.body).not.toHaveTextContent(/\bahora\b/i)
    })

    it('un evento que llega con la página abierta la cambia sin volver a pedir la lista', async () => {
      const fetch = simularPanel(ESTADOS)
      const { enchufes } = montar('/')
      expect(await codigosDeLaLista()).toEqual(['CA-HU-04', 'CA-AYAS-01', 'CA-MAC-01', 'CA-CG-02', 'CA-HUAR-01'])
      await screen.findByRole('group', { name: /Mapa del Perú/ })
      act(() => enchufes.ultimo().abrir())
      await waitFor(() => expect(pedidosDe(fetch, '/api/regimenes')).toBe(2)) // al conectar se refresca

      // CA-MAC-01 junta sus lecturas y pasa a estar en alerta con una banda peor.
      act(() =>
        enchufes.ultimo().recibir({
          evento: 'estacion.regimen',
          datos: estadoDe('CA-MAC-01', { hora: 13, pm2_5: 130.2, pm10: 260, alerta: true, inca: 'umbral_de_cuidado' }),
        }),
      )

      await waitFor(async () => expect(await codigosDeLaLista()).toEqual(['CA-MAC-01', 'CA-HU-04', 'CA-AYAS-01', 'CA-CG-02', 'CA-HUAR-01']))
      expect(screen.getByText(/hay 2 estaciones en alerta/)).toBeInTheDocument()
      expect(document.querySelector('[data-cifras]')).toHaveTextContent('Estaciones en alerta2de 3 con estado')
      expect(document.querySelector('[data-cifras]')).toHaveTextContent(fechaHora('2021-12-15T13:00:00-05:00'))
      expect(marca('CA-MAC-01')).toHaveAttribute('data-banda', 'umbral_de_cuidado')
      expect(marca('CA-MAC-01').querySelector('[data-anillo]')).not.toBeNull()
      expect(pedidosDe(fetch, '/api/regimenes')).toBe(2)
    })
  })
})
