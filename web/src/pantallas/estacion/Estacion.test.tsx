// El comportamiento de la Estación (spec §5.3): sus estados, la tarjeta con el
// estado guardado, la curva de lo recibido en la sesión y la llegada de un evento.

import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { anotarVista, lecturasVistas } from '../../api/vistas'
import { fechaHora } from '../../componentes/indice'
import { cerrarSesion, recargarSesion } from '../../sesion/almacen'
import { EN_ALERTA, estadoDe, montar, simularPanel } from '../panel/dobles'

const REGIMEN = (id: string) => `GET /api/estaciones/${id}/regimen`
const SIN_REGIMEN = { estado: 404, cuerpo: { error: 'la estación todavía no tiene un régimen registrado' } }

/** El estado de una estación sin el código: lo que responde GET /api/estaciones/{id}/regimen. */
function regimenDe(...args: Parameters<typeof estadoDe>) {
  const { estacion: _estacion, ...regimen } = estadoDe(...args)
  return { estado: 200, cuerpo: regimen }
}

/** La tarjeta principal: la del último estado recibido. */
const tarjeta = (id: string) => within(screen.getByText(`${id} · último estado recibido`).closest('section')!)
const puntos = () => document.querySelectorAll('[data-capa="curva"] [data-punto]')
const pedidosDe = (fetch: ReturnType<typeof simularPanel>, ruta: string) =>
  fetch.mock.calls.filter((c) => new URL(c[0].url).pathname === ruta).length

