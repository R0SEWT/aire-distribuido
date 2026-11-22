// La pantalla del clúster con la API falsa: los nodos, las cuatro métricas con
// su nombre verdadero (el CPU es el del proceso de la API), los eventos y los
// estados de toda pantalla.

import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { crearClienteDeConsultas } from '../api/cliente'
import { Proveedores } from '../App'
import { fabricaDeEnchufes, SALUD_OK, simularApi } from '../pruebas/dobles'
import { abrirSesion, cerrarSesion, recargarSesion } from '../sesion/almacen'
import Cluster from './Cluster'

type Rutas = Parameters<typeof simularApi>[0]

const NODOS = [
  { nombre: 'nodo-a', direccion: '10.0.0.1:5000', workers: 4, latencia_ms: 0.19, libre: true },
  { nombre: 'nodo-b', direccion: '10.0.0.2:5000', workers: 8, latencia_ms: 0.28, libre: false },
]

const PROCESO = { cpu_usuario_s: 12.0, cpu_sistema_s: 3.0, en_marcha_s: 1800, nucleos: 8, heap_mb: 21.7, sistema_mb: 48.3, max_rss_mb: 63.9, goroutines: 27 }

const METRICAS = {
  endpoints: {
    'POST /api/predicciones': { pedidos: 12000, p50_ms: 0.3, p95_ms: 0.9, p99_ms: 2.1 },
    'GET /api/regimenes': { pedidos: 40, p50_ms: 0.2, p95_ms: 0.5, p99_ms: 0.6 },
  },
  nodos_en_pool: 2,
  ultimo_entrenamiento: 'e-20261119-1530',
  escrituras_descartadas: 0,
  escrituras_fallidas: 0,
  escritura: {
    mongo: { escritas: 12000, descartadas: 0, fallidas: 0, lotes: 3120 },
    redis: { escritas: 12000, descartadas: 0, fallidas: 0, lotes: 3050 },
  },
  puntos_de_control: { escritos: 44, fallidos: 0, omitidos: 0 },
  proceso_api: PROCESO,
}

const EVENTOS = [
  { ts: '2026-11-19T15:30:04.512-05:00', tipo: 'alta', nodo: 'nodo-a', detalle: '10.0.0.1:5000: 4 workers, latencia 0.49 ms' },
  { ts: '2026-11-19T15:30:02.118-05:00', tipo: 'baja', nodo: 'nodo-a', detalle: '10.0.0.1:4990: entrenamiento e-20261119-1530 terminado' },
  { ts: '2026-11-19T15:29:40.000-05:00', tipo: 'error', nodo: 'nodo-b', detalle: 'hizo abortar el entrenamiento e-20261119-1529' },
]

function api(rutas: Rutas = {}) {
  return simularApi({
    'GET /api/salud': SALUD_OK,
    'GET /api/cluster/nodos': { estado: 200, cuerpo: NODOS },
    'GET /api/cluster/eventos': { estado: 200, cuerpo: EVENTOS },
    'GET /api/metricas': { estado: 200, cuerpo: METRICAS },
    ...rutas,
  })
}

function montar() {
  const enchufes = fabricaDeEnchufes()
  const consultas = crearClienteDeConsultas()
  consultas.setDefaultOptions({ queries: { ...consultas.getDefaultOptions().queries, retry: false } })
  render(
    <Proveedores consultas={consultas} tiempoReal={{ crearEnchufe: enchufes.crearEnchufe }}>
      <Cluster />
    </Proveedores>,
  )
  act(() => enchufes.ultimo().abrir())
  return { emitir: (evento: string, datos: unknown) => act(() => enchufes.ultimo().recibir({ evento, datos })) }
}

/** La tarjeta (section) que lleva ese título. */
function tarjeta(titulo: string | RegExp) {
  return within(screen.getByRole('heading', { name: titulo }).closest('section')!)
}

/** El texto de la cifra que lleva ese rótulo, con su unidad. */
function cifra(rotulo: string) {
  const resumen = document.querySelector<HTMLElement>('[data-resumen]')!
  return within(resumen).getByText(rotulo).nextElementSibling?.textContent
}

