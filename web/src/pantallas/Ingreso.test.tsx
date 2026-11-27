// El ingreso sin registro: el botón «No pienso crearme una cuenta aquí», el
// diálogo que abre y el ingreso de invitado (POST /api/auth/invitado). El
// ingreso con usuario y clave se prueba de punta a punta en App.test.tsx; aquí,
// que sigue igual con el botón nuevo al lado.

import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { crearClienteDeConsultas } from '../api/cliente'
import { Proveedores, Rutas } from '../App'
import { fabricaDeEnchufes, respuesta, SALUD_OK, simularApi, tokenDe } from '../pruebas/dobles'
import { cerrarSesion, recargarSesion, sesionActual } from '../sesion/almacen'

type RutasDeLaApi = Parameters<typeof simularApi>[0]

const BOTON = 'No pienso crearme una cuenta aquí'
const TITULO = 'Perdón, era parte de la nota'
const SIN_INVITADO = { estado: 404, cuerpo: { error: 'el ingreso de invitado no está habilitado' } } as const

function montar(rutas: RutasDeLaApi = {}) {
  const fetch = simularApi({ 'GET /api/salud': SALUD_OK, ...rutas })
  render(
    <Proveedores consultas={crearClienteDeConsultas()} tiempoReal={{ crearEnchufe: fabricaDeEnchufes().crearEnchufe }}>
      <MemoryRouter initialEntries={['/ingreso']}>
        <Rutas />
      </MemoryRouter>
    </Proveedores>,
  )
  return fetch
}

const boton = () => screen.getByRole('button', { name: BOTON })
const dialogo = () => screen.getByRole('dialog', { name: TITULO })
const pedidosA = (fetch: ReturnType<typeof simularApi>, ruta: string) => fetch.mock.calls.map((c) => c[0]).filter((p) => new URL(p.url).pathname === ruta)

async function abrir() {
  await userEvent.click(boton())
  return dialogo()
}

/** El diálogo se fue y el foco volvió al botón que lo abrió. */
async function cerradoYConElFocoDeVuelta() {
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  expect(boton()).toHaveFocus()
}

/** Un ingreso de invitado que no responde hasta que se le diga. */
function invitadoQueEspera(rutas: RutasDeLaApi = {}) {
  const fetch = montar(rutas)
  let soltar!: (r: Response) => void
  const espera = new Promise<Response>((resolver) => (soltar = resolver))
  let pedidos = 0
  vi.stubGlobal('fetch', (pedido: Request) => {
    if (new URL(pedido.url).pathname !== '/api/auth/invitado') return fetch(pedido)
    pedidos += 1
    return espera
  })
  return {
    responder: (estado: number, cuerpo: unknown) => soltar(respuesta(estado, cuerpo)),
    pedidos: () => pedidos,
  }
}

