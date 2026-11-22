// La pantalla de entrenamiento de punta a punta, con la API y el WebSocket
// falsos: el formulario y cada rechazo de la API, las rondas que se reproducen,
// el resultado con la compuerta del modelo, un aborto a mitad y el historial.

import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { crearClienteDeConsultas } from '../api/cliente'
import { Proveedores } from '../App'
import { fabricaDeEnchufes, simularApi } from '../pruebas/dobles'
import { abrirSesion, cerrarSesion, recargarSesion } from '../sesion/almacen'
import Entrenamiento from './Entrenamiento'

type Rutas = Parameters<typeof simularApi>[0]

const NODOS = [
  { nombre: 'nodo-a', direccion: '10.0.0.1:5000', workers: 4, latencia_ms: 0.2, libre: true },
  { nombre: 'nodo-b', direccion: '10.0.0.2:5000', workers: 8, latencia_ms: 0.3, libre: true },
]
const MODELO = { id: 'sembrado-kmeans-json', k: 3, features: ['pm2_5', 'pm10'], iteraciones: 44, inercia: 50950.4, regimenes: [] }
const ID = 'e-20261121-1530'

function ronda(iter: number, id = ID) {
  return { id, iter, inercia: 73000 - iter * 1000, ms_ronda: 0.4, ms_fusion: 0.004, ms_por_nodo: { 'nodo-a': 0.31, 'nodo-b': 0.38 } }
}

const FIN = { id: ID, iteraciones: 4, paro: 'tolerancia', dif_secuencial: 7.5e-15, ms_total: 56.4, modelo_id: `entrenado-${ID}` }

const DETALLE = {
  id: ID,
  estado: 'terminado',
  nodos: ['nodo-a', 'nodo-b'],
  replicas: 1,
  iteraciones: 4,
  paro: 'tolerancia',
  dif_secuencial: 7.5e-15,
  ms_total: 56.4,
  modelo_id: `entrenado-${ID}`,
  rondas: [1, 2, 3, 4].map((i) => ronda(i)),
}

/** La API de un sistema recién levantado, con dos nodos libres y sin historial; `rutas` pisa lo que haga falta. */
function api(rutas: Rutas = {}) {
  return simularApi({
    'GET /api/cluster/nodos': { estado: 200, cuerpo: NODOS },
    'GET /api/modelos/activo': { estado: 200, cuerpo: MODELO },
    'GET /api/entrenamientos': { estado: 200, cuerpo: [] },
    'POST /api/entrenamientos': { estado: 202, cuerpo: { id: ID, estado: 'en_curso' } },
    [`GET /api/entrenamientos/${ID}`]: { estado: 200, cuerpo: DETALLE },
    ...rutas,
  })
}

function montar() {
  const enchufes = fabricaDeEnchufes()
  // Sin reintentos: un error de la API se ve en el acto, como lo vería una persona tras esperarlos.
  const consultas = crearClienteDeConsultas()
  consultas.setDefaultOptions({ queries: { ...consultas.getDefaultOptions().queries, retry: false } })
  render(
    <Proveedores consultas={consultas} tiempoReal={{ crearEnchufe: enchufes.crearEnchufe }}>
      <Entrenamiento />
    </Proveedores>,
  )
  act(() => enchufes.ultimo().abrir())
  return {
    /** Llega un evento por el WebSocket. */
    emitir: (evento: string, datos: unknown) => act(() => enchufes.ultimo().recibir({ evento, datos })),
    cortar: () => act(() => enchufes.ultimo().cerrarDesdeElServidor(1006)),
  }
}

const formulario = () => within(screen.getByRole('form', { name: 'Nuevo entrenamiento' }))
const lanzar = () => userEvent.click(formulario().getByRole('button', { name: 'Lanzar entrenamiento' }))

async function escribir(campo: string, valor: string) {
  const entrada = formulario().getByLabelText(campo)
  await userEvent.clear(entrada)
  if (valor !== '') await userEvent.type(entrada, valor)
}