describe('la pantalla del clúster', () => {
  beforeEach(() => {
    recargarSesion()
    abrirSesion({ usuario: 'ana', token: 'token-de-admin', rol: 'admin', expira: Date.now() + 3_600_000 })
  })
  afterEach(() => cerrarSesion('salir'))

  describe('los estados', () => {
    it('mientras carga lo dice en cada tarjeta, con el título ya a la vista', async () => {
      vi.stubGlobal('fetch', () => new Promise<Response>(() => {}))
      montar()

      expect(screen.getByRole('heading', { name: 'Clúster' })).toBeInTheDocument()
      expect(await screen.findAllByText('Cargando…')).toHaveLength(5)
      expect(cifra('Nodos en el pool')).toBe('—consultando…')
      expect(screen.getByText('Consultando /api/salud…')).toBeInTheDocument()
    })

    it('si la API falla lo dice en la tarjeta y deja reintentar', async () => {
      let caida = true
      api({ 'GET /api/metricas': () => (caida ? { estado: 503, cuerpo: { error: 'la API se está apagando' } } : { estado: 200, cuerpo: METRICAS }) })
      montar()

      const error = (await tarjeta('Uso de CPU y memoria').findByText('El servicio no está disponible')).closest<HTMLElement>('[data-estado]')!
      expect(error).toHaveTextContent('la API se está apagando')
      // Lo que sí respondió se muestra igual.
      expect(await tarjeta('Nodos del pool').findByRole('table')).toBeInTheDocument()

      caida = false
      await userEvent.click(within(error).getByRole('button', { name: 'Reintentar' }))

      expect(await tarjeta('Uso de CPU y memoria').findByText(/Promedio desde que arrancó la API/)).toBeInTheDocument()
    })

    it('sin nodos explica por qué puede ser y cómo levantarlos', async () => {
      api({ 'GET /api/cluster/nodos': { estado: 200, cuerpo: [] } })
      montar()

      const vacio = await tarjeta('Nodos del pool').findByText('No hay nodos en el pool')
      expect(vacio.closest('[data-estado]')).toHaveTextContent(/Justo después de un entrenamiento es normal/)
      expect(cifra('Nodos en el pool')).toBe('0· 0 libres')
    })

    it('sin eventos lo dice', async () => {
      api({ 'GET /api/cluster/eventos': { estado: 200, cuerpo: [] } })
      montar()

      expect(await screen.findByText('Todavía no hay eventos')).toBeInTheDocument()
    })
  })

  describe('los nodos', () => {
    it('lista nombre, dirección, workers, latido y si está libre', async () => {
      api()
      montar()

      const tabla = await tarjeta('Nodos del pool').findByRole('table', { name: 'Nodos del pool' })
      const filas = within(tabla)
        .getAllByRole('row')
        .slice(1)
        .map((f) => within(f).getAllByRole('cell').map((c) => c.textContent))
      expect(filas).toEqual([
        ['nodo-a10.0.0.1:5000', '4', '0.19', '—', 'Libre'],
        ['nodo-b10.0.0.2:5000', '8', '0.28', '—', 'Entrenando'],
      ])
      expect(cifra('Nodos en el pool')).toBe('2· 1 libre')
      // El contrato del doble no tiene el último entrenamiento: la nota lo dice sin inventar tiempos.
      expect(document.querySelector('[data-nota-nodos]')).toHaveTextContent('su CPU no viaja por el protocolo')
    })

    it('muestra lo que tardó cada nodo por ronda en el último entrenamiento, que es lo que se mide de su carga', async () => {
      const ronda = (iter: number, a: number, b: number) => ({ iter, inercia: 1, ms_ronda: 1, ms_fusion: 0, ms_por_nodo: { 'nodo-a': a, 'nodo-b': b } })
      api({
        'GET /api/entrenamientos/e-20261119-1530': {
          estado: 200,
          cuerpo: {
            id: 'e-20261119-1530',
            estado: 'terminado',
            nodos: ['nodo-a', 'nodo-b'],
            replicas: 300,
            iteraciones: 2,
            paro: 'tolerancia',
            dif_secuencial: 0,
            ms_total: 90,
            rondas: [ronda(1, 30, 50), ronda(2, 40, 70)],
          },
        },
      })
      montar()

      const tabla = await tarjeta('Nodos del pool').findByRole('table', { name: 'Nodos del pool' })
      await waitFor(() =>
        expect(
          within(tabla)
            .getAllByRole('row')
            .slice(1)
            .map((f) => within(f).getAllByRole('cell')[3]!.textContent),
        ).toEqual(['35', '60']),
      )
      expect(document.querySelector('[data-nota-nodos]')).toHaveTextContent(
        'en el último entrenamiento (e-20261119-1530, 300 réplicas, 2 rondas), medido desde la API y con la red incluida',
      )
    })
  })

  describe('el uso de CPU', () => {
    it('lleva el nombre de lo que es: el proceso de la API, no los nodos', async () => {
      api()
      montar()

      const proceso = tarjeta('Uso de CPU y memoria')
      expect(proceso.getByText('Proceso de la API (el coordinador), no los nodos')).toBeInTheDocument()
      expect(await proceso.findByText(/Es el CPU del proceso de la API, que coordina el clúster; no el de los nodos/)).toHaveTextContent(
        'El protocolo del clúster no transporta el CPU de cada nodo',
      )
      expect(document.querySelector('[data-resumen]')).toHaveTextContent('CPU del proceso de la API')
    })

    it('con una sola consulta muestra el promedio desde el arranque y lo dice', async () => {
      api()
      montar()

      const proceso = tarjeta('Uso de CPU y memoria')
      expect(await proceso.findByText(/Promedio desde que arrancó la API/)).toBeInTheDocument()
      // 15 s de CPU en 1800 s en marcha: 0.8 % de un núcleo.
      expect(cifra('CPU del proceso de la API')).toBe('0.8% de un núcleo')
      expect(proceso.getByText(/el tope son 800 % \(8\s+núcleos\)/)).toBeInTheDocument()
    })

    it('con dos consultas calcula el uso entre las dos: Δ(usuario + sistema) / Δen_marcha', async () => {
      let proceso = PROCESO
      api({ 'GET /api/metricas': () => ({ estado: 200, cuerpo: { ...METRICAS, proceso_api: proceso } }) })
      const { emitir } = montar()
      await tarjeta('Uso de CPU y memoria').findByText(/Promedio desde que arrancó la API/)

      // Dos segundos después la API gastó 0.5 s de CPU (0.4 de usuario y 0.1 de sistema): un cuarto de núcleo.
      proceso = { ...PROCESO, cpu_usuario_s: 12.4, cpu_sistema_s: 3.1, en_marcha_s: 1802 }
      emitir('entrenamiento.fin', { id: 'e-1', error: 'x' }) // la pantalla refresca al terminar un entrenamiento

      expect(await tarjeta('Uso de CPU y memoria').findByText(/Entre las dos últimas consultas \(2\.0 s\)/)).toBeInTheDocument()
      expect(cifra('CPU del proceso de la API')).toBe('25.0% de un núcleo')
      expect(screen.getByRole('img', { name: /Uso de CPU del proceso de la API .* ahora, 25\.0 % de un núcleo/ })).toBeInTheDocument()
    })

    it('si la API se reinició entre dos consultas, no inventa un uso: vuelve al promedio', async () => {
      let proceso = PROCESO
      api({ 'GET /api/metricas': () => ({ estado: 200, cuerpo: { ...METRICAS, proceso_api: proceso } }) })
      const { emitir } = montar()
      await tarjeta('Uso de CPU y memoria').findByText(/Promedio desde que arrancó la API/)

      proceso = { ...PROCESO, cpu_usuario_s: 0.05, cpu_sistema_s: 0.05, en_marcha_s: 4 }
      emitir('entrenamiento.fin', { id: 'e-1', error: 'x' })

      await waitFor(() => expect(cifra('CPU del proceso de la API')).toBe('2.5% de un núcleo'))
      expect(tarjeta('Uso de CPU y memoria').getByText(/Promedio desde que arrancó la API/)).toBeInTheDocument()
    })

    it('donde no se mide el CPU (Windows) lo dice en vez de mostrar 0 %', async () => {
      api({
        'GET /api/metricas': {
          estado: 200,
          cuerpo: {
            ...METRICAS,
            proceso_api: { ...PROCESO, cpu_usuario_s: 0, cpu_sistema_s: 0, max_rss_mb: 0, no_disponible: ['cpu_usuario_s', 'cpu_sistema_s', 'max_rss_mb'] },
          },
        },
      })
      montar()

      expect(await tarjeta('Uso de CPU y memoria').findByText('esta plataforma no mide el tiempo de CPU')).toBeInTheDocument()
      expect(cifra('CPU del proceso de la API')).toBe('—no se mide aquí')
    })

    it('muestra la memoria del proceso', async () => {
      api()
      montar()

      const proceso = tarjeta('Uso de CPU y memoria')
      expect((await proceso.findByText('Heap vivo')).nextElementSibling).toHaveTextContent('21.7MB')
      expect(proceso.getByText('Máx. residente').nextElementSibling).toHaveTextContent('63.9MB')
      expect(proceso.getByText('Goroutines').nextElementSibling).toHaveTextContent('27')
    })
  })

  describe('la latencia', () => {
    it('lista cada ruta con pedidos, p50, p95 y p99, de la más lenta a la más rápida', async () => {
      api()
      montar()

      const tabla = await tarjeta('Latencia de la API por ruta').findByRole('table', { name: 'Latencia de la API por ruta' })
      const filas = within(tabla)
        .getAllByRole('row')
        .slice(1)
        .map((f) => within(f).getAllByRole('cell').slice(0, 5).map((c) => c.textContent))
      expect(filas).toEqual([
        ['POST /api/predicciones', '12,000', '0.30', '0.90', '2.1'],
        ['GET /api/regimenes', '40', '0.20', '0.50', '0.60'],
      ])
      expect(tarjeta('Latencia de la API por ruta').getByText(/El 99 % de las 12,000 predicciones respondió en 2\.1 ms o menos: dentro del objetivo de 100 ms/)).toBeInTheDocument()
      expect(within(tabla).getByRole('img', { name: /la línea punteada marca los 100 ms/ })).toBeInTheDocument()
      expect(cifra('Latencia de la API · peor p99')).toBe('2.10ms')
    })

    it('marca con palabras la ruta cuyo p99 llega a los 100 ms', async () => {
      api({
        'GET /api/metricas': {
          estado: 200,
          cuerpo: { ...METRICAS, endpoints: { ...METRICAS.endpoints, 'POST /api/predicciones': { pedidos: 500, p50_ms: 3, p95_ms: 80, p99_ms: 140 } } },
        },
      })
      montar()

      const latencia = tarjeta('Latencia de la API por ruta')
      expect(await latencia.findByText('p99 sobre 100 ms')).toBeInTheDocument()
      expect(latencia.getByText(/fuera del objetivo de 100 ms/)).toBeInTheDocument()
      expect(latencia.getByRole('img', { name: /p99 140 ms: p99 sobre los 100 ms/ })).toBeInTheDocument()
    })
  })

  describe('las escrituras', () => {
    it('por almacén: escritas, descartadas, fallidas y lotes, y los puntos de control', async () => {
      api()
      montar()

      const escrituras = tarjeta('Escrituras')
      const tabla = await escrituras.findByRole('table')
      const filas = within(tabla)
        .getAllByRole('row')
        .slice(1)
        .map((f) => within(f).getAllByRole('cell').map((c) => c.textContent))
      expect(filas).toEqual([
        ['MongoDB', '12,000', '0', '0', '3,120'],
        ['Redis', '12,000', '0', '0', '3,050'],
      ])
      expect(escrituras.getByText(/No se perdió ninguna escritura/)).toBeInTheDocument()
      expect(escrituras.getByText(/Puntos de control de los entrenamientos/)).toHaveTextContent('44 escritos, 0 fallidos y 0 omitidos')
      expect(cifra('Escrituras perdidas')).toBe('0de 24,000')
    })

    it('si se perdieron escrituras lo dice con cuántas y por qué', async () => {
      api({
        'GET /api/metricas': {
          estado: 200,
          cuerpo: {
            ...METRICAS,
            escrituras_descartadas: 120,
            escrituras_fallidas: 8,
            escritura: {
              mongo: { escritas: 11872, descartadas: 120, fallidas: 8, lotes: 3000 },
              redis: { escritas: 12000, descartadas: 0, fallidas: 0, lotes: 3050 },
            },
          },
        },
      })
      montar()

      expect(await tarjeta('Escrituras').findByText(/Se perdieron 128 escrituras: 120 descartadas por una cola llena y 8 que el almacén rechazó/)).toBeInTheDocument()
      expect(cifra('Escrituras perdidas')).toBe('128de 24,000')
    })
  })

  describe('los eventos del clúster', () => {
    it('del más reciente al más antiguo, con su tipo en palabras, el nodo y el detalle', async () => {
      api()
      montar()

      const lista = await screen.findByRole('list', { name: 'Eventos del clúster' })
      const eventos = within(lista).getAllByRole('listitem')
      expect(eventos.map((e) => e.dataset.evento)).toEqual(['alta', 'baja', 'error'])
      expect(eventos[0]).toHaveTextContent(/19 nov.*15:30:04.*Alta.*nodo-a.*4 workers, latencia 0\.49 ms/)
      expect(eventos[1]).toHaveTextContent(/15:30:02.*Baja.*entrenamiento e-20261119-1530 terminado/)
      expect(eventos[2]).toHaveTextContent(/Error.*nodo-b.*hizo abortar el entrenamiento/)
      expect(screen.getByText(/Después de cada entrenamiento es normal ver una baja y un alta por nodo/)).toBeInTheDocument()
    })

    it('pide 50 por omisión y 200 si se elige', async () => {
      const fetch = api()
      montar()
      await screen.findByRole('list', { name: 'Eventos del clúster' })
      const pedidos = () => fetch.mock.calls.map((c) => new URL(c[0].url)).filter((u) => u.pathname === '/api/cluster/eventos')
      expect(pedidos().map((u) => u.searchParams.get('limite'))).toEqual(['50'])

      await userEvent.click(screen.getByRole('button', { name: '200' }))

      await waitFor(() => expect(pedidos().at(-1)?.searchParams.get('limite')).toBe('200'))
    })

    it('al terminar un entrenamiento se vuelven a pedir, sin esperar al refresco', async () => {
      let eventos = EVENTOS.slice(1)
      api({ 'GET /api/cluster/eventos': () => ({ estado: 200, cuerpo: eventos }) })
      const { emitir } = montar()
      const lista = await screen.findByRole('list', { name: 'Eventos del clúster' })
      expect(within(lista).getAllByRole('listitem')).toHaveLength(2)

      eventos = EVENTOS
      emitir('entrenamiento.fin', { id: 'e-1', error: 'x' })

      await waitFor(() => expect(within(lista).getAllByRole('listitem')).toHaveLength(3))
    })
  })

  describe('la salud', () => {
    it('dice cómo está cada componente, con su palabra', async () => {
      api({ 'GET /api/salud': { estado: 200, cuerpo: { ...SALUD_OK.cuerpo, redis: 'error' } } })
      montar()

      const salud = tarjeta('Salud de cada componente')
      await waitFor(() => expect(salud.getByText('Redis').nextElementSibling).toHaveTextContent('No responde'))
      expect(salud.getByText('API').nextElementSibling).toHaveTextContent('Responde')
      expect(salud.getByText('MongoDB').nextElementSibling).toHaveTextContent('Responde')
      expect(salud.getByText('Modelo activo').nextElementSibling).toHaveTextContent('sembrado-kmeans-json')
    })

    it('si /api/salud no responde, dice que la API no responde y que de lo demás no hay dato', async () => {
      api({ 'GET /api/salud': 'sin_respuesta' })
      montar()

      const salud = tarjeta('Salud de cada componente')
      await waitFor(() => expect(salud.getByText('API').nextElementSibling).toHaveTextContent('No responde'))
      expect(salud.getByText('Redis').nextElementSibling).toHaveTextContent('Sin dato')
    })
  })
})
