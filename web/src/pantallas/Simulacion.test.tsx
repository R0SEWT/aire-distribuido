// La pantalla de Simulación contra una API falsa que aplica la regla de la
// ventana de 24 horas: qué se manda y en qué orden, la pausa, un error a mitad
// de camino, la comparación con lo esperado y el aviso de sistema no limpio.

import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { crearClienteDeConsultas } from '../api/cliente'
import type { EstadoEstacion } from '../api/enVivo'
import { lecturasVistas } from '../api/vistas'
import { Proveedores } from '../App'
import { fabricaDeEnchufes, respuesta } from '../pruebas/dobles'
import { abrirSesion, cerrarSesion, recargarSesion, type Rol } from '../sesion/almacen'
import Simulacion from './Simulacion'
import { apiDePrueba, SERIES } from './simulacion/apiDePrueba'
import { Simulador } from './simulacion/motor'

const LARGO = 30_000

function montar(rol: Rol) {
  abrirSesion({ usuario: rol, token: `token-de-${rol}`, rol, expira: Date.now() + 3_600_000 })
  // Sin esperas entre horas: la prueba no depende del reloj.
  const simulador = new Simulador({ esperar: async () => {} })
  const enchufes = fabricaDeEnchufes()
  render(
    <Proveedores consultas={crearClienteDeConsultas()} tiempoReal={{ crearEnchufe: enchufes.crearEnchufe }}>
      <Simulacion simulador={simulador} />
    </Proveedores>,
  )
  return simulador
}

function estadoDe(estacion: string, fecha_hora: string): EstadoEstacion {
  return {
    estacion,
    fecha_hora,
    regimen: 0,
    excede_eca: false,
    media_24h: { pm2_5: 8, pm10: 15, lecturas: 24 },
    alerta: false,
    inca: 'buena',
    ts: '2026-11-21T10:00:00-05:00',
  }
}

/** Un pedido que se suelta a mano. */
function puerta() {
  let abrir!: () => void
  const cerrada = new Promise<void>((r) => (abrir = r))
  return { cerrada, abrir }
}

const dia = (id: string) => SERIES.dias.find((d) => d.id === id)!
const elegirDia = (id: string) => userEvent.selectOptions(screen.getByLabelText('Estación y día'), id)
const terminar = () => screen.findByText('Terminó', {}, { timeout: LARGO })

