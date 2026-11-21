// La sesión de punta a punta dentro de la app: el ingreso, el menú según el
// rol, las guardas, el 401 y el vencimiento.

import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useQuery } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api, crearClienteDeConsultas, pedir } from './api/cliente'
import { useEvento } from './api/tiempoReal'
import { Proveedores, Rutas } from './App'
import { fabricaDeEnchufes, SALUD_OK, simularApi, tokenDe } from './pruebas/dobles'
import { abrirSesion, cerrarSesion, recargarSesion, sesionActual, type Rol } from './sesion/almacen'

function montar(ruta = '/', extra?: React.ReactNode) {
  const enchufes = fabricaDeEnchufes()
  const vista = render(
    <Proveedores consultas={crearClienteDeConsultas()} tiempoReal={{ crearEnchufe: enchufes.crearEnchufe }}>
      <MemoryRouter initialEntries={[ruta]}>
        <Rutas />
        {extra}
      </MemoryRouter>
    </Proveedores>,
  )
  return { ...vista, enchufes }
}

function sesionDe(rol: Rol, enMs = 3_600_000) {
  abrirSesion({ usuario: rol === 'admin' ? 'ana' : 'demo', token: `token-de-${rol}`, rol, expira: Date.now() + enMs })
}

async function ingresarComo(usuario: string, clave: string) {
  await userEvent.type(screen.getByLabelText('Usuario'), usuario)
  await userEvent.type(screen.getByLabelText('Clave'), clave)
  await userEvent.click(screen.getByRole('button', { name: 'Ingresar' }))
}

function menu() {
  return within(screen.getByRole('navigation', { name: 'Pantallas' }))
    .getAllByRole('link')
    .map((e) => e.textContent)
}