describe('la Estación', () => {
  beforeEach(() => recargarSesion())
  afterEach(() => cerrarSesion('salir'))

  describe('los estados de la pantalla', () => {
    it('mientras carga lo dice, con el título a la vista', () => {
      simularPanel([])

      montar('/estacion/CA-HU-04')

      expect(screen.getByRole('heading', { name: 'Estación' })).toBeInTheDocument()
      expect(screen.getByRole('status')).toHaveTextContent('Cargando las estaciones…')
    })

    it('sin estación elegida pide elegir una, y el selector lleva a ella', async () => {
      simularPanel([], { [REGIMEN('CA-HU-04')]: regimenDe('CA-HU-04', EN_ALERTA) })
      montar('/estacion')

      expect(await screen.findByText('Elige una estación')).toBeInTheDocument()
      const selector = screen.getByRole('combobox', { name: 'Elegir estación' })
      expect(within(selector).getByRole('group', { name: 'No miden PM por hora' })).toHaveTextContent('CA-01 · Chimbote')

      await userEvent.selectOptions(selector, 'CA-HU-04')

      expect(await screen.findByText('CA-HU-04 · último estado recibido')).toBeInTheDocument()
    })

    it('una estación que no está en el catálogo lo dice, sin preguntarle a la API por su estado', async () => {
      const fetch = simularPanel([])
      montar('/estacion/CA-XX-99')

      expect(await screen.findByText('No hay ninguna estación «CA-XX-99»')).toBeInTheDocument()
      expect(pedidosDe(fetch, '/api/estaciones/CA-XX-99/regimen')).toBe(0)
    })

    it('si la API falla lo dice y ofrece reintentar; la ficha sigue a la vista', async () => {
      let caida = true
      simularPanel([], {
        [REGIMEN('CA-HU-04')]: () => (caida ? { estado: 503, cuerpo: { error: 'Redis no disponible' } } : regimenDe('CA-HU-04', EN_ALERTA)),
      })
      montar('/estacion/CA-HU-04')

      const error = await screen.findByRole('alert', {}, { timeout: 5000 })
      expect(error).toHaveTextContent('El servicio no está disponible')
      expect(error).toHaveTextContent('Redis no disponible')
      expect(screen.getByText('Objeto de CA-HU-04')).toBeInTheDocument()

      caida = false
      await userEvent.click(within(error).getByRole('button', { name: 'Reintentar' }))

      expect(await screen.findByText('CA-HU-04 · último estado recibido')).toBeInTheDocument()
    })

    it('una estación que mide y todavía no recibió lecturas no es un error', async () => {
      simularPanel([], { [REGIMEN('CA-CG-02')]: SIN_REGIMEN })
      montar('/estacion/CA-CG-02')

      const vacio = await screen.findByText('CA-CG-02 todavía no tiene estado')
      expect(vacio.closest('[data-estado]')).toHaveAttribute('data-estado', 'vacio')
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
      expect(screen.getByText(/todavía no recibió ningún estado de CA-CG-02/)).toBeInTheDocument()

      await userEvent.click(screen.getByRole('button', { name: 'Ir a simulación' }))
      expect(await screen.findByRole('heading', { name: 'Simulación de prueba' })).toBeInTheDocument()
    })

    it('una estación que no mide PM por hora lo dice: nunca va a tener estado ni curva', async () => {
      simularPanel([], { [REGIMEN('CA-01')]: SIN_REGIMEN })
      montar('/estacion/CA-01')

      expect(await screen.findByText('CA-01 no mide PM por hora')).toBeInTheDocument()
      expect(screen.getByText(/nunca va a tener estado/)).toBeInTheDocument()
      expect(screen.queryByText('Lo recibido en esta sesión')).not.toBeInTheDocument()
      expect(screen.getByText('No mide PM2.5 y PM10 por hora: no hay fechas que dar.')).toBeInTheDocument()
    })
  })

  describe('con estado', () => {
    it('la tarjeta muestra las dos medias en grande, la ventana, la alerta, la banda y la fecha del estado', async () => {
      simularPanel([], { [REGIMEN('CA-HU-04')]: regimenDe('CA-HU-04', { ...EN_ALERTA, hora: 12 }) })
      montar('/estacion/CA-HU-04')
      await screen.findByText('CA-HU-04 · último estado recibido')

      expect(tarjeta('CA-HU-04').getByText('88.8')).toHaveClass('text-6xl')
      expect(tarjeta('CA-HU-04').getByText('120.7')).toHaveClass('text-6xl')
      const estado = within(document.querySelector<HTMLElement>('[data-estado-estacion]')!)
      expect(estado.getByText('INCA mala')).toBeInTheDocument()
      expect(estado.getByText('En alerta')).toBeInTheDocument()
      expect(estado.getByText('24')).toBeInTheDocument()
      expect(screen.getByText(fechaHora('2021-12-15T12:00:00-05:00'))).toBeInTheDocument()
      expect(
        screen.getByText('La media de las últimas 24 horas de PM2.5 es 88.8 µg/m³ y la de PM10 es 120.7 µg/m³: las dos superan el ECA (50 y 100).'),
      ).toBeInTheDocument()
      expect(document.body).not.toHaveTextContent(/\bahora\b/i)
    })

    it('el «excede el ECA» del régimen es el del estado guardado: no se le pregunta al modelo activo', async () => {
      const fetch = simularPanel([], { [REGIMEN('CA-HU-04')]: regimenDe('CA-HU-04', { regimen: 1, excede_eca: true }) })
      montar('/estacion/CA-HU-04')
      await screen.findByText('CA-HU-04 · último estado recibido')

      const estado = document.querySelector('[data-estado-estacion]')!
      expect(estado).toHaveTextContent('Régimen1, el de la última lectura; su media excede el ECA')
      expect(within(estado as HTMLElement).getByText('Sin alerta')).toBeInTheDocument()
      expect(pedidosDe(fetch, '/api/modelos/activo')).toBe(0)
    })

    it('con menos de 18 lecturas no hay media: lo dice en vez de mostrar un cero', async () => {
      simularPanel([], { [REGIMEN('CA-MAC-01')]: regimenDe('CA-MAC-01', { pm2_5: null, pm10: null, inca: null, lecturas: 3 }) })
      montar('/estacion/CA-MAC-01')
      await screen.findByText('CA-MAC-01 · último estado recibido')

      expect(screen.getAllByText('sin datos suficientes')).toHaveLength(2)
      expect(within(document.querySelector<HTMLElement>('[data-estado-estacion]')!).getByText('Sin datos suficientes')).toBeInTheDocument()
      expect(screen.getByText('Todavía no hay media de 24 horas: la ventana tiene 3 lecturas y hacen falta 18.')).toBeInTheDocument()
      expect(document.querySelectorAll('[data-punto="sin-media"]')).toHaveLength(1)
    })

    it('«¿De dónde sale este número?» da la regla, la norma y por qué el régimen no es la alerta', async () => {
      simularPanel([], { [REGIMEN('CA-HU-04')]: regimenDe('CA-HU-04', EN_ALERTA) })
      montar('/estacion/CA-HU-04')

      const explicacion = (await screen.findByText('¿De dónde sale este número?')).closest('details')!
      expect(explicacion).toHaveTextContent('DS 003-2017-MINAM')
      expect(explicacion).toHaveTextContent('RM 181-2016-MINAM')
      expect(explicacion).toHaveTextContent('más de 50 µg/m³ de PM2.5 o más de 100 µg/m³ de PM10')
      expect(explicacion).toHaveTextContent('El régimen no es la alerta')
    })

    it('la ficha dice qué vigila, dónde, cuándo midió y la población del distrito, como contexto', async () => {
      simularPanel([], { [REGIMEN('CA-HU-04')]: regimenDe('CA-HU-04', EN_ALERTA) })
      montar('/estacion/CA-HU-04')

      const ficha = (await screen.findByRole('heading', { name: 'Objeto de CA-HU-04' })).closest('section')!
      expect(ficha).toHaveTextContent('Tipo de actividadladrilleras')
      expect(ficha).toHaveTextContent('DistritoLurigancho')
      expect(ficha).toHaveTextContent(/Del 10 dic\.? 2021 al 20 dic\.? 2021: 11 días con datos/)
      expect(ficha).toHaveTextContent('240,814 habitantes en Lurigancho')
      expect(within(ficha).getByRole('link', { name: 'INEI, censo de 2017' })).toHaveAttribute('href', 'https://www.inei.gob.pe/ejemplo.pdf')
      expect(ficha).toHaveTextContent('Es contexto, no gente expuesta')
    })

    it('lo que la fuente no trae se dice, no se inventa', async () => {
      simularPanel([], { [REGIMEN('CA-HUAR-01')]: regimenDe('CA-HUAR-01') })
      montar('/estacion/CA-HUAR-01')

      const ficha = (await screen.findByRole('heading', { name: 'Objeto de CA-HUAR-01' })).closest('section')!
      expect(ficha).toHaveTextContent('Distritola fuente no lo dice')
      expect(ficha).toHaveTextContent('Población del distritola fuente no lo dice')
    })
  })

  describe('la curva', () => {
    it('tiene solo lo que la pestaña recibió: con un GET, un punto', async () => {
      simularPanel([], { [REGIMEN('CA-HU-04')]: regimenDe('CA-HU-04', EN_ALERTA) })
      montar('/estacion/CA-HU-04')
      await screen.findByText('CA-HU-04 · último estado recibido')

      expect(screen.getByRole('heading', { name: 'Media de 24 horas de PM10' })).toBeInTheDocument()
      expect(puntos()).toHaveLength(1)
      expect(document.querySelector('[data-curva-pie]')).toHaveTextContent('1 estado recibido por esta pestaña')
      expect(document.querySelector('[data-curva-pie]')).toHaveTextContent('No es una serie histórica')
      expect(screen.getByRole('img', { name: /Media de 24 horas de PM10 de CA-HU-04: 1 estado recibido/ })).toBeInTheDocument()
      // Sin estilos en línea: la política de contenido no los admite.
      expect(document.querySelectorAll('[style]')).toHaveLength(0)
    })

    it('lleva la línea del ECA y las bandas INCA del contaminante elegido, con sus palabras', async () => {
      simularPanel([], { [REGIMEN('CA-HU-04')]: regimenDe('CA-HU-04', EN_ALERTA) })
      montar('/estacion/CA-HU-04')
      await screen.findByText('CA-HU-04 · último estado recibido')

      const curva = screen.getByRole('img', { name: /Media de 24 horas de PM10/ })
      expect(curva).toHaveTextContent('ECA 100 µg/m³')
      expect([...curva.querySelectorAll('[data-franja]')].map((f) => f.getAttribute('data-franja'))).toEqual(['buena', 'moderada'])
      expect(curva).toHaveTextContent('Buena')
      expect(curva.querySelector('[data-punto="sobre-eca"]')).not.toBeNull()

      await userEvent.click(screen.getByRole('button', { name: 'PM2.5' }))

      const otra = screen.getByRole('img', { name: /Media de 24 horas de PM2.5/ })
      expect(otra).toHaveTextContent('ECA 50 µg/m³')
      expect(otra.querySelectorAll('[data-franja]')).toHaveLength(3)
    })

    it('abre con el contaminante que supera el ECA', async () => {
      simularPanel([], { [REGIMEN('CA-HU-04')]: regimenDe('CA-HU-04', { pm2_5: 50.3, pm10: 73.5, alerta: true, inca: 'mala' }) })
      montar('/estacion/CA-HU-04')

      expect(await screen.findByRole('heading', { name: 'Media de 24 horas de PM2.5' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'PM2.5' })).toHaveAttribute('aria-pressed', 'true')
    })

    it('un evento que llega con la página abierta cambia la tarjeta y suma un punto, sin volver a pedir', async () => {
      const fetch = simularPanel([], { [REGIMEN('CA-HU-04')]: regimenDe('CA-HU-04', { hora: 10, pm2_5: 20, pm10: 90, inca: 'moderada' }) })
      const { enchufes } = montar('/estacion/CA-HU-04')
      await screen.findByText('CA-HU-04 · último estado recibido')
      act(() => enchufes.ultimo().abrir())
      await waitFor(() => expect(pedidosDe(fetch, '/api/estaciones/CA-HU-04/regimen')).toBe(2)) // al conectar se refresca
      expect(puntos()).toHaveLength(1)
      expect(screen.getByText('Sin alerta')).toBeInTheDocument()

      act(() => enchufes.ultimo().recibir({ evento: 'estacion.regimen', datos: estadoDe('CA-HU-04', { ...EN_ALERTA, hora: 11 }) }))
      // El de otra estación no toca esta página.
      act(() => enchufes.ultimo().recibir({ evento: 'estacion.regimen', datos: estadoDe('CA-AYAS-01', { hora: 11 }) }))

      expect(await tarjeta('CA-HU-04').findByText('120.7')).toBeInTheDocument()
      expect(screen.getByText('En alerta')).toBeInTheDocument()
      expect(screen.getByText(fechaHora('2021-12-15T11:00:00-05:00'))).toBeInTheDocument()
      expect(puntos()).toHaveLength(2)
      expect(document.querySelector('[data-curva-pie]')).toHaveTextContent('2 estados recibidos por esta pestaña')
      expect(pedidosDe(fetch, '/api/estaciones/CA-HU-04/regimen')).toBe(2)
    })

    it('una estación sin estado se llena sola con su primer evento', async () => {
      simularPanel([], { [REGIMEN('CA-CG-02')]: SIN_REGIMEN })
      const { enchufes } = montar('/estacion/CA-CG-02')
      await screen.findByText('CA-CG-02 todavía no tiene estado')
      act(() => enchufes.ultimo().abrir())

      act(() => enchufes.ultimo().recibir({ evento: 'estacion.regimen', datos: estadoDe('CA-CG-02', { pm2_5: null, pm10: null, inca: null, lecturas: 1 }) }))

      expect(await screen.findByText('CA-CG-02 · último estado recibido')).toBeInTheDocument()
      expect(screen.getByText('Todavía no hay media de 24 horas: la ventana tiene 1 lectura y hacen falta 18.')).toBeInTheDocument()
    })

    it('lo que el Panel recibió antes de entrar a la estación ya está en la curva', async () => {
      simularPanel([estadoDe('CA-HU-04', { hora: 10, pm10: 90, inca: 'moderada' })], {
        [REGIMEN('CA-HU-04')]: regimenDe('CA-HU-04', { ...EN_ALERTA, hora: 12 }),
      })
      const { enchufes } = montar('/')
      const tabla = await screen.findByRole('table', { name: /por gravedad/ })
      act(() => enchufes.ultimo().abrir())
      act(() => enchufes.ultimo().recibir({ evento: 'estacion.regimen', datos: estadoDe('CA-HU-04', { ...EN_ALERTA, hora: 11 }) }))
      act(() => enchufes.ultimo().recibir({ evento: 'estacion.regimen', datos: estadoDe('CA-HU-04', { ...EN_ALERTA, hora: 12 }) }))
      await within(tabla).findByText('En alerta')

      await userEvent.click(within(tabla).getByRole('link', { name: 'CA-HU-04' }))

      await screen.findByText('CA-HU-04 · último estado recibido')
      expect(puntos()).toHaveLength(3)
    })
  })
})