/** Los pedidos POST /api/entrenamientos que se hicieron, con su cuerpo. */
async function pedidosDeEntrenamiento(fetch: ReturnType<typeof simularApi>) {
  const pedidos = fetch.mock.calls.map((c) => c[0]).filter((p) => p.method === 'POST' && p.url.endsWith('/api/entrenamientos'))
  return Promise.all(pedidos.map((p) => p.clone().json()))
}

describe('la pantalla de entrenamiento', () => {
  beforeEach(() => {
    recargarSesion()
    abrirSesion({ usuario: 'ana', token: 'token-de-admin', rol: 'admin', expira: Date.now() + 3_600_000 })
  })
  afterEach(() => cerrarSesion('salir'))

  describe('el formulario', () => {
    it('muestra los nodos libres y propone usarlos todos', async () => {
      api()
      montar()

      const libres = await screen.findByRole('list', { name: 'Nodos libres' })
      expect(within(libres).getAllByRole('listitem').map((e) => e.textContent)).toEqual(['nodo-a4 workers', 'nodo-b8 workers'])
      expect(formulario().getByLabelText('Nodos')).toHaveValue('2')
      expect(formulario().getByLabelText('Réplicas')).toHaveValue('1')
      expect(formulario().getByLabelText('Semilla')).toHaveValue('42')
      expect(await formulario().findByText('sembrado-kmeans-json')).toBeInTheDocument()
    })

    it('manda nodos, réplicas y semilla como enteros', async () => {
      const fetch = api()
      montar()
      await screen.findByRole('list', { name: 'Nodos libres' })

      await escribir('Nodos', '1')
      await escribir('Semilla', '-7')
      await lanzar()

      await screen.findByText(/lanzado desde aquí/)
      expect(await pedidosDeEntrenamiento(fetch)).toEqual([{ nodos: 1, replicas: 1, semilla: -7 }])
    })

    it('con un campo inválido lo dice junto al campo y no le pide nada a la API', async () => {
      const fetch = api()
      montar()
      await screen.findByRole('list', { name: 'Nodos libres' })

      await escribir('Réplicas', '2.5')
      await escribir('Semilla', '')
      await lanzar()

      const replicas = formulario().getByLabelText('Réplicas')
      expect(replicas).toBeInvalid()
      expect(replicas).toHaveAccessibleDescription('Tiene que ser un número entero, sin decimales.')
      expect(formulario().getByLabelText('Semilla')).toHaveAccessibleDescription('Falta la semilla.')
      expect(await pedidosDeEntrenamiento(fetch)).toEqual([])

      // Al corregirlo, el error del campo se va.
      await escribir('Réplicas', '3')
      expect(formulario().getByLabelText('Réplicas')).toBeValid()
    })

    it('el preajuste «Medición larga» pone 300 réplicas y dice que no cambia el modelo activo', async () => {
      const fetch = api()
      montar()
      await screen.findByRole('list', { name: 'Nodos libres' })
      expect(formulario().getByRole('button', { name: 'Entrenar el modelo' })).toHaveAttribute('aria-pressed', 'true')

      await userEvent.click(formulario().getByRole('button', { name: 'Medición larga' }))

      expect(formulario().getByLabelText('Réplicas')).toHaveValue('300')
      expect(formulario().getByText(/300 réplicas es una medición/)).toBeInTheDocument()
      expect(formulario().getByText(/No cambia el modelo activo/)).toBeInTheDocument()

      await lanzar()
      await screen.findByText(/lanzado desde aquí/)
      expect(await pedidosDeEntrenamiento(fetch)).toEqual([{ nodos: 2, replicas: 300, semilla: 42 }])
    })

    it('si se piden más nodos que los libres avisa, pero deja que la API decida', async () => {
      api({ 'POST /api/entrenamientos': { estado: 422, cuerpo: { error: 'hay 2 nodos libres en el pool y se pidieron 4' } } })
      montar()
      await screen.findByRole('list', { name: 'Nodos libres' })

      await escribir('Nodos', '4')

      expect(formulario().getByText(/Hay 2 nodos libres y pides 4: la API lo va a rechazar/)).toBeInTheDocument()
      await lanzar()
      expect(await formulario().findByRole('alert')).toHaveTextContent('hay 2 nodos libres en el pool y se pidieron 4')
    })

    it.each([
      [
        '409: ya hay uno en curso',
        { estado: 409, cuerpo: { error: 'ya hay un entrenamiento en curso: e-20261121-1528' } },
        ['Ya hay un entrenamiento en curso', 'La API dice: «ya hay un entrenamiento en curso: e-20261121-1528»', 'La API entrena de a uno'],
      ],
      [
        '422: no hay nodos suficientes',
        { estado: 422, cuerpo: { error: 'hay 0 nodos libres en el pool y se pidieron 2' } },
        ['La API rechazó el pedido', 'La API dice: «hay 0 nodos libres en el pool y se pidieron 2»'],
      ],
      [
        '400: demasiadas réplicas',
        { estado: 400, cuerpo: { error: 'replicas 5000 es demasiado: el máximo es 379' } },
        ['La API rechazó el pedido', 'La API dice: «replicas 5000 es demasiado: el máximo es 379»'],
      ],
      [
        '503: la API no puede entrenar',
        { estado: 503, cuerpo: { error: 'esta API no puede entrenar: arrancó sin el gold (-gold)' } },
        ['La API no puede entrenar ahora', 'La API dice: «esta API no puede entrenar: arrancó sin el gold (-gold)»', 'No es un problema del formulario'],
      ],
      ['la API no responde', 'sin_respuesta' as const, ['La API no responde', 'Puede estar apagada o reiniciándose']],
      ['403: el rol no alcanza', { estado: 403, cuerpo: { error: 'se requiere el rol admin' } }, ['No tienes permiso para ver esto', 'se requiere el rol admin']],
    ])('un rechazo de la API se dice con claridad · %s', async (_caso, rechazo, textos) => {
      api({ 'POST /api/entrenamientos': rechazo })
      montar()
      await screen.findByRole('list', { name: 'Nodos libres' })

      await lanzar()

      const alerta = await formulario().findByRole('alert')
      for (const texto of textos) expect(alerta).toHaveTextContent(texto)
      // No quedó nada a medias: se puede corregir y volver a lanzar.
      expect(formulario().getByRole('button', { name: 'Lanzar entrenamiento' })).toBeEnabled()
      expect(screen.getByText('Todavía no hay un entrenamiento a la vista')).toBeInTheDocument()
    })

    it('el rechazo se borra al volver a lanzar y salir bien', async () => {
      let ocupada = true
      api({
        'POST /api/entrenamientos': () =>
          ocupada
            ? { estado: 409, cuerpo: { error: 'ya hay un entrenamiento en curso: e-1' } }
            : { estado: 202, cuerpo: { id: ID, estado: 'en_curso' } },
      })
      montar()
      await screen.findByRole('list', { name: 'Nodos libres' })
      await lanzar()
      await formulario().findByRole('alert')

      ocupada = false
      await lanzar()

      await screen.findByText(/lanzado desde aquí/)
      expect(formulario().queryByRole('alert')).not.toBeInTheDocument()
    })

    it('si no se puede consultar el pool lo dice y deja reintentar', async () => {
      let caida = true
      api({ 'GET /api/cluster/nodos': () => (caida ? { estado: 503, cuerpo: { error: 'no disponible' } } : { estado: 200, cuerpo: NODOS }) })
      montar()

      const alerta = await formulario().findByRole('alert')
      expect(alerta).toHaveTextContent('No se pudo consultar el pool de nodos')

      caida = false
      await userEvent.click(within(alerta).getByRole('button', { name: 'Reintentar' }))

      expect(await screen.findByRole('list', { name: 'Nodos libres' })).toBeInTheDocument()
    })

    it('sin ningún nodo conectado dice cómo levantarlos', async () => {
      api({ 'GET /api/cluster/nodos': { estado: 200, cuerpo: [] } })
      montar()

      expect(await formulario().findByText(/No hay ningún nodo conectado/)).toBeInTheDocument()
      expect(formulario().getByLabelText('Nodos')).toHaveValue('1')
    })
  })

  describe('las rondas', () => {
    it('llegan juntas y se reproducen de a una, diciendo que es una reproducción', async () => {
      api()
      const { emitir } = montar()
      await screen.findByRole('list', { name: 'Nodos libres' })
      await lanzar()
      await screen.findByText('Cargando datos en los nodos')

      // Las cuatro rondas y el fin, de un golpe: así llegan de un entrenamiento de 50 ms.
      emitir('entrenamiento.ronda', ronda(1))
      emitir('entrenamiento.ronda', ronda(2))
      emitir('entrenamiento.ronda', ronda(3))
      emitir('entrenamiento.ronda', ronda(4))
      emitir('entrenamiento.fin', FIN)

      // Recién llegadas, ninguna a la vista todavía, y el resultado tampoco.
      expect(screen.getByText('Reproduciendo · ronda 0 de 4')).toBeInTheDocument()
      expect(screen.queryByText('Resultado')).not.toBeInTheDocument()
      expect(screen.getByText(/Lo que ves es una reproducción: cada ronda se muestra 120 ms, unas 300 veces más de lo que tardó/)).toBeInTheDocument()
      expect(screen.getByText(/El entrenamiento real duró 56 ms/)).toBeInTheDocument()

      // Pasan de a una.
      expect(await screen.findByText('Reproduciendo · ronda 1 de 4')).toBeInTheDocument()
      expect(await screen.findByText('Reproduciendo · ronda 2 de 4')).toBeInTheDocument()
      expect(await screen.findByText('Reproduciendo · ronda 3 de 4')).toBeInTheDocument()
      expect(await screen.findByText('Terminado · 4 rondas')).toBeInTheDocument()
      expect(screen.getByText(/Lo que viste fue una reproducción/)).toBeInTheDocument()

      // Los dos gráficos dicen la última ronda.
      expect(screen.getByRole('img', { name: /Inercia por ronda: de 72,000\.0 en la ronda 1 a 69,000\.0 en la ronda 4/ })).toBeInTheDocument()
      expect(screen.getByRole('img', { name: /Tiempo de respuesta de cada nodo en la ronda 4: nodo-a, 0\.31 ms; nodo-b, 0\.38 ms/ })).toBeInTheDocument()
    })

    it('no pierde las rondas que llegan antes que el 202', async () => {
      let emitir: (evento: string, datos: unknown) => void = () => {}
      api({
        'POST /api/entrenamientos': () => {
          // El entrenamiento entero pasa antes de que la respuesta del POST llegue al navegador.
          emitir('entrenamiento.ronda', ronda(1))
          emitir('entrenamiento.ronda', ronda(2))
          emitir('entrenamiento.fin', { ...FIN, iteraciones: 2 })
          return { estado: 202, cuerpo: { id: ID, estado: 'en_curso' } }
        },
        [`GET /api/entrenamientos/${ID}`]: { estado: 200, cuerpo: { ...DETALLE, iteraciones: 2, rondas: [ronda(1), ronda(2)] } },
      })
      ;({ emitir } = montar())
      await screen.findByRole('list', { name: 'Nodos libres' })

      await lanzar()

      expect(await screen.findByText('Terminado · 2 rondas')).toBeInTheDocument()
      expect(screen.getByText(/lanzado desde aquí/)).toBeInTheDocument()
    })

    it('«Ir al final» salta la reproducción y «Repetir» la vuelve a empezar', async () => {
      api()
      const { emitir } = montar()
      await screen.findByRole('list', { name: 'Nodos libres' })
      await lanzar()
      await screen.findByText('Cargando datos en los nodos')
      for (let i = 1; i <= 4; i++) emitir('entrenamiento.ronda', ronda(i))
      emitir('entrenamiento.fin', FIN)

      await userEvent.click(screen.getByRole('button', { name: 'Ir al final' }))
      expect(screen.getByText('Terminado · 4 rondas')).toBeInTheDocument()

      await userEvent.click(screen.getByRole('button', { name: 'Lenta' }))
      await userEvent.click(screen.getByRole('button', { name: 'Repetir' }))
      expect(screen.getByText('Reproduciendo · ronda 0 de 4')).toBeInTheDocument()
      expect(screen.getByText(/cada ronda se muestra 300 ms/)).toBeInTheDocument()
    })

    it('mientras el entrenamiento corre no deja lanzar otro y explica por qué', async () => {
      api()
      const { emitir } = montar()
      await screen.findByRole('list', { name: 'Nodos libres' })
      await lanzar()
      await screen.findByText('Cargando datos en los nodos')

      const boton = formulario().getByRole('button', { name: 'Lanzar entrenamiento' })
      expect(boton).toBeDisabled()
      expect(formulario().getByRole('status')).toHaveTextContent(`Hay un entrenamiento en curso (${ID}). La API entrena de a uno`)

      // Una medición larga: las rondas llegan de a una y se ven en vivo.
      emitir('entrenamiento.ronda', { ...ronda(1), ms_ronda: 140 })
      expect(await screen.findByText('En vivo · ronda 1')).toBeInTheDocument()
      expect(screen.getByText(/Cada ronda está tardando 140 ms de verdad/)).toBeInTheDocument()
      expect(boton).toBeDisabled()
    })

    it('sigue un entrenamiento que lanzó otra sesión', async () => {
      api()
      const { emitir } = montar()
      await screen.findByRole('list', { name: 'Nodos libres' })

      emitir('entrenamiento.ronda', ronda(1, 'e-de-otro'))

      expect(await screen.findByText(/lanzado desde otra sesión/)).toBeInTheDocument()
      expect(formulario().getByRole('button', { name: 'Lanzar entrenamiento' })).toBeDisabled()
    })
  })

  describe('el resultado', () => {
    async function hastaElFinal(rutas: Rutas, fin: unknown, rondas = 4) {
      const fetch = api(rutas)
      const vista = montar()
      await screen.findByRole('list', { name: 'Nodos libres' })
      await lanzar()
      await screen.findByText('Cargando datos en los nodos')
      for (let i = 1; i <= rondas; i++) vista.emitir('entrenamiento.ronda', ronda(i))
      vista.emitir('entrenamiento.fin', fin)
      await userEvent.click(screen.getByRole('button', { name: 'Ir al final' }))
      return { ...vista, fetch }
    }

    it('al terminar pide el detalle y muestra iteraciones, paro, diferencia, tiempo y que el modelo quedó activo', async () => {
      const { fetch } = await hastaElFinal({ 'GET /api/modelos/activo': { estado: 200, cuerpo: { ...MODELO, id: `entrenado-${ID}` } } }, FIN)

      const resultado = within((await screen.findByText('Resultado')).closest('section')!)
      expect(resultado.getByText('Iteraciones').nextElementSibling).toHaveTextContent('4')
      expect(resultado.getByText('Tiempo total real').nextElementSibling).toHaveTextContent('56 ms')
      expect(resultado.getByText('Motivo de paro').nextElementSibling).toHaveTextContent('Convergió')
      expect(resultado.getByText('Dif. con el secuencial').nextElementSibling).toHaveTextContent('7.5 e-15')
      expect(resultado.getByText(/2 nodos:/)).toHaveTextContent('2 nodos: nodo-a, nodo-b · 1 réplica')
      expect(resultado.getByText(/El modelo nuevo quedó activo/)).toHaveTextContent(`entrenado-${ID} pasó la compuerta`)
      await waitFor(() => expect(resultado.queryByText('Pidiendo el detalle a la API…')).not.toBeInTheDocument())
      expect(fetch.mock.calls.some((c) => c[0].url.endsWith(`/api/entrenamientos/${ID}`))).toBe(true)
    })

    it('con réplicas dice que fue una medición y que el modelo activo no cambió', async () => {
      await hastaElFinal(
        { [`GET /api/entrenamientos/${ID}`]: { estado: 200, cuerpo: { ...DETALLE, replicas: 300, modelo_id: undefined } } },
        { ...FIN, modelo_id: undefined },
      )

      expect(await screen.findByText(/Fue una medición: el modelo activo no cambió/)).toHaveTextContent('Con 300 réplicas')
    })

    it('si el modelo no pasa la compuerta, dice que no se activó y por qué, con las palabras de la API', async () => {
      const aviso = 'el modelo no se activó y sigue el anterior: dif_secuencial 0.52 supera la tolerancia 1e-09'
      await hastaElFinal(
        { [`GET /api/entrenamientos/${ID}`]: { estado: 200, cuerpo: { ...DETALLE, modelo_id: undefined, aviso, dif_secuencial: 0.52 } } },
        { ...FIN, modelo_id: undefined, aviso, dif_secuencial: 0.52 },
      )

      expect(await screen.findByText(/El modelo no se activó/)).toHaveTextContent(`La API dice: «${aviso}».`)
    })

    it('si el detalle no se puede pedir, muestra los números del evento de fin y lo dice', async () => {
      await hastaElFinal({ [`GET /api/entrenamientos/${ID}`]: { estado: 503, cuerpo: { error: 'MongoDB no disponible' } } }, FIN)

      const resultado = within((await screen.findByText('Resultado')).closest('section')!)
      expect(resultado.getByText('Iteraciones').nextElementSibling).toHaveTextContent('4')
      expect(await resultado.findByText(/No se pudo pedir el detalle a la API/)).toBeInTheDocument()
    })

    it('un aborto a mitad: quedan las rondas que llegaron, el error de la API y el nodo que falló', async () => {
      const error = 'ronda 3 abortada: nodo "nodo-b": esperando PARCIAL: leer: connection reset by peer'
      api({
        [`GET /api/entrenamientos/${ID}`]: {
          estado: 200,
          cuerpo: { id: ID, estado: 'abortado', nodos: ['nodo-a', 'nodo-b'], replicas: 300, rondas: [ronda(1), ronda(2)], error },
        },
      })
      const { emitir } = montar()
      await screen.findByRole('list', { name: 'Nodos libres' })
      await lanzar()
      await screen.findByText('Cargando datos en los nodos')
      emitir('entrenamiento.ronda', ronda(1))
      emitir('entrenamiento.ronda', ronda(2))

      emitir('entrenamiento.fin', { id: ID, error })

      // Las dos rondas completas se terminan de mostrar antes de dar el resultado.
      expect(await screen.findByText('Abortado en la ronda 3')).toBeInTheDocument()
      expect(screen.getByText(/se muestran las 2 rondas que llegaron a completarse/)).toBeInTheDocument()
      const resultado = within(screen.getByText('El entrenamiento abortó').closest('section')!)
      expect(resultado.getByText(/Falló el nodo nodo-b\./)).toHaveTextContent('No se activó ningún modelo: sigue el anterior.')
      expect(resultado.getByText(error)).toBeInTheDocument()
      expect(resultado.getByText('Llegó a completar 2 rondas.')).toBeInTheDocument()
      expect(screen.getByRole('img', { name: /Inercia por ronda: .* en la ronda 2\./ })).toBeInTheDocument()
    })

    it('un aborto antes de la primera ronda no deja gráficos vacíos', async () => {
      const error = 'la carga falló: el nodo nodo-a no tiene el gold'
      api({
        [`GET /api/entrenamientos/${ID}`]: {
          estado: 200,
          cuerpo: { id: ID, estado: 'abortado', nodos: ['nodo-a', 'nodo-b'], replicas: 1, rondas: [], error },
        },
      })
      const { emitir } = montar()
      await screen.findByRole('list', { name: 'Nodos libres' })
      await lanzar()
      await screen.findByText('Cargando datos en los nodos')

      emitir('entrenamiento.fin', { id: ID, error })

      expect(await screen.findByText('Abortado antes de la primera ronda')).toBeInTheDocument()
      expect(await screen.findByText(/Falló el nodo nodo-a\./)).toBeInTheDocument()
      expect(screen.getByText('No llegó a completar ninguna ronda.')).toBeInTheDocument()
      expect(screen.queryByRole('img', { name: /Inercia/ })).not.toBeInTheDocument()
    })

    it('tras terminar espera a que los nodos vuelvan al pool, y lo explica en vez de mostrar «0 nodos»', async () => {
      let nodos: unknown[] = NODOS
      api({ 'GET /api/cluster/nodos': () => ({ estado: 200, cuerpo: nodos }) })
      const { emitir } = montar()
      await screen.findByRole('list', { name: 'Nodos libres' })
      await lanzar()
      await screen.findByText('Cargando datos en los nodos')
      emitir('entrenamiento.ronda', ronda(1))

      // Al cerrar la corrida los nodos dejan el pool: la API responde una lista vacía.
      nodos = []
      emitir('entrenamiento.fin', { ...FIN, iteraciones: 1 })

      const boton = formulario().getByRole('button', { name: 'Lanzar entrenamiento' })
      await waitFor(() => expect(formulario().getByRole('status')).toHaveTextContent(/Los nodos están volviendo al pool.*Volvieron 0 de 2/))
      expect(boton).toBeDisabled()
      expect(formulario().getByText('volviendo al pool…')).toBeInTheDocument()
      expect(formulario().queryByText(/No hay ningún nodo conectado/)).not.toBeInTheDocument()

      // Unos 2 s después vuelven a llamar a la API.
      nodos = NODOS
      await waitFor(() => expect(boton).toBeEnabled(), { timeout: 3000 })
      expect(screen.getByRole('list', { name: 'Nodos libres' })).toBeInTheDocument()
    })

    it('con el tiempo real caído lo avisa y se entera del fin pidiendo el detalle', async () => {
      api()
      const { cortar } = montar()
      await screen.findByRole('list', { name: 'Nodos libres' })
      cortar()
      expect(await formulario().findByText(/El tiempo real está desconectado/)).toBeInTheDocument()

      await lanzar()

      // Sin eventos: las rondas y el fin salen de GET /api/entrenamientos/{id}.
      expect(await screen.findByText('Terminado · 4 rondas', {}, { timeout: 3000 })).toBeInTheDocument()
      expect(await screen.findByText(/El modelo nuevo quedó activo|Activó su modelo/)).toBeInTheDocument()
    })
  })

  describe('el historial', () => {
    const HISTORIAL = [
      { id: 'e-3', estado: 'abortado', nodos: ['nodo-a', 'nodo-b'], replicas: 1100, error: 'ronda 25 abortada: nodo "nodo-b": no respondió' },
      { id: 'e-2', estado: 'terminado', nodos: ['nodo-a', 'nodo-b'], replicas: 300, iteraciones: 44, paro: 'tolerancia', dif_secuencial: 1.1e-14, ms_total: 1887.2 },
      {
        id: 'e-1',
        estado: 'terminado',
        nodos: ['nodo-a'],
        replicas: 1,
        iteraciones: 44,
        paro: 'max_iter',
        dif_secuencial: 7.5e-15,
        ms_total: 56.4,
        modelo_id: 'entrenado-e-1',
      },
    ]

    it('mientras carga lo dice', async () => {
      vi.stubGlobal('fetch', () => new Promise<Response>(() => {}))
      montar()

      expect(await screen.findByText('Cargando el historial…')).toBeInTheDocument()
      expect(screen.getByRole('heading', { name: 'Entrenamiento' })).toBeInTheDocument()
    })

    it('vacío: dice que todavía no se lanzó ninguno', async () => {
      api()
      montar()

      expect(await screen.findByText('Todavía no se lanzó ningún entrenamiento')).toBeInTheDocument()
    })

    it('si la API falla lo dice y deja reintentar', async () => {
      let caida = true
      api({
        'GET /api/entrenamientos': () => (caida ? { estado: 503, cuerpo: { error: 'MongoDB no disponible' } } : { estado: 200, cuerpo: HISTORIAL }),
      })
      montar()

      const error = (await screen.findByText('El servicio no está disponible')).closest<HTMLElement>('[data-estado]')!
      expect(error).toHaveTextContent('MongoDB no disponible')

      caida = false
      await userEvent.click(within(error).getByRole('button', { name: 'Reintentar' }))

      expect(await screen.findByRole('table', { name: 'Historial de entrenamientos' })).toBeInTheDocument()
    })

    it('lista cada entrenamiento con su estado y lo que pasó con su modelo', async () => {
      api({
        'GET /api/entrenamientos': { estado: 200, cuerpo: HISTORIAL },
        'GET /api/modelos/activo': { estado: 200, cuerpo: { ...MODELO, id: 'entrenado-e-1' } },
      })
      montar()

      const tabla = await screen.findByRole('table', { name: 'Historial de entrenamientos' })
      const filas = within(tabla)
        .getAllByRole('row')
        .slice(1)
        .map((f) => within(f).getAllByRole('cell').map((c) => c.textContent))
      expect(filas[0]!.slice(0, 4)).toEqual(['e-3', 'Abortado', '2', '1,100'])
      expect(filas[0]![6]).toBe('ronda 25 abortada: nodo "nodo-b": no respondió')
      expect(filas[1]).toEqual(['e-2', 'Terminado', '2', '300', '44', '1.9 s', 'Medición: no cambia el modelo', 'Ver rondas'])
      expect(filas[2]!.slice(4)).toEqual(['44', '56 ms', 'Es el modelo activo', 'Ver rondas'])
    })

    it('abrir uno muestra sus rondas completas y su resultado, sin volver a reproducirlas', async () => {
      api({
        'GET /api/entrenamientos': { estado: 200, cuerpo: HISTORIAL },
        'GET /api/entrenamientos/e-2': {
          estado: 200,
          cuerpo: { ...HISTORIAL[1], rondas: [1, 2, 3].map((i) => ronda(i, 'e-2')) },
        },
      })
      montar()
      await screen.findByRole('table', { name: 'Historial de entrenamientos' })

      await userEvent.click(screen.getByRole('button', { name: 'Ver las rondas de e-2' }))

      expect(await screen.findByText('Terminado · 3 rondas')).toBeInTheDocument()
      expect(screen.getByText(/del historial/)).toBeInTheDocument()
      expect(screen.getByText(/Fue una medición: el modelo activo no cambió/)).toBeInTheDocument()
      // La fila abierta ya no ofrece abrirse.
      expect(screen.queryByRole('button', { name: 'Ver las rondas de e-2' })).not.toBeInTheDocument()
      // Y se puede volver a ver cómo fue.
      await userEvent.click(screen.getByRole('button', { name: 'Repetir' }))
      expect(screen.getByText('Reproduciendo · ronda 0 de 3')).toBeInTheDocument()
    })

    it('si no se puede abrir uno, lo dice', async () => {
      api({ 'GET /api/entrenamientos': { estado: 200, cuerpo: HISTORIAL } })
      montar()
      await screen.findByRole('table', { name: 'Historial de entrenamientos' })

      await userEvent.click(screen.getByRole('button', { name: 'Ver las rondas de e-3' }))

      expect(await screen.findByText('No se encontró lo que se pidió')).toBeInTheDocument()
    })

    it('con muchos muestra los 10 más recientes y deja ver todos', async () => {
      const muchos = Array.from({ length: 13 }, (_, i) => ({ ...HISTORIAL[1]!, id: `e-${String(13 - i).padStart(2, '0')}` }))
      api({ 'GET /api/entrenamientos': { estado: 200, cuerpo: muchos } })
      montar()

      const tabla = await screen.findByRole('table', { name: 'Historial de entrenamientos' })
      expect(within(tabla).getAllByRole('row')).toHaveLength(11) // el encabezado y diez
      expect(screen.getByText(/Se muestran los 10 más recientes de 13/)).toBeInTheDocument()

      await userEvent.click(screen.getByRole('button', { name: 'Ver todos' }))

      expect(within(tabla).getAllByRole('row')).toHaveLength(14)
    })

    it('al terminar un entrenamiento, el historial se vuelve a pedir', async () => {
      let historial: unknown[] = []
      api({ 'GET /api/entrenamientos': () => ({ estado: 200, cuerpo: historial }) })
      const { emitir } = montar()
      await screen.findByText('Todavía no se lanzó ningún entrenamiento')

      historial = [HISTORIAL[2]!]
      emitir('entrenamiento.fin', { ...FIN, id: 'e-1' })

      expect(await screen.findByRole('table', { name: 'Historial de entrenamientos' })).toHaveTextContent('e-1')
    })
  })
})