describe('Simulación', () => {
  beforeEach(() => recargarSesion())
  afterEach(() => cerrarSesion('salir'))

  it('antes de empezar dice que la corrida vive en esta pestaña y qué pasa si se recarga o se cierra', async () => {
    apiDePrueba()
    montar('admin')

    const aviso = (await screen.findByText('La corrida vive en esta pestaña:')).closest('p')!
    expect(aviso).toHaveTextContent('recargar la página o cerrarla a mitad la corta y deja el tramo a medias en el sistema')
    expect(aviso).toHaveTextContent('Mientras esté en curso, el navegador pregunta antes de salir.')
    // Está a la vista antes de empezar: el botón todavía es el de cargar.
    expect(await screen.findByRole('button', { name: 'Cargar el escenario' })).toBeInTheDocument()
  })

  it('con una corrida en curso el navegador pregunta antes de salir; al terminar, ya no', async () => {
    const suelta = puerta()
    apiDePrueba({ alPredecir: async (_l, n) => void (n === 3 && (await suelta.cerrada)) })
    const poner = vi.spyOn(window, 'addEventListener')
    const quitar = vi.spyOn(window, 'removeEventListener')
    const avisos = (espia: typeof poner) => espia.mock.calls.filter((c) => c[0] === 'beforeunload')
    montar('usuario')
    await screen.findByRole('button', { name: 'Simular este día' })
    expect(avisos(poner)).toHaveLength(0)

    await userEvent.click(screen.getByRole('button', { name: 'Simular este día' }))

    // En curso, con la tercera lectura en viaje: el aviso está puesto, y es de los que hacen preguntar.
    await waitFor(() => expect(avisos(poner)).toHaveLength(1))
    expect(avisos(quitar)).toHaveLength(0)
    const evento = new Event('beforeunload', { cancelable: true })
    window.dispatchEvent(evento)
    expect(evento.defaultPrevented).toBe(true)

    suelta.abrir()
    await terminar()

    expect(avisos(quitar)).toHaveLength(1)
    expect(avisos(quitar)[0]![1]).toBe(avisos(poner)[0]![1])
    const despues = new Event('beforeunload', { cancelable: true })
    window.dispatchEvent(despues)
    expect(despues.defaultPrevented).toBe(false)
  }, LARGO)

  it('una lectura a mano que responde después de «Salir» no queda anotada para la sesión siguiente', async () => {
    const ESTACION = 'CA-AYAS-01'
    const suelta = puerta()
    apiDePrueba({ estados: [estadoDe(ESTACION, '2021-12-15T10:00:00-05:00')], alPredecir: async () => void (await suelta.cerrada) })
    montar('usuario')
    await userEvent.click(await screen.findByRole('button', { name: 'Una lectura a mano' }))
    await userEvent.selectOptions(screen.getByLabelText('Estación'), ESTACION)
    await userEvent.type(screen.getByLabelText('PM2.5 (µg/m³)'), '70.2')
    await userEvent.type(screen.getByLabelText('PM10 (µg/m³)'), '190.5')
    await waitFor(() => expect(screen.getByRole('button', { name: 'Enviar la lectura' })).toBeEnabled())
    await userEvent.click(screen.getByRole('button', { name: 'Enviar la lectura' }))
    await screen.findByRole('button', { name: 'Enviando…' })

    // «Salir» con el POST en viaje; la respuesta llega cuando la sesión ya no existe.
    act(() => cerrarSesion('salir'))
    suelta.abrir()
    await act(() => new Promise((r) => setTimeout(r, 50)))

    expect(lecturasVistas(ESTACION)).toEqual([])
  })

  it('si no se pueden leer las series, lo dice y no deja la pantalla en blanco', async () => {
    apiDePrueba({ series: null })

    montar('usuario')

    expect(screen.getByRole('heading', { name: 'Simulación' })).toBeInTheDocument()
    const alerta = await screen.findByRole('alert')
    expect(within(alerta).getByText(/no se pudo leer \/datos\/series\.json/)).toBeInTheDocument()
  })

  describe('el escenario', () => {
    it(
      'un admin lo carga entero: las 667 lecturas en el orden del archivo, y las 14 estaciones quedan como se esperaba',
      async () => {
        const api = apiDePrueba()
        montar('admin')

        // Lo que se va a mandar, con las cifras del archivo.
        expect(await screen.findByText(/14 y 15 de diciembre de 2021/)).toBeInTheDocument()
        expect(screen.getByText('667')).toBeInTheDocument()
        await userEvent.click(await screen.findByRole('button', { name: 'Cargar el escenario' }))
        await terminar()

        expect(api.lecturas).toHaveLength(667)
        expect(api.lecturas).toEqual(SERIES.escenario.lecturas)
        expect(screen.getByText(/14 de 14 estaciones quedaron como dice el archivo/)).toBeInTheDocument()
        expect(screen.getByText(/Terminaron en alerta CA-HU-04 y CA-HU-09/)).toBeInTheDocument()
        const tabla = screen.getByRole('table', { name: /contra lo esperado/ })
        expect(within(tabla).getAllByText('Coincide')).toHaveLength(14)
        expect(within(tabla).queryByText(/Difiere/)).not.toBeInTheDocument()
      },
      LARGO,
    )

    it('consulta el estado de las estaciones antes de mandar la primera lectura', async () => {
      const api = apiDePrueba({ alPredecir: (_, n) => (n === 3 ? respuesta(503, { error: 'corte de la prueba' }) : undefined) })
      montar('admin')

      await userEvent.click(await screen.findByRole('button', { name: 'Cargar el escenario' }))
      await waitFor(() => expect(api.lecturas.length).toBeGreaterThan(0))

      const primera = api.pedidos.indexOf('POST /api/predicciones')
      // Una vez al abrir la pantalla y otra, fresca, al apretar el botón.
      expect(api.pedidos.slice(0, primera).filter((p) => p === 'GET /api/regimenes').length).toBeGreaterThanOrEqual(2)
    })

    it('un usuario no puede cargarlo: solo simula un día o manda una lectura', async () => {
      apiDePrueba()
      montar('usuario')

      const modos = within(await screen.findByRole('group', { name: 'Qué se simula' }))
      expect(modos.queryByRole('button', { name: 'El escenario' })).not.toBeInTheDocument()
      expect(modos.getByRole('button', { name: 'Un día de una estación' })).toHaveAttribute('aria-pressed', 'true')
      expect(screen.getByText(/Cargar el escenario es de quien administra/)).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Cargar el escenario' })).not.toBeInTheDocument()
    })
  })

  describe('un día de una estación', () => {
    it('ofrece los nueve días del archivo, cada uno con su caso', async () => {
      apiDePrueba()
      montar('usuario')

      const lista = await screen.findByLabelText('Estación y día')
      expect(within(lista).getAllByRole('option')).toHaveLength(SERIES.dias.length)
      expect(SERIES.dias).toHaveLength(9)
      await elegirDia('CA-RZC-02-2021-12-21')
      expect(screen.getByText(dia('CA-RZC-02-2021-12-21').descripcion)).toBeInTheDocument()
    })

    it('manda sus lecturas en orden y dice cuándo se enciende la alerta, como espera el archivo', async () => {
      const api = apiDePrueba()
      montar('usuario')
      const d = dia('CA-ESP-01-2021-12-21')

      await screen.findByLabelText('Estación y día')
      await elegirDia(d.id)
      await userEvent.click(await screen.findByRole('button', { name: 'Simular este día' }))
      await terminar()

      expect(api.lecturas).toEqual(d.lecturas)
      expect(screen.getByText(/1 de 1 estación quedó como dice el archivo/)).toBeInTheDocument()
      expect(screen.getByText(/supera el ECA, el de PM10 \(100\)\. Hay alerta desde el 21 dic\. 2021, 07:00/)).toBeInTheDocument()
      // Las cifras son las que respondió la API en la última lectura.
      expect(screen.getAllByText('131.9').length).toBeGreaterThan(0)
      expect(screen.getAllByText('22.6').length).toBeGreaterThan(0)
    })

    it('un día con huecos que termina sin media: sin alerta ni banda, y coincide con lo esperado', async () => {
      apiDePrueba()
      montar('usuario')

      await screen.findByLabelText('Estación y día')
      await elegirDia('CA-MAC-01-2022-05-26')
      await userEvent.click(await screen.findByRole('button', { name: 'Simular este día' }))
      await terminar()

      expect(screen.getByText(/La ventana de 24 horas tiene 17 lecturas: falta 1 para que la API calcule la media/)).toBeInTheDocument()
      expect(within(screen.getByRole('table', { name: /contra lo esperado/ })).getByText('Coincide')).toBeInTheDocument()
    })

    it('si la API responde otra cosa que lo esperado, la fila dice en qué difiere', async () => {
      const d = dia('CA-HUA-01-2021-12-20')
      apiDePrueba({
        alPredecir: (_, n, calculada) =>
          n === d.lecturas.length ? respuesta(200, { ...calculada, media_24h: { ...calculada.media_24h, pm10: 99.9 }, alerta: true }) : undefined,
      })
      montar('usuario')

      await screen.findByLabelText('Estación y día')
      await elegirDia(d.id)
      await userEvent.click(await screen.findByRole('button', { name: 'Simular este día' }))
      await terminar()

      expect(screen.getByText(/La estación no quedó como dice el archivo\. Difiere 1: mira la tabla/)).toBeInTheDocument()
      const tabla = within(screen.getByRole('table', { name: /contra lo esperado/ }))
      expect(tabla.getByText('Difiere')).toBeInTheDocument()
      expect(tabla.getByText(/media de PM10, alerta y hora de la primera alerta/)).toBeInTheDocument()
      expect(tabla.getByText(/difiere: esperado 18\.2/)).toBeInTheDocument()
    })

    it('si solo difiere el régimen, lo separa: depende del modelo activo', async () => {
      const d = dia('CA-HUA-01-2021-12-20')
      apiDePrueba({ alPredecir: (_, __, calculada) => respuesta(200, { ...calculada, regimen: 2, modelo_id: 'entrenado-hoy' }) })
      montar('usuario')

      await screen.findByLabelText('Estación y día')
      await elegirDia(d.id)
      await userEvent.click(await screen.findByRole('button', { name: 'Simular este día' }))
      await terminar()

      expect(screen.getByText('Solo difiere el régimen')).toBeInTheDocument()
      expect(screen.getByText(/el esperado vale para el modelo sembrado y el activo es entrenado-hoy/)).toBeInTheDocument()
    })
  })

  describe('pausar y detener', () => {
    it('en pausa no manda la lectura siguiente; al continuar sigue donde quedó y termina', async () => {
      const d = dia('CA-HUA-01-2021-12-20')
      const paso = puerta()
      const api = apiDePrueba({ alPredecir: async (_, n) => (n === 10 ? paso.cerrada : undefined) })
      montar('usuario')
      await screen.findByLabelText('Estación y día')
      await elegirDia(d.id)
      await userEvent.click(await screen.findByRole('button', { name: 'Simular este día' }))
      await waitFor(() => expect(api.lecturas).toHaveLength(10))

      await userEvent.click(screen.getByRole('button', { name: 'Pausar' }))
      paso.abrir()

      expect(await screen.findByText('En pausa')).toBeInTheDocument()
      // La que estaba en viaje se anota; la número 11 no sale.
      await waitFor(() => expect(screen.getByRole('progressbar', { name: 'Lecturas enviadas' })).toHaveValue(10))
      await new Promise((r) => setTimeout(r, 30))
      expect(api.lecturas).toHaveLength(10)

      await userEvent.click(screen.getByRole('button', { name: 'Continuar' }))
      await terminar()

      expect(api.lecturas).toEqual(d.lecturas)
      expect(screen.getByText(/1 de 1 estación quedó como dice el archivo/)).toBeInTheDocument()
    })

    it('al detener no manda más y dice que lo enviado ya está en el sistema', async () => {
      const d = dia('CA-HUA-01-2021-12-20')
      const paso = puerta()
      const api = apiDePrueba({ alPredecir: async (_, n) => (n === 6 ? paso.cerrada : undefined) })
      montar('usuario')
      await screen.findByLabelText('Estación y día')
      await elegirDia(d.id)
      await userEvent.click(await screen.findByRole('button', { name: 'Simular este día' }))
      await waitFor(() => expect(api.lecturas).toHaveLength(6))

      await userEvent.click(screen.getByRole('button', { name: 'Detener' }))
      paso.abrir()

      expect(await screen.findByText('Detenida')).toBeInTheDocument()
      expect(screen.getByText(/Se detuvo con 5 de 48 lecturas enviadas\. Lo enviado ya está en el sistema/)).toBeInTheDocument()
      await new Promise((r) => setTimeout(r, 30))
      expect(api.lecturas).toHaveLength(6)
      expect(screen.getByText('Faltan 43 lecturas')).toBeInTheDocument()
    })

    it('mientras hay una corrida no se puede elegir otra cosa', async () => {
      const paso = puerta()
      const api = apiDePrueba({ alPredecir: async (_, n) => (n === 2 ? paso.cerrada : undefined) })
      const simulador = montar('admin')
      await userEvent.click(await screen.findByRole('button', { name: 'Cargar el escenario' }))
      await waitFor(() => expect(api.lecturas).toHaveLength(2))

      for (const nombre of ['El escenario', 'Un día de una estación', 'Una lectura a mano']) {
        expect(screen.getByRole('button', { name: nombre })).toBeDisabled()
      }
      expect(screen.queryByRole('button', { name: /Cargar el escenario/ })).not.toBeInTheDocument()
      simulador.detener()
      paso.abrir()
    })
  })

  describe('si la API falla a mitad de la corrida', () => {
    it('se detiene en esa lectura con las palabras de la API, y al reintentar la manda otra vez y termina', async () => {
      const d = dia('CA-HUA-01-2021-12-20')
      let caida = true
      const api = apiDePrueba({ alPredecir: (_, n) => (n === 20 && caida ? respuesta(503, { error: 'no hay modelo activo' }) : undefined) })
      montar('usuario')
      await screen.findByLabelText('Estación y día')
      await elegirDia(d.id)
      await userEvent.click(await screen.findByRole('button', { name: 'Simular este día' }))

      const alerta = await screen.findByRole('alert')
      expect(within(alerta).getByText(/El servicio no está disponible: no hay modelo activo/)).toBeInTheDocument()
      expect(within(alerta).getByText(/La corrida espera ahí: no la salta/)).toBeInTheDocument()
      expect(within(alerta).getByText(/Las 19 anteriores ya están en el sistema/)).toBeInTheDocument()
      expect(screen.getByText('Detenida por un error')).toBeInTheDocument()
      await new Promise((r) => setTimeout(r, 30))
      expect(api.lecturas).toHaveLength(20)

      caida = false
      await userEvent.click(screen.getByRole('button', { name: 'Reintentar esta lectura' }))
      await terminar()

      // La lectura 20 salió dos veces; ninguna se saltó.
      expect(api.lecturas).toEqual([...d.lecturas.slice(0, 20), ...d.lecturas.slice(19)])
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
      expect(screen.getByText(/1 de 1 estación quedó como dice el archivo/)).toBeInTheDocument()
    })
  })

  describe('si el sistema no está limpio', () => {
    it('avisa que el estado no retrocede, no manda nada solo y deja simular «de todos modos»', async () => {
      const d = dia('CA-HUA-01-2021-12-20')
      const api = apiDePrueba({ estados: [estadoDe('CA-HUA-01', '2022-03-05T10:00:00-05:00')] })
      montar('usuario')
      await screen.findByLabelText('Estación y día')
      await elegirDia(d.id)

      const aviso = await screen.findByRole('alert')
      expect(within(aviso).getByText('El sistema no está limpio para este tramo')).toBeInTheDocument()
      expect(within(aviso).getByText(/El último estado de una estación no retrocede/)).toBeInTheDocument()
      expect(within(aviso).getByText(/un estado posterior a todo el tramo: ninguna de sus lecturas lo va a cambiar/)).toBeInTheDocument()
      expect(within(aviso).getByText(/CA-HUA-01 · estado del 5 mar\. 2022, 10:00/)).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Simular este día' })).not.toBeInTheDocument()
      expect(api.lecturas).toHaveLength(0)

      await userEvent.click(screen.getByRole('button', { name: 'Simular este día de todos modos' }))
      await terminar()

      // La API predice igual: la respuesta coincide, pero el estado guardado no retrocedió.
      expect(api.lecturas).toEqual(d.lecturas)
      expect(api.estado.get('CA-HUA-01')?.fecha_hora).toBe('2022-03-05T10:00:00-05:00')
    })

    it('ofrece los días que todavía moverían el estado de su estación', async () => {
      apiDePrueba({ estados: [estadoDe('CA-HUA-01', '2022-03-05T10:00:00-05:00'), estadoDe('CA-SOC-01', '2022-03-05T10:00:00-05:00')] })
      montar('usuario')
      await screen.findByLabelText('Estación y día')
      await elegirDia('CA-HUA-01-2021-12-20')

      const aviso = await screen.findByRole('alert')
      const ofrecidos = within(aviso).getAllByRole('button').map((b) => b.textContent)
      expect(ofrecidos).toHaveLength(7)
      expect(ofrecidos.join(' ')).not.toMatch(/CA-HUA-01|CA-SOC-01/)

      await userEvent.click(within(aviso).getByRole('button', { name: /CA-ESP-01/ }))

      expect(screen.getByLabelText('Estación y día')).toHaveValue('CA-ESP-01-2021-12-21')
      expect(await screen.findByText('Sistema limpio para este tramo')).toBeInTheDocument()
    })

    it('si se ensució después de abrir la pantalla, lo descubre al apretar y no empieza', async () => {
      const d = dia('CA-HUA-01-2021-12-20')
      const api = apiDePrueba()
      montar('usuario')
      await screen.findByLabelText('Estación y día')
      await elegirDia(d.id)
      expect(await screen.findByText('Sistema limpio para este tramo')).toBeInTheDocument()

      // Otra persona mandó lecturas de 2022 a esa estación mientras tanto.
      api.estado.set('CA-HUA-01', estadoDe('CA-HUA-01', '2022-03-05T10:00:00-05:00'))
      await userEvent.click(screen.getByRole('button', { name: 'Simular este día' }))

      expect(await screen.findByText('El sistema no está limpio para este tramo')).toBeInTheDocument()
      expect(api.lecturas).toHaveLength(0)
      expect(screen.getByRole('button', { name: 'Simular este día de todos modos' })).toBeInTheDocument()
    })

    it('el escenario sobre un sistema con un día ya simulado: dice qué estaciones no se van a mover', async () => {
      apiDePrueba({ estados: [estadoDe('CA-HUA-01', '2021-12-21T00:00:00-05:00'), estadoDe('CA-AYAS-01', '2021-12-14T12:00:00-05:00')] })
      montar('admin')

      const aviso = await screen.findByRole('alert')
      expect(within(aviso).getByText(/un estado posterior a todo el tramo/)).toBeInTheDocument()
      expect(within(aviso).getByText(/un estado de una hora del tramo: hasta esa hora no cambia/)).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Cargar el escenario de todos modos' })).toBeInTheDocument()
    })

    it('un tramo que ya se cargó entero no es una alarma: lo dice y deja repetirlo', async () => {
      const d = dia('CA-HUA-01-2021-12-20')
      apiDePrueba({ estados: [estadoDe('CA-HUA-01', d.esperado.fecha_hora)] })
      montar('usuario')
      await screen.findByLabelText('Estación y día')
      await elegirDia(d.id)

      expect(await screen.findByText('Este tramo ya está cargado')).toBeInTheDocument()
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Volver a simular' })).toBeInTheDocument()
    })
  })

  describe('una lectura a mano', () => {
    it('solo ofrece las 17 estaciones con PM por hora y la manda a la hora siguiente al último estado', async () => {
      const api = apiDePrueba({ estados: [estadoDe('CA-AYAS-01', '2021-12-16T00:00:00-05:00')] })
      montar('usuario')
      await userEvent.click(await screen.findByRole('button', { name: 'Una lectura a mano' }))

      const estaciones = within(screen.getByLabelText('Estación')).getAllByRole('option')
      expect(estaciones).toHaveLength(17)
      expect(estaciones.map((o) => o.textContent)).toContain('CA-RZC-02')
      await userEvent.selectOptions(screen.getByLabelText('Estación'), 'CA-AYAS-01')
      await userEvent.type(screen.getByLabelText('PM2.5 (µg/m³)'), '70.2')
      await userEvent.type(screen.getByLabelText('PM10 (µg/m³)'), '190.5')
      await userEvent.click(screen.getByRole('button', { name: 'Enviar la lectura' }))

      expect(await screen.findByText(/La API asignó la lectura al régimen/)).toBeInTheDocument()
      expect(api.lecturas).toEqual([{ estacion: 'CA-AYAS-01', fecha_hora: '2021-12-16T01:00:00-05:00', pm2_5: 70.2, pm10: 190.5 }])
    })

    it('a una estación sin estado no se le puede mandar: no hay una hora siguiente', async () => {
      const api = apiDePrueba()
      montar('usuario')
      await userEvent.click(await screen.findByRole('button', { name: 'Una lectura a mano' }))

      await userEvent.type(screen.getByLabelText('PM2.5 (µg/m³)'), '10')
      await userEvent.type(screen.getByLabelText('PM10 (µg/m³)'), '20')

      expect(await screen.findByText(/todavía no tiene estado: no hay una «hora siguiente»/)).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Enviar la lectura' })).toBeDisabled()
      expect(api.lecturas).toHaveLength(0)
    })

    it('si la API rechaza la lectura, muestra sus palabras', async () => {
      apiDePrueba({
        estados: [estadoDe('CA-AYAS-01', '2021-12-16T00:00:00-05:00')],
        alPredecir: () => respuesta(400, { error: 'pm10 no puede superar 10000 µg/m³' }),
      })
      montar('usuario')
      await userEvent.click(await screen.findByRole('button', { name: 'Una lectura a mano' }))
      await userEvent.selectOptions(screen.getByLabelText('Estación'), 'CA-AYAS-01')
      await userEvent.type(screen.getByLabelText('PM2.5 (µg/m³)'), '10')
      await userEvent.type(screen.getByLabelText('PM10 (µg/m³)'), '20000')
      await userEvent.click(screen.getByRole('button', { name: 'Enviar la lectura' }))

      const alerta = await screen.findByRole('alert')
      expect(within(alerta).getByText('pm10 no puede superar 10000 µg/m³')).toBeInTheDocument()
    })
  })
})