describe('la curva de la Estación con lecturas enviadas en la sesión', () => {
  beforeEach(() => recargarSesion())
  afterEach(() => cerrarSesion('salir'))

  const lecturas = () => document.querySelectorAll('[data-capa="lecturas"] [data-lectura]')
  const vista = (hora: number, pm10: number, media: number | null) => ({
    fecha_hora: `2021-12-15T${String(hora).padStart(2, '0')}:00:00-05:00`,
    pm2_5: pm10 / 2,
    pm10,
    media_pm2_5: media === null ? null : media / 2,
    media_pm10: media,
    lecturas: 24,
    alerta: (media ?? 0) > 100,
    inca: null,
    regimen: 2,
    excede_eca: true,
  })

  it('sin lecturas enviadas dibuja solo la media y dice que las lecturas no se ven', async () => {
    simularPanel([], { [REGIMEN('CA-HU-04')]: regimenDe('CA-HU-04', EN_ALERTA) })
    montar('/estacion/CA-HU-04')
    await screen.findByText('CA-HU-04 · último estado recibido')

    expect(lecturas()).toHaveLength(0)
    expect(document.querySelector('[data-capa="leyenda"]')).toBeNull()
    const pie = document.querySelector('[data-curva-pie]')!
    expect(pie).toHaveAttribute('data-curva', 'media')
    expect(pie).toHaveTextContent('Las lecturas horarias no se ven: el estado no las trae, y esta sesión no envió ninguna de CA-HU-04 desde Simulación.')
  })

  it('con lecturas enviadas dibuja cada lectura y la línea de la media, con su leyenda y su texto', async () => {
    simularPanel([], { [REGIMEN('CA-HU-04')]: regimenDe('CA-HU-04', { ...EN_ALERTA, hora: 10 }) })
    montar('/estacion/CA-HU-04')
    await screen.findByText('CA-HU-04 · último estado recibido')

    // Lo que hace el simulador con cada lectura que manda y su respuesta.
    act(() => {
      anotarVista('CA-HU-04', vista(8, 260, 110))
      anotarVista('CA-HU-04', vista(9, 190, 115.2))
      anotarVista('CA-HU-04', vista(10, 140, 120.7))
      anotarVista('CA-AYAS-01', vista(10, 30, 20))
    })

    expect(await screen.findByRole('heading', { name: 'Lecturas horarias y media de 24 horas de PM10' })).toBeInTheDocument()
    expect(screen.getByText('Lo enviado y lo recibido en esta sesión')).toBeInTheDocument()
    expect(lecturas()).toHaveLength(3)
    // La media: tres horas, una de ellas (las 10) también recibida como estado. No se duplica.
    expect(puntos()).toHaveLength(3)
    const curva = screen.getByRole('img', { name: /Lecturas horarias y media de 24 horas de PM10 de CA-HU-04: 3 lecturas horarias enviadas/ })
    expect(within(curva.querySelector<SVGGElement>('[data-capa="leyenda"]')! as unknown as HTMLElement).getByText('Lectura horaria')).toBeInTheDocument()
    expect(curva).toHaveTextContent('Media de 24 h')
    const pie = document.querySelector('[data-curva-pie]')!
    expect(pie).toHaveAttribute('data-curva', 'lecturas-y-media')
    expect(pie).toHaveTextContent('3 lecturas horarias de PM10 que esta sesión envió desde Simulación')
    expect(pie).toHaveTextContent('Cada rombo es una de esas lecturas')
    expect(pie).toHaveTextContent('El ECA es de 24 horas: se compara con la media, no con cada lectura.')
    // Una lectura de 260 no es un punto «sobre el ECA»: eso se dice de la media.
    expect(curva.querySelectorAll('[data-punto="sobre-eca"]')).toHaveLength(3)
    expect(document.querySelectorAll('[style]')).toHaveLength(0)
  })

  it('al terminar la sesión, lo enviado no queda para quien entre después', async () => {
    simularPanel([], { [REGIMEN('CA-HU-04')]: regimenDe('CA-HU-04', EN_ALERTA) })
    act(() => anotarVista('CA-HU-04', vista(8, 260, 110)))
    expect(lecturasVistas('CA-HU-04')).toHaveLength(1)

    montar('/estacion/CA-HU-04')
    await screen.findByText('CA-HU-04 · último estado recibido')
    act(() => cerrarSesion('salir'))

    expect(lecturasVistas('CA-HU-04')).toHaveLength(0)
  })
})