describe('la app', () => {
  beforeEach(() => recargarSesion())
  afterEach(() => {
    cerrarSesion('salir')
    vi.useRealTimers()
  })

  describe('sin sesión', () => {
    it('cualquier ruta lleva al ingreso', async () => {
      simularApi({ 'GET /api/salud': SALUD_OK })

      montar('/cluster')

      expect(await screen.findByRole('form', { name: 'Ingreso' })).toBeInTheDocument()
      expect(screen.queryByRole('navigation', { name: 'Pantallas' })).not.toBeInTheDocument()
    })

    it('el ingreso muestra el estado de la API con su palabra', async () => {
      simularApi({ 'GET /api/salud': SALUD_OK })

      montar('/ingreso')

      expect(await screen.findByText('API en línea')).toBeInTheDocument()
    })

    it('si la API no responde, el punto de estado lo dice', async () => {
      simularApi({ 'GET /api/salud': 'sin_respuesta' })

      montar('/ingreso')

      expect(await screen.findByText('API sin respuesta')).toBeInTheDocument()
    })

    it('si la API responde pero un almacén no, el estado es «con problemas»', async () => {
      simularApi({
        'GET /api/salud': { estado: 200, cuerpo: { ...SALUD_OK.cuerpo, mongo: 'error' } },
      })

      montar('/ingreso')

      expect(await screen.findByText('API con problemas')).toBeInTheDocument()
      expect(screen.getByText(/MongoDB no responde/)).toBeInTheDocument()
    })
  })

  describe('el ingreso', () => {
    it('con un usuario común entra al panel y el menú no tiene lo de admin', async () => {
      const fetch = simularApi({
        'GET /api/salud': SALUD_OK,
        'POST /api/auth/token': { estado: 200, cuerpo: tokenDe('usuario') },
      })
      montar('/ingreso')

      await ingresarComo('demo', 'una-clave')

      expect(await screen.findByRole('heading', { name: 'Panel' })).toBeInTheDocument()
      expect(menu()).toEqual(['Panel', 'Estación', 'Simulación', 'Impacto'])
      const pedido = fetch.mock.calls.map((c) => c[0]).find((p) => p.url.endsWith('/api/auth/token'))!
      expect(await pedido.clone().json()).toEqual({ usuario: 'demo', clave: 'una-clave' })
      expect(JSON.parse(sessionStorage.getItem('aire.sesion')!)).toMatchObject({ rol: 'usuario', usuario: 'demo' })
      expect(localStorage.getItem('aire.sesion')).toBeNull()
    })

    it('con un admin el menú suma Entrenamiento y Clúster', async () => {
      simularApi({
        'GET /api/salud': SALUD_OK,
        'POST /api/auth/token': { estado: 200, cuerpo: tokenDe('admin') },
      })
      montar('/ingreso')

      await ingresarComo('ana', 'otra-clave')

      await screen.findByRole('heading', { name: 'Panel' })
      expect(menu()).toEqual(['Panel', 'Estación', 'Simulación', 'Impacto', 'Entrenamiento', 'Clúster'])
      expect(screen.getByText('administrador', { exact: false })).toBeInTheDocument()
    })

    it('después de ingresar vuelve a la pantalla que se había pedido', async () => {
      simularApi({
        'GET /api/salud': SALUD_OK,
        'POST /api/auth/token': { estado: 200, cuerpo: tokenDe('usuario') },
      })
      montar('/impacto')

      await ingresarComo('demo', 'una-clave')

      expect(await screen.findByRole('heading', { name: 'Impacto' })).toBeInTheDocument()
    })

    it('con un 401 dice «usuario o clave incorrectos», sin más, y borra la clave', async () => {
      simularApi({
        'GET /api/salud': SALUD_OK,
        'POST /api/auth/token': { estado: 401, cuerpo: { error: 'usuario o clave incorrectos' } },
      })
      montar('/ingreso')

      await ingresarComo('demo', 'mala')

      expect(await screen.findByRole('alert')).toHaveTextContent(/^Usuario o clave incorrectos\.$/)
      expect(screen.getByLabelText('Clave')).toHaveValue('')
      expect(screen.getByLabelText('Usuario')).toHaveValue('demo')
      expect(sesionActual()).toBeNull()
    })

    it('si la API no responde lo dice y deja reintentar; al volver la API, entra', async () => {
      let apagada = true
      simularApi({
        'GET /api/salud': SALUD_OK,
        'POST /api/auth/token': () => (apagada ? 'sin_respuesta' : { estado: 200, cuerpo: tokenDe('usuario') }),
      })
      montar('/ingreso')

      await ingresarComo('demo', 'una-clave')

      const alerta = await screen.findByRole('alert')
      expect(alerta).toHaveTextContent('La API no responde')
      expect(screen.getByRole('form', { name: 'Ingreso' })).toBeInTheDocument() // no quedó en blanco

      apagada = false
      await userEvent.click(within(alerta).getByRole('button', { name: 'Reintentar' }))

      expect(await screen.findByRole('heading', { name: 'Panel' })).toBeInTheDocument()
    })

    it('un token sin fecha de vencimiento válida no abre sesión', async () => {
      simularApi({
        'GET /api/salud': SALUD_OK,
        'POST /api/auth/token': { estado: 200, cuerpo: { token: 't', rol: 'usuario', expira: 'mañana' } },
      })
      montar('/ingreso')

      await ingresarComo('demo', 'una-clave')

      expect(await screen.findByRole('alert')).toHaveTextContent('fecha de vencimiento')
      expect(sesionActual()).toBeNull()
    })

    it('con sesión ya iniciada, /ingreso lleva al panel', async () => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('usuario')

      montar('/ingreso')

      expect(await screen.findByRole('heading', { name: 'Panel' })).toBeInTheDocument()
    })
  })

  describe('las guardas por rol', () => {
    it.each(['/entrenamiento', '/cluster'])('a un usuario que fuerza %s se le dice que no tiene permiso', async (ruta) => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('usuario')

      montar(ruta)

      const estado = await screen.findByText('Esta pantalla es solo para administradores')
      expect(estado.closest('[data-estado]')).toHaveAttribute('data-estado', 'sin-permiso')
      expect(screen.queryByText('En construcción')).not.toBeInTheDocument()
    })

    it.each([
      ['/entrenamiento', 'Entrenamiento'],
      ['/cluster', 'Clúster'],
    ])('un admin entra a %s', async (ruta, titulo) => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('admin')

      montar(ruta)

      expect(await screen.findByRole('heading', { name: titulo })).toBeInTheDocument()
    })

    it.each([
      ['/', 'Panel'],
      ['/estacion', 'Estación'],
      ['/estacion/CA-AYAS-01', 'Estación'],
      ['/simulacion', 'Simulación'],
      ['/impacto', 'Impacto'],
    ])('un usuario entra a %s, que todavía muestra su marcador', async (ruta, titulo) => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('usuario')

      montar(ruta)

      expect(await screen.findByRole('heading', { name: titulo })).toBeInTheDocument()
      expect(screen.getByText('En construcción')).toBeInTheDocument()
    })

    it('el muestrario de componentes está fuera del menú, en /muestrario', async () => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('usuario')

      montar('/muestrario')

      expect(await screen.findByRole('heading', { name: 'Muestrario de componentes' })).toBeInTheDocument()
      expect(menu()).not.toContain('Muestrario')
    })

    it('una dirección que no existe lo dice', async () => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('usuario')

      montar('/no-existe')

      expect(await screen.findByText('Esta dirección no existe')).toBeInTheDocument()
    })
  })

  describe('el fin de la sesión', () => {
    it('salir vuelve al ingreso y borra el token', async () => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('admin')
      montar('/')
      await screen.findByRole('heading', { name: 'Panel' })

      await userEvent.click(screen.getByRole('button', { name: 'Salir' }))

      expect(await screen.findByRole('form', { name: 'Ingreso' })).toBeInTheDocument()
      expect(sessionStorage.getItem('aire.sesion')).toBeNull()
      expect(screen.queryByText('La sesión venció.')).not.toBeInTheDocument()
    })

    it('un 401 de la API en cualquier pedido vuelve al ingreso y avisa que la sesión venció', async () => {
      simularApi({
        'GET /api/salud': SALUD_OK,
        'GET /api/estaciones': { estado: 401, cuerpo: { error: 'token ausente, inválido o expirado' } },
      })
      sesionDe('usuario')
      function PideEstaciones() {
        useQuery({ queryKey: ['estaciones'], queryFn: () => pedir(api.GET('/api/estaciones')) })
        return null
      }

      montar('/', <PideEstaciones />)

      expect(await screen.findByRole('form', { name: 'Ingreso' })).toBeInTheDocument()
      expect(screen.getByText('La sesión venció.')).toBeInTheDocument()
      expect(sesionActual()).toBeNull()
    })

    it('al llegar la hora de expira, la sesión se cierra sola', async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true })
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('usuario', 10 * 60_000)
      montar('/')
      await screen.findByRole('heading', { name: 'Panel' })

      act(() => void vi.advanceTimersByTime(10 * 60_000))

      expect(await screen.findByRole('form', { name: 'Ingreso' })).toBeInTheDocument()
      expect(screen.getByText('La sesión venció.')).toBeInTheDocument()
    })

    it('avisa cuando faltan menos de 5 minutos para que venza', async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true })
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('usuario', 10 * 60_000)
      montar('/')
      await screen.findByRole('heading', { name: 'Panel' })
      expect(screen.queryByText('La sesión está por vencer.')).not.toBeInTheDocument()

      act(() => void vi.advanceTimersByTime(6 * 60_000))

      expect(await screen.findByText('La sesión está por vencer.')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Volver a ingresar ahora' })).toBeInTheDocument()
    })
  })

  describe('el tiempo real', () => {
    it('abre un solo WebSocket por sesión, con el token como subprotocolo, y muestra su estado', async () => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('usuario')

      const { enchufes } = montar('/')

      expect(await screen.findByText('Tiempo real: conectando…')).toBeInTheDocument()
      expect(enchufes.creados).toHaveLength(1)
      expect(enchufes.ultimo().protocolos).toEqual(['aire.jwt', 'token-de-usuario'])

      act(() => enchufes.ultimo().abrir())
      expect(await screen.findByText('Tiempo real: conectado')).toBeInTheDocument()
    })

    it('sin sesión no abre ninguno', async () => {
      simularApi({ 'GET /api/salud': SALUD_OK })

      const { enchufes } = montar('/ingreso')

      await screen.findByRole('form', { name: 'Ingreso' })
      expect(enchufes.creados).toHaveLength(0)
    })

    it('una pantalla se suscribe a un evento por su nombre', async () => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('usuario')
      const recibidos = vi.fn()
      function Escucha() {
        useEvento('estacion.regimen', recibidos)
        return null
      }
      const { enchufes } = montar('/', <Escucha />)
      await screen.findByText('Tiempo real: conectando…')
      act(() => enchufes.ultimo().abrir())

      act(() => enchufes.ultimo().recibir({ evento: 'estacion.regimen', datos: { estacion: 'CA-AYAS-01', alerta: true } }))
      act(() => enchufes.ultimo().recibir({ evento: 'entrenamiento.fin', datos: { id: 'e-1', error: 'x' } }))

      expect(recibidos).toHaveBeenCalledExactlyOnceWith({ estacion: 'CA-AYAS-01', alerta: true })
    })

    it('si el servidor cierra con 1008 porque el token venció, vuelve al ingreso y no reintenta', async () => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('usuario')
      const { enchufes } = montar('/')
      await screen.findByText('Tiempo real: conectando…')
      act(() => enchufes.ultimo().abrir())

      act(() => enchufes.ultimo().cerrarDesdeElServidor(1008, 'el token expiró: pide otro y vuelve a conectar'))

      expect(await screen.findByRole('form', { name: 'Ingreso' })).toBeInTheDocument()
      expect(screen.getByText('La sesión venció.')).toBeInTheDocument()
      expect(enchufes.creados).toHaveLength(1)
    })

    it('si el servidor corta por otra cosa (1008 sin vencer), lo muestra y deja reconectar a mano', async () => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('usuario')
      const { enchufes } = montar('/')
      await screen.findByText('Tiempo real: conectando…')
      act(() => enchufes.ultimo().abrir())

      act(() => enchufes.ultimo().cerrarDesdeElServidor(1008, 'cliente lento: no lee al ritmo de los eventos'))

      expect(await screen.findByText('Tiempo real: desconectado')).toBeInTheDocument()
      expect(enchufes.creados).toHaveLength(1)

      await userEvent.click(screen.getByRole('button', { name: 'Reconectar' }))
      expect(enchufes.creados).toHaveLength(2)
    })

    it('al salir cierra el WebSocket', async () => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('usuario')
      const { enchufes } = montar('/')
      await screen.findByText('Tiempo real: conectando…')
      act(() => enchufes.ultimo().abrir())
      const enchufe = enchufes.ultimo()

      await userEvent.click(screen.getByRole('button', { name: 'Salir' }))

      await waitFor(() => expect(enchufe.cerradoCon?.codigo).toBe(1000))
    })
  })

  describe('el armazón', () => {
    it('el pie dice de dónde salen los datos y enlaza a Swagger, al repositorio y al informe', async () => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('usuario')

      montar('/')

      const pie = within(await screen.findByRole('contentinfo'))
      expect(pie.getByText(/OEFA/)).toBeInTheDocument()
      expect(pie.getByRole('link', { name: 'ODC-By' })).toHaveAttribute('href', 'https://opendatacommons.org/licenses/by/')
      expect(pie.getByRole('link', { name: 'API (Swagger)' })).toHaveAttribute('href', '/docs')
      expect(pie.getByRole('link', { name: 'Repositorio' })).toHaveAttribute('href', expect.stringContaining('github.com'))
      expect(pie.getByRole('link', { name: 'Informe' })).toBeInTheDocument()
    })

    it('el modo proyector se pone y se quita, y se recuerda', async () => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('usuario')
      montar('/')
      const boton = await screen.findByRole('button', { name: 'Modo proyector' })
      expect(boton).toHaveAttribute('aria-pressed', 'false')

      await userEvent.click(boton)

      expect(document.documentElement).toHaveAttribute('data-modo', 'proyector')
      expect(boton).toHaveAttribute('aria-pressed', 'true')
      expect(localStorage.getItem('aire.modo')).toBe('proyector')

      await userEvent.click(boton)
      expect(document.documentElement).not.toHaveAttribute('data-modo')
      expect(localStorage.getItem('aire.modo')).toBeNull()
    })

    it('la pantalla en la que se está queda marcada en el menú', async () => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('admin')

      montar('/cluster')

      await screen.findByRole('heading', { name: 'Clúster' })
      const nav = within(screen.getByRole('navigation', { name: 'Pantallas' }))
      expect(nav.getByRole('link', { name: 'Clúster' })).toHaveAttribute('aria-current', 'page')
      expect(nav.getByRole('link', { name: 'Panel' })).not.toHaveAttribute('aria-current')
    })
  })
})