describe('el ingreso sin registro', () => {
  beforeEach(() => recargarSesion())
  afterEach(() => cerrarSesion('salir'))

  describe('el botón', () => {
    it('está debajo del formulario y es secundario: «Ingresar» sigue siendo el único que lo envía', async () => {
      montar()

      const formulario = await screen.findByRole('form', { name: 'Ingreso' })
      expect(formulario.compareDocumentPosition(boton()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
      expect(formulario).not.toContainElement(boton())
      expect(boton()).toHaveAttribute('type', 'button')
      expect(boton()).toHaveClass('pildora')
      expect(screen.getByRole('button', { name: 'Ingresar' })).toHaveClass('boton')
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    })

    it('con un ingreso con clave en viaje, el botón no se ofrece: dos respuestas no se disputan la sesión', async () => {
      const fetch = montar()
      let soltar!: (r: Response) => void
      const espera = new Promise<Response>((resolver) => (soltar = resolver))
      vi.stubGlobal('fetch', (pedido: Request) =>
        new URL(pedido.url).pathname === '/api/auth/token' ? espera : fetch(pedido),
      )

      await userEvent.type(await screen.findByLabelText('Usuario'), 'admin')
      await userEvent.type(screen.getByLabelText('Clave'), 'una-clave')
      await userEvent.click(screen.getByRole('button', { name: 'Ingresar' }))

      expect(boton()).toBeDisabled()
      soltar(respuesta(401, { error: 'usuario o clave incorrectos' }))
      await waitFor(() => expect(boton()).toBeEnabled())
    })

    it('no pide nada a la API hasta que se aprieta «Entrar igual»', async () => {
      const fetch = montar({ 'POST /api/auth/invitado': { estado: 200, cuerpo: tokenDe('usuario') } })

      await abrir()

      expect(pedidosA(fetch, '/api/auth/invitado')).toHaveLength(0)
      expect(sesionActual()).toBeNull()
    })
  })

  describe('el diálogo', () => {
    it('es modal, con su título, la foto del gato con su descripción y la explicación en serio', async () => {
      montar()

      const d = await abrir()

      expect(d.tagName).toBe('DIALOG')
      expect(d).toHaveAttribute('open')
      expect(d).toHaveAttribute('aria-modal', 'true')
      expect(within(d).getByRole('heading', { name: TITULO })).toBeInTheDocument()
      // Solo el título, la foto y los botones: sin explicación.
      expect(d).not.toHaveTextContent('El enunciado pide')
      const foto = within(d).getByRole('img', { name: /gato .* lágrimas/ })
      // Con el tamaño declarado, el diálogo no salta al cargar la imagen.
      expect(foto).toHaveAttribute('width', '635')
      expect(foto).toHaveAttribute('height', '356')
      // La imagen sale del propio origen (la CSP no admite otra) y sin estilos en línea.
      expect(foto.getAttribute('src')).toMatch(/^\/.*gato.*\.webp$/)
      expect(d.querySelector('[style]')).toBeNull()
      expect(within(d).getByRole('button', { name: 'Cerrar' })).toBeInTheDocument()
    })

    it('al abrir, el foco va a «Entrar igual»', async () => {
      montar()

      const d = await abrir()

      expect(within(d).getByRole('button', { name: 'Entrar igual' })).toHaveFocus()
    })

    it('el foco no sale del diálogo: Tab da la vuelta entre sus controles, en los dos sentidos', async () => {
      montar()
      const d = await abrir()
      const entrar = within(d).getByRole('button', { name: 'Entrar igual' })
      const cerrar = within(d).getByRole('button', { name: 'Cerrar' })

      await userEvent.tab()
      expect(cerrar).toHaveFocus()
      await userEvent.tab()
      expect(entrar).toHaveFocus()
      await userEvent.tab({ shift: true })
      expect(cerrar).toHaveFocus()
      await userEvent.tab({ shift: true })
      expect(entrar).toHaveFocus()
    })

    it('Escape lo cierra y el foco vuelve al botón que lo abrió', async () => {
      montar()
      await abrir()

      await userEvent.keyboard('{Escape}')

      await cerradoYConElFocoDeVuelta()
    })

    it('«Cerrar» lo cierra y el foco vuelve al botón que lo abrió', async () => {
      montar()
      const d = await abrir()

      await userEvent.click(within(d).getByRole('button', { name: 'Cerrar' }))

      await cerradoYConElFocoDeVuelta()
      expect(sesionActual()).toBeNull()
    })

    it('un clic fuera lo cierra; uno dentro, no', async () => {
      montar()
      const d = await abrir()

      await userEvent.click(within(d).getByRole('heading', { name: TITULO }))
      await userEvent.click(within(d).getByRole('img'))
      expect(dialogo()).toBeInTheDocument()

      // Fuera del contenido, el clic cae en el propio <dialog> (su telón).
      await userEvent.click(d)

      await cerradoYConElFocoDeVuelta()
    })

    it('soltar fuera un arrastre que empezó dentro (seleccionar el texto) no lo cierra', async () => {
      montar()
      const d = await abrir()

      await userEvent.pointer([
        { keys: '[MouseLeft>]', target: within(d).getByRole('heading', { name: TITULO }) },
        { target: d },
        { keys: '[/MouseLeft]', target: d },
      ])

      expect(dialogo()).toBeInTheDocument()
    })

    it('se puede volver a abrir después de cerrarlo', async () => {
      montar()
      await abrir()
      await userEvent.keyboard('{Escape}')
      await cerradoYConElFocoDeVuelta()

      const d = await abrir()

      expect(within(d).getByRole('button', { name: 'Entrar igual' })).toHaveFocus()
    })
  })

  describe('«Entrar igual»', () => {
    it('entra como invitado: lleva al panel con el rol usuario, sin mandar credenciales', async () => {
      const fetch = montar({ 'POST /api/auth/invitado': { estado: 200, cuerpo: tokenDe('usuario') } })
      const d = await abrir()

      await userEvent.click(within(d).getByRole('button', { name: 'Entrar igual' }))

      expect(await screen.findByRole('heading', { name: 'Panel' })).toBeInTheDocument()
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
      const menu = within(screen.getByRole('navigation', { name: 'Pantallas' })).getAllByRole('link')
      expect(menu.map((e) => e.textContent)).toEqual(['Panel', 'Estación', 'Simulación', 'Impacto'])
      expect(document.querySelector('[data-sesion-rol]')).toHaveAttribute('data-sesion-rol', 'usuario')
      expect(document.querySelector('[data-sesion-rol]')).toHaveTextContent('invitado · usuario')

      // El pedido no lleva cuerpo ni token: las credenciales de la cuenta no pasan por el navegador.
      const pedidos = pedidosA(fetch, '/api/auth/invitado')
      expect(pedidos).toHaveLength(1)
      expect(pedidos[0]!.method).toBe('POST')
      expect(await pedidos[0]!.clone().text()).toBe('')
      expect(pedidos[0]!.headers.get('Authorization')).toBeNull()
      expect(pedidosA(fetch, '/api/auth/token')).toHaveLength(0)

      // La sesión es la misma que deja el ingreso normal: en sessionStorage y con el token de la API.
      expect(JSON.parse(sessionStorage.getItem('aire.sesion')!)).toMatchObject({ rol: 'usuario', usuario: 'invitado', token: 'token-de-usuario' })
      expect(localStorage.getItem('aire.sesion')).toBeNull()
    })

    it('mientras pide dice «Entrando…» y no pide dos veces', async () => {
      const { responder, pedidos } = invitadoQueEspera()
      const d = await abrir()

      await userEvent.click(within(d).getByRole('button', { name: 'Entrar igual' }))

      const entrando = await within(d).findByRole('button', { name: 'Entrando…' })
      expect(entrando).toHaveAttribute('aria-disabled', 'true')
      // No se deshabilita de verdad: perdería el foco, y dentro del diálogo no hay a dónde vaya.
      expect(entrando).toHaveFocus()
      expect(within(d).queryByRole('button', { name: 'Entrar igual' })).not.toBeInTheDocument()
      // Apretarlo otra vez, con el ratón o con Enter, no pide de nuevo.
      await userEvent.click(entrando)
      await userEvent.keyboard('{Enter}')
      expect(pedidos()).toBe(1)

      responder(200, tokenDe('usuario'))

      expect(await screen.findByRole('heading', { name: 'Panel' })).toBeInTheDocument()
    })

    it('si la API no tiene invitado (404) lo dice y solo queda cerrar', async () => {
      const fetch = montar({ 'POST /api/auth/invitado': SIN_INVITADO })
      const d = await abrir()

      await userEvent.click(within(d).getByRole('button', { name: 'Entrar igual' }))

      const alerta = await within(d).findByRole('alert')
      expect(alerta).toHaveTextContent('Este sistema no tiene cuenta de invitado.')
      expect(alerta).toHaveTextContent('Pídele una a quien lo levantó.')
      // No repite las palabras de la API ni ofrece reintentar lo que no va a cambiar.
      expect(alerta).not.toHaveTextContent('no está habilitado')
      // Y deja de ofrecer lo que no hay.
      expect(within(d).queryByRole('button', { name: /Entrar igual|Entrando/ })).not.toBeInTheDocument()
      const cerrar = within(d).getByRole('button', { name: 'Cerrar' })
      expect(cerrar).toHaveFocus()
      // Con un solo control, Tab se queda en él.
      await userEvent.tab()
      expect(cerrar).toHaveFocus()
      expect(sesionActual()).toBeNull()
      expect(screen.getByRole('form', { name: 'Ingreso' })).toBeInTheDocument()

      await userEvent.click(within(d).getByRole('button', { name: 'Cerrar' }))
      await cerradoYConElFocoDeVuelta()

      // Al abrirlo otra vez empieza de cero: se puede volver a probar (quizá ya le pusieron una).
      const otraVez = await abrir()
      expect(within(otraVez).queryByRole('alert')).not.toBeInTheDocument()
      expect(within(otraVez).getByRole('button', { name: 'Entrar igual' })).toHaveFocus()
      expect(pedidosA(fetch, '/api/auth/invitado')).toHaveLength(1)
    })

    it('si la API no responde lo dice como el resto y deja reintentar; al volver, entra', async () => {
      let apagada = true
      montar({ 'POST /api/auth/invitado': () => (apagada ? 'sin_respuesta' : { estado: 200, cuerpo: tokenDe('usuario') }) })
      const d = await abrir()

      await userEvent.click(within(d).getByRole('button', { name: 'Entrar igual' }))

      const alerta = await within(d).findByRole('alert')
      expect(alerta).toHaveTextContent('La API no responde.')
      expect(alerta).toHaveTextContent('Puede estar apagada o reiniciándose.')
      const reintentar = within(d).getByRole('button', { name: 'Entrar igual' })
      expect(reintentar).toBeEnabled()
      expect(reintentar).toHaveFocus()

      apagada = false
      await userEvent.click(reintentar)

      expect(await screen.findByRole('heading', { name: 'Panel' })).toBeInTheDocument()
    })

    it('un error de la API (503) se muestra con sus palabras', async () => {
      montar({ 'POST /api/auth/invitado': { estado: 503, cuerpo: { error: 'la API se está apagando' } } })
      const d = await abrir()

      await userEvent.click(within(d).getByRole('button', { name: 'Entrar igual' }))

      const alerta = await within(d).findByRole('alert')
      expect(alerta).toHaveTextContent('El servicio no está disponible.')
      expect(alerta).toHaveTextContent('la API se está apagando')
      expect(sesionActual()).toBeNull()
    })

    it('un token sin fecha de vencimiento válida tampoco abre sesión por aquí: es el mismo camino', async () => {
      montar({ 'POST /api/auth/invitado': { estado: 200, cuerpo: { token: 't', rol: 'usuario', expira: 'mañana' } } })
      const d = await abrir()

      await userEvent.click(within(d).getByRole('button', { name: 'Entrar igual' }))

      expect(await within(d).findByRole('alert')).toHaveTextContent('fecha de vencimiento')
      expect(sesionActual()).toBeNull()
    })

    it('si se cierra el diálogo mientras pide, la respuesta que llega después no abre la sesión', async () => {
      const { responder } = invitadoQueEspera()
      const d = await abrir()
      await userEvent.click(within(d).getByRole('button', { name: 'Entrar igual' }))
      await within(d).findByRole('button', { name: 'Entrando…' })

      await userEvent.keyboard('{Escape}')
      await cerradoYConElFocoDeVuelta()
      responder(200, tokenDe('usuario'))
      await new Promise((r) => setTimeout(r, 20))

      expect(sesionActual()).toBeNull()
      expect(screen.getByRole('form', { name: 'Ingreso' })).toBeInTheDocument()
      expect(screen.queryByRole('heading', { name: 'Panel' })).not.toBeInTheDocument()
      // Y no quedó trabado: se puede abrir y entrar.
      expect(within(await abrir()).getByRole('button', { name: 'Entrar igual' })).toBeEnabled()
    })
  })

  describe('el ingreso con usuario y clave', () => {
    it('sigue igual: pide el token con las credenciales y entra con ese usuario', async () => {
      const fetch = montar({ 'POST /api/auth/token': { estado: 200, cuerpo: tokenDe('admin') } })

      await userEvent.type(await screen.findByLabelText('Usuario'), 'ana')
      await userEvent.type(screen.getByLabelText('Clave'), 'otra-clave')
      await userEvent.click(screen.getByRole('button', { name: 'Ingresar' }))

      expect(await screen.findByRole('heading', { name: 'Panel' })).toBeInTheDocument()
      expect(await pedidosA(fetch, '/api/auth/token')[0]!.clone().json()).toEqual({ usuario: 'ana', clave: 'otra-clave' })
      expect(pedidosA(fetch, '/api/auth/invitado')).toHaveLength(0)
      expect(JSON.parse(sessionStorage.getItem('aire.sesion')!)).toMatchObject({ rol: 'admin', usuario: 'ana' })
    })

    it('Enter en el formulario lo envía: no abre el diálogo', async () => {
      const fetch = montar({ 'POST /api/auth/token': { estado: 401, cuerpo: { error: 'usuario o clave incorrectos' } } })

      await userEvent.type(await screen.findByLabelText('Usuario'), 'demo')
      await userEvent.type(screen.getByLabelText('Clave'), 'mala{Enter}')

      expect(await screen.findByRole('alert')).toHaveTextContent(/^Usuario o clave incorrectos\.$/)
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
      expect(pedidosA(fetch, '/api/auth/token')).toHaveLength(1)
    })

    it('un error del formulario no se mezcla con el diálogo', async () => {
      montar({
        'POST /api/auth/token': { estado: 401, cuerpo: { error: 'usuario o clave incorrectos' } },
        'POST /api/auth/invitado': SIN_INVITADO,
      })
      await userEvent.type(await screen.findByLabelText('Usuario'), 'demo')
      await userEvent.type(screen.getByLabelText('Clave'), 'mala{Enter}')
      await screen.findByRole('alert')

      const d = await abrir()

      expect(within(d).queryByRole('alert')).not.toBeInTheDocument()
      await userEvent.click(within(d).getByRole('button', { name: 'Entrar igual' }))
      expect(await within(d).findByRole('alert')).toHaveTextContent('Este sistema no tiene cuenta de invitado.')
    })
  })
})
