// La sesión de punta a punta dentro de la app: el ingreso, el menú según el
// rol, las guardas, el 401 y el vencimiento.

import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, MemoryRouter, Route, Routes } from 'react-router'
import { lazy, StrictMode, useState } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api, claves, crearClienteDeConsultas, pedir } from './api/cliente'
import { EN_VIVO, pedirRegimenes, type EstadoEstacion } from './api/enVivo'
import { useEvento } from './api/tiempoReal'
import { anotarVista, lecturasVistas } from './api/vistas'
import { Proveedores, Rutas } from './App'
import { Armazon } from './armazon/Armazon'
import { entrenamientoALaVista } from './pantallas/entrenamiento/almacen'
import { simulador } from './pantallas/simulacion/motor'
import type { Tramo } from './pantallas/simulacion/series'
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

    // Las seis pantallas de la spec (§5.2 a §5.7) ya están construidas: ninguna
    // ruta muestra el marcador «En construcción» con que nació el andamiaje, y
    // cada una se presenta con su encabezado y su bajada.
    it.each([
      ['/', 'Panel', 'usuario', 'El último estado recibido de cada estación, con su fecha.'],
      ['/estacion', 'Estación', 'usuario', 'El último estado recibido de una estación,'],
      ['/simulacion', 'Simulación', 'usuario', 'Lecturas reales de OEFA mandadas una por una a la API'],
      ['/impacto', 'Impacto', 'usuario', 'Qué dicen las mediciones reales de OEFA'],
      ['/entrenamiento', 'Entrenamiento', 'admin', 'El K-Means se reparte entre los nodos del clúster.'],
      ['/cluster', 'Clúster', 'admin', 'Los nodos que entrenan, cómo se porta la API que los coordina'],
    ] as const)('%s ya no muestra «En construcción»: es la pantalla de %s', async (ruta, titulo, rol, bajada) => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe(rol)

      montar(ruta)

      const encabezado = (await screen.findByRole('heading', { level: 1, name: titulo })).closest('[data-encabezado]')
      expect(encabezado).toHaveTextContent(bajada)
      expect(screen.queryByText('En construcción')).not.toBeInTheDocument()
      expect(document.querySelector('[data-en-construccion]')).toBeNull()
      // La pantalla dibuja algo propio debajo del encabezado, aunque la API de la prueba no le responda nada.
      expect(encabezado!.parentElement!.children.length).toBeGreaterThan(1)
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
    ])('un usuario entra a %s', async (ruta, titulo) => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('usuario')

      montar(ruta)

      expect(await screen.findByRole('heading', { name: titulo })).toBeInTheDocument()
      expect(screen.queryByText('En construcción')).not.toBeInTheDocument()
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
    describe('no deja nada para quien entre después en la misma pestaña', () => {
      const ESTADO = {
        estacion: 'CA-HU-04',
        fecha_hora: '2021-12-15T10:00:00-05:00',
        regimen: 2,
        excede_eca: false,
        media_24h: { pm2_5: 88.8, pm10: 120.7, lecturas: 24 },
        alerta: true,
        inca: 'mala',
        ts: '2026-11-21T17:10:00-05:00',
      }
      const lecturaDe = (hora: number) => ({ estacion: 'CA-HU-04', fecha_hora: `2021-12-15T1${hora}:00:00-05:00`, pm2_5: 20, pm10: 90 })
      const TRAMO: Tramo = {
        id: 'dia-de-prueba',
        tipo: 'dia',
        titulo: 'CA-HU-04 · 15 dic 2021',
        desde: lecturaDe(1).fecha_hora,
        hasta: lecturaDe(3).fecha_hora,
        lecturas: [lecturaDe(1), lecturaDe(2), lecturaDe(3)],
        estaciones: ['CA-HU-04'],
        esperado: {},
        primera: { 'CA-HU-04': lecturaDe(1).fecha_hora },
        ultima: { 'CA-HU-04': lecturaDe(3).fecha_hora },
      }
      const PREDICCION = {
        estacion: 'CA-HU-04',
        regimen: 2,
        excede_eca: false,
        media_24h: { pm2_5: 80, pm10: 119, lecturas: 24 },
        alerta: true,
        inca: 'mala',
        pm2_5_medio_regimen: 24.4,
        pm10_medio_regimen: 58.2,
        modelo_id: 'sembrado-kmeans-json',
        ms: 0.04,
      }
      const ENTRENAMIENTO = { id: 'e-1', estado: 'terminado', nodos: ['nodo-a'], replicas: 1, iteraciones: 1, ms_total: 50, rondas: [] }

      afterEach(() => simulador.reiniciar())

      /** Una sesión de admin que ya dejó de todo en la pestaña: curvas, lecturas enviadas, un entrenamiento a la vista y una simulación corriendo. */
      async function sesionConDeTodo(rutas: Parameters<typeof simularApi>[0] = {}) {
        simularApi({ 'GET /api/salud': SALUD_OK, 'POST /api/predicciones': { estado: 200, cuerpo: PREDICCION }, ...rutas })
        sesionDe('admin')
        const { enchufes } = montar('/muestrario')
        await screen.findByRole('heading', { name: 'Muestrario de componentes' })
        act(() => enchufes.ultimo().abrir())
        // Un estado recibido: queda para la curva, también en sessionStorage.
        act(() => enchufes.ultimo().recibir({ evento: 'estacion.regimen', datos: ESTADO }))
        await waitFor(() => expect(sessionStorage.getItem('aire.curvas')).toContain('CA-HU-04'))
        // Un entrenamiento abierto del historial, a la vista.
        act(() => entrenamientoALaVista.despachar({ tipo: 'ver', detalle: ENTRENAMIENTO as never }))
        // Y una simulación en curso, que ya mandó una lectura y espera para la siguiente.
        act(() => simulador.iniciar(TRAMO, 1))
        await waitFor(() => expect(lecturasVistas('CA-HU-04')).toHaveLength(1))
        expect(simulador.leer().fase).toBe('en_curso')
        expect(entrenamientoALaVista.leer().seguimiento.corrida?.id).toBe('e-1')
      }

      function nadaQueda() {
        expect(sessionStorage.getItem('aire.curvas')).toBeNull()
        expect(lecturasVistas('CA-HU-04')).toEqual([])
        expect(entrenamientoALaVista.leer().seguimiento.corrida).toBeNull()
        expect(simulador.leer()).toMatchObject({ fase: 'lista', tramo: null, enviadas: 0 })
        expect(document.querySelector('[data-simulacion-en-curso]')).toBeNull()
      }

      it('«Salir» borra las curvas, las lecturas enviadas y el entrenamiento a la vista, y detiene la simulación', async () => {
        await sesionConDeTodo()

        await userEvent.click(screen.getByRole('button', { name: 'Salir' }))

        await screen.findByRole('form', { name: 'Ingreso' })
        nadaQueda()
      })

      it('lo mismo cuando la sesión vence', async () => {
        await sesionConDeTodo()

        act(() => cerrarSesion('vencida'))

        await screen.findByRole('form', { name: 'Ingreso' })
        expect(screen.getByText('La sesión venció.')).toBeInTheDocument()
        nadaQueda()
      })

      it('lo mismo cuando la API responde 401 a un pedido', async () => {
        await sesionConDeTodo({ 'GET /api/estaciones': { estado: 401, cuerpo: { error: 'token inválido' } } })

        // El Panel pide el catálogo y la API ya no acepta el token.
        await userEvent.click(within(screen.getByRole('navigation', { name: 'Pantallas' })).getByRole('link', { name: 'Panel' }))

        await screen.findByRole('form', { name: 'Ingreso' })
        expect(sesionActual()).toBeNull()
        nadaQueda()
      })

      it('la simulación detenida no manda más lecturas con la sesión cerrada', async () => {
        await sesionConDeTodo()
        const predicciones = () => (globalThis.fetch as ReturnType<typeof simularApi>).mock.calls.filter((c) => c[0].url.endsWith('/api/predicciones')).length
        const antes = predicciones()

        await userEvent.click(screen.getByRole('button', { name: 'Salir' }))
        await screen.findByRole('form', { name: 'Ingreso' })
        // Pasa de sobra el segundo que el simulador esperaba para la lectura siguiente.
        await new Promise((r) => setTimeout(r, 1200))

        expect(predicciones()).toBe(antes)
      })

      it('quien entra después ve la curva de la estación vacía, no la del anterior', async () => {
        await sesionConDeTodo({
          'POST /api/auth/token': { estado: 200, cuerpo: tokenDe('usuario') },
          'GET /api/estaciones': { estado: 200, cuerpo: [{ id: 'CA-HU-04', objeto_fiscalizado: 'Ladrilleras', tipo_actividad: 'ladrilleras', departamento: 'LIMA', provincia: 'LIMA', distrito: 'LURIGANCHO', lat: -11.99, lon: -76.92 }] },
          'GET /datos/impacto.json': { estado: 200, cuerpo: { metadatos: { reglas: {} }, totales: {}, distritos: [], senales: {}, anticipacion: {}, alcance: { estaciones_sin_pm_horario: [] }, estaciones: [], poblacion_distritos: [] } },
          'GET /api/estaciones/CA-HU-04/regimen': { estado: 404, cuerpo: { error: 'la estación CA-HU-04 todavía no tiene un régimen registrado' } },
        })
        await userEvent.click(screen.getByRole('button', { name: 'Salir' }))
        await screen.findByRole('form', { name: 'Ingreso' })

        await ingresarComo('demo', 'una-clave')
        await screen.findByRole('navigation', { name: 'Pantallas' })
        await userEvent.click(within(screen.getByRole('navigation', { name: 'Pantallas' })).getByRole('link', { name: 'Estación' }))
        await userEvent.selectOptions(await screen.findByRole('combobox'), 'CA-HU-04')

        expect(await screen.findByText(/Esta pestaña todavía no recibió ningún estado de CA-HU-04 ni le envió lecturas/)).toBeInTheDocument()
        expect(document.querySelectorAll('[data-capa="curva"] [data-punto], [data-lectura]')).toHaveLength(0)
      })

      it('al arrancar sin sesión borra lo que una sesión vencida dejó guardado antes de recargar', async () => {
        simularApi({ 'GET /api/salud': SALUD_OK })
        sessionStorage.setItem('aire.curvas', JSON.stringify({ 'CA-HU-04': [{ fecha_hora: ESTADO.fecha_hora, t: Date.parse(ESTADO.fecha_hora), pm2_5: 88.8, pm10: 120.7, lecturas: 24, alerta: true, inca: 'mala' }] }))

        montar('/ingreso')

        await screen.findByRole('form', { name: 'Ingreso' })
        await waitFor(() => expect(sessionStorage.getItem('aire.curvas')).toBeNull())
      })

      it('recargar la página con la sesión viva no borra las curvas, tampoco en desarrollo (StrictMode)', async () => {
        simularApi({ 'GET /api/salud': SALUD_OK })
        sesionDe('usuario')
        sessionStorage.setItem('aire.curvas', JSON.stringify({ 'CA-HU-04': [{ fecha_hora: ESTADO.fecha_hora, t: Date.parse(ESTADO.fecha_hora), pm2_5: 88.8, pm10: 120.7, lecturas: 24, alerta: true, inca: 'mala' }] }))
        const enchufes = fabricaDeEnchufes()

        // Como main.tsx: con StrictMode, que en desarrollo desmonta y vuelve a montar todo al cargar.
        render(
          <StrictMode>
            <Proveedores consultas={crearClienteDeConsultas()} tiempoReal={{ crearEnchufe: enchufes.crearEnchufe }}>
              <MemoryRouter initialEntries={['/muestrario']}>
                <Rutas />
              </MemoryRouter>
            </Proveedores>
          </StrictMode>,
        )

        await screen.findByRole('heading', { name: 'Muestrario de componentes' })
        expect(sessionStorage.getItem('aire.curvas')).toContain('CA-HU-04')
        expect(sesionActual()).not.toBeNull()
      })

      it('y con StrictMode, «Salir» sigue limpiando todo', async () => {
        simularApi({ 'GET /api/salud': SALUD_OK })
        sesionDe('usuario')
        sessionStorage.setItem('aire.curvas', JSON.stringify({ 'CA-HU-04': [] }))
        const enchufes = fabricaDeEnchufes()
        render(
          <StrictMode>
            <Proveedores consultas={crearClienteDeConsultas()} tiempoReal={{ crearEnchufe: enchufes.crearEnchufe }}>
              <MemoryRouter initialEntries={['/muestrario']}>
                <Rutas />
              </MemoryRouter>
            </Proveedores>
          </StrictMode>,
        )
        await screen.findByRole('heading', { name: 'Muestrario de componentes' })
        act(() => anotarVista('CA-HU-04', { fecha_hora: ESTADO.fecha_hora, pm2_5: 20, pm10: 90, media_pm2_5: null, media_pm10: null, lecturas: 1, alerta: false, inca: null, regimen: 0, excede_eca: false }))

        await userEvent.click(screen.getByRole('button', { name: 'Salir' }))

        await screen.findByRole('form', { name: 'Ingreso' })
        expect(sessionStorage.getItem('aire.curvas')).toBeNull()
        expect(lecturasVistas('CA-HU-04')).toEqual([])
      })
    })

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

    it('quien entra después de un «Salir» no aterriza en la pantalla del anterior', async () => {
      simularApi({
        'GET /api/salud': SALUD_OK,
        'POST /api/auth/token': { estado: 200, cuerpo: tokenDe('usuario') },
      })
      sesionDe('admin')
      montar('/cluster')
      await screen.findByRole('heading', { name: 'Clúster' })

      await userEvent.click(screen.getByRole('button', { name: 'Salir' }))
      await screen.findByRole('form', { name: 'Ingreso' })
      await ingresarComo('demo', 'una-clave')

      expect(await screen.findByRole('heading', { name: 'Panel' })).toBeInTheDocument()
      expect(screen.queryByText('Esta pantalla es solo para administradores')).not.toBeInTheDocument()
    })

    it('si la sesión venció, quien vuelve a entrar sí regresa a donde estaba', async () => {
      simularApi({
        'GET /api/salud': SALUD_OK,
        'POST /api/auth/token': { estado: 200, cuerpo: tokenDe('admin') },
      })
      sesionDe('admin')
      montar('/cluster')
      await screen.findByRole('heading', { name: 'Clúster' })

      act(() => cerrarSesion('vencida'))
      await screen.findByRole('form', { name: 'Ingreso' })
      await ingresarComo('ana', 'otra-clave')

      expect(await screen.findByRole('heading', { name: 'Clúster' })).toBeInTheDocument()
    })

    it('si el rol que entra no puede ver la pantalla que se había pedido, va al panel', async () => {
      simularApi({
        'GET /api/salud': SALUD_OK,
        'POST /api/auth/token': { estado: 200, cuerpo: tokenDe('usuario') },
      })
      montar('/entrenamiento') // sin sesión: la guarda recuerda /entrenamiento

      await ingresarComo('demo', 'una-clave')

      expect(await screen.findByRole('heading', { name: 'Panel' })).toBeInTheDocument()
      expect(screen.queryByText('Esta pantalla es solo para administradores')).not.toBeInTheDocument()
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

      const aviso = (await screen.findByText('La sesión está por vencer.')).closest('[role]')!
      expect(screen.getByRole('button', { name: 'Volver a ingresar ahora' })).toBeInTheDocument()
      // No interrumpe ni se vuelve a anunciar cada minuto: es un «status», y la
      // cuenta de minutos, que es lo que cambia, queda fuera de lo que se anuncia.
      expect(aviso).toHaveAttribute('role', 'status')
      expect(within(aviso as HTMLElement).getByText(/en 4 minutos/)).toHaveAttribute('aria-hidden', 'true')
      const anunciado = [...aviso.querySelectorAll('p')].map((p) => {
        const copia = p.cloneNode(true) as HTMLElement
        copia.querySelectorAll('[aria-hidden="true"]').forEach((e) => e.remove())
        return copia.textContent
      })

      act(() => void vi.advanceTimersByTime(60_000))

      expect(await screen.findByText(/en 3 minutos/)).toBeInTheDocument()
      const despues = [...aviso.querySelectorAll('p')].map((p) => {
        const copia = p.cloneNode(true) as HTMLElement
        copia.querySelectorAll('[aria-hidden="true"]').forEach((e) => e.remove())
        return copia.textContent
      })
      expect(despues).toEqual(anunciado)
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

    it('un token que la API ya no acepta no queda reintentando: al tercer apretón fallido se comprueba la sesión y vuelve al ingreso', async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true })
      const fetch = simularApi({
        'GET /api/salud': SALUD_OK,
        // La API se reinició con otro JWT_SECRETO: el token de la pestaña ya no vale.
        'GET /api/modelos/activo': { estado: 401, cuerpo: { error: 'token ausente, inválido o expirado' } },
      })
      sesionDe('usuario')
      const { enchufes } = montar('/')
      await screen.findByText('Tiempo real: conectando…')

      // El 401 del apretón de manos llega al navegador como un cierre 1006, igual que una API apagada.
      for (let i = 0; i < 3; i++) {
        act(() => enchufes.ultimo().cerrarDesdeElServidor(1006))
        act(() => void vi.advanceTimersByTime(30_000))
      }

      expect(await screen.findByRole('form', { name: 'Ingreso' })).toBeInTheDocument()
      expect(screen.getByText('La sesión venció.')).toBeInTheDocument()
      expect(fetch.mock.calls.filter((c) => c[0].url.endsWith('/api/modelos/activo'))).toHaveLength(1)
      const creados = enchufes.creados.length
      act(() => void vi.advanceTimersByTime(120_000))
      expect(enchufes.creados).toHaveLength(creados) // dejó de reintentar
    })

    it('si la API solo está apagada, el apretón fallido no saca de la sesión: sigue reintentando', async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true })
      simularApi({ 'GET /api/salud': SALUD_OK, 'GET /api/modelos/activo': { estado: 502 } })
      sesionDe('usuario')
      const { enchufes } = montar('/')
      await screen.findByText('Tiempo real: conectando…')

      for (let i = 0; i < 4; i++) {
        act(() => enchufes.ultimo().cerrarDesdeElServidor(1006))
        act(() => void vi.advanceTimersByTime(30_000))
      }

      expect(await screen.findByText('Tiempo real: reconectando…')).toBeInTheDocument()
      expect(sesionActual()).not.toBeNull()
      expect(enchufes.creados).toHaveLength(5)
    })

    describe('la caché en vivo', () => {
      const estadoDe = (estacion: string, hora: number, alerta = false): EstadoEstacion => ({
        estacion,
        fecha_hora: `2021-12-13T${hora}:00:00-05:00`,
        regimen: alerta ? 1 : 0,
        excede_eca: alerta,
        media_24h: { pm2_5: 10, pm10: alerta ? 121.4 : 20, lecturas: 24 },
        alerta,
        inca: alerta ? 'mala' : 'buena',
        ts: `2021-12-13T${hora}:00:02-05:00`,
      })

      // Se monta en una pantalla que no pide datos: en «/» está el Panel, que lee esta misma lista.
      const SIN_DATOS = '/muestrario'

      // Lo que hace el Panel: leer la lista de la caché, marcada en vivo. No se suscribe a nada.
      function Lista({ pedirLista }: { pedirLista: () => Promise<EstadoEstacion[]> }) {
        const consultas = useQueryClient()
        const lista = useQuery({
          queryKey: claves.regimenes,
          meta: EN_VIVO,
          queryFn: () => pedirRegimenes(consultas, pedirLista),
        })
        return (
          <ul aria-label="Estados">
            {lista.data?.map((e) => (
              <li key={e.estacion}>
                {e.estacion} {e.alerta ? 'en alerta' : 'sin alerta'}
              </li>
            ))}
          </ul>
        )
      }
      const filas = () => within(screen.getByRole('list', { name: 'Estados' })).queryAllByRole('listitem').map((f) => f.textContent)

      it('el evento estacion.regimen actualiza la lista sin que la pantalla se suscriba', async () => {
        simularApi({ 'GET /api/salud': SALUD_OK })
        sesionDe('usuario')
        const pedirLista = vi.fn(async () => [estadoDe('CA-A', 14), estadoDe('CA-B', 14)])
        const { enchufes } = montar(SIN_DATOS, <Lista pedirLista={pedirLista} />)
        await waitFor(() => expect(filas()).toEqual(['CA-A sin alerta', 'CA-B sin alerta']))
        act(() => enchufes.ultimo().abrir())
        await waitFor(() => expect(pedirLista).toHaveBeenCalledTimes(2)) // al conectar se refresca

        act(() => enchufes.ultimo().recibir({ evento: 'estacion.regimen', datos: estadoDe('CA-B', 15, true) }))

        await waitFor(() => expect(filas()).toEqual(['CA-A sin alerta', 'CA-B en alerta']))
        expect(pedirLista).toHaveBeenCalledTimes(2) // el evento no vuelve a pedir la lista
      })

      it('al volver de un corte se vuelve a pedir lo marcado en vivo: lo que cambió sin conexión aparece', async () => {
        vi.useFakeTimers({ shouldAdvanceTime: true })
        simularApi({ 'GET /api/salud': SALUD_OK })
        sesionDe('usuario')
        let enLaApi = [estadoDe('CA-A', 14)]
        const pedirLista = vi.fn(async () => enLaApi)
        const { enchufes } = montar(SIN_DATOS, <Lista pedirLista={pedirLista} />)
        act(() => enchufes.ultimo().abrir())
        await waitFor(() => expect(filas()).toEqual(['CA-A sin alerta']))
        // Un evento antes del corte: de CA-A «ya se sabe por eventos»… hasta que la conexión se cae.
        act(() => enchufes.ultimo().recibir({ evento: 'estacion.regimen', datos: estadoDe('CA-A', 15) }))
        const antes = pedirLista.mock.calls.length

        act(() => enchufes.ultimo().cerrarDesdeElServidor(1006))
        enLaApi = [estadoDe('CA-A', 16, true)] // cambió mientras no había WebSocket: ese evento se perdió
        act(() => void vi.advanceTimersByTime(1_000))
        expect(pedirLista).toHaveBeenCalledTimes(antes) // reconectando todavía no pide nada
        act(() => enchufes.ultimo().abrir())

        // Al reabrirse se olvidó el evento viejo de CA-A: manda lo que responde el GET.
        await waitFor(() => expect(filas()).toEqual(['CA-A en alerta']))
        expect(pedirLista).toHaveBeenCalledTimes(antes + 1)
      })

      it('con el WebSocket caído, lo que responde la API manda sobre el evento de antes del corte', async () => {
        vi.useFakeTimers({ shouldAdvanceTime: true })
        simularApi({ 'GET /api/salud': SALUD_OK })
        sesionDe('usuario')
        let enLaApi = [estadoDe('CA-A', 9)]
        const pedirLista = vi.fn(async () => enLaApi)
        // Una pantalla que se puede cerrar y volver a abrir, como al navegar al Panel.
        function ConLista() {
          const [abierta, setAbierta] = useState(true)
          return (
            <>
              <button type="button" onClick={() => setAbierta(!abierta)}>
                {abierta ? 'cerrar la lista' : 'abrir la lista'}
              </button>
              {abierta && <Lista pedirLista={pedirLista} />}
            </>
          )
        }
        const { enchufes } = montar(SIN_DATOS, <ConLista />)
        act(() => enchufes.ultimo().abrir())
        await waitFor(() => expect(filas()).toEqual(['CA-A sin alerta']))
        act(() => enchufes.ultimo().recibir({ evento: 'estacion.regimen', datos: estadoDe('CA-A', 10) }))

        // Cae el WebSocket; otra sesión manda la lectura de las 11, que deja a CA-A en alerta. Ese evento no llega.
        act(() => enchufes.ultimo().cerrarDesdeElServidor(1006))
        await screen.findByText('Tiempo real: reconectando…')
        enLaApi = [estadoDe('CA-A', 11, true)]

        // Se navega a la pantalla sin haber reconectado: el GET trae las 11 y eso es lo que se ve.
        await userEvent.click(screen.getByRole('button', { name: 'cerrar la lista' }))
        await userEvent.click(screen.getByRole('button', { name: 'abrir la lista' }))

        await waitFor(() => expect(filas()).toEqual(['CA-A en alerta']))
        expect(screen.getByText('Tiempo real: reconectando…')).toBeInTheDocument()
      })

      it('un evento que llega apenas se abre el WebSocket no se pierde al vaciar la memoria de la conexión', async () => {
        simularApi({ 'GET /api/salud': SALUD_OK })
        sesionDe('usuario')
        let responder!: (lista: EstadoEstacion[]) => void
        const pedirLista = vi.fn(() => new Promise<EstadoEstacion[]>((r) => (responder = r)))
        const { enchufes } = montar(SIN_DATOS, <Lista pedirLista={pedirLista} />)
        await waitFor(() => expect(pedirLista).toHaveBeenCalled())

        // Abre y recibe en el mismo turno, sin dejar que React dibuje en el medio: la memoria
        // de la conexión se vacía al abrir, no un render después, o se llevaría este evento.
        act(() => {
          enchufes.ultimo().abrir()
          enchufes.ultimo().recibir({ evento: 'estacion.regimen', datos: estadoDe('CA-B', 15, true) })
        })
        await act(() => new Promise((r) => setTimeout(r, 30))) // React dibuja y corren sus efectos
        await act(async () => responder([estadoDe('CA-B', 14)])) // y recién ahora llega la foto vieja del GET

        await waitFor(() => expect(filas()).toEqual(['CA-B en alerta']))
      })

      it('llega el evento S2 y después la respuesta del GET con S1: queda S2', async () => {
        simularApi({ 'GET /api/salud': SALUD_OK })
        sesionDe('usuario')
        let responder!: (lista: EstadoEstacion[]) => void
        const pedirLista = vi.fn(() => new Promise<EstadoEstacion[]>((r) => (responder = r)))
        const { enchufes } = montar(SIN_DATOS, <Lista pedirLista={pedirLista} />)
        await waitFor(() => expect(pedirLista).toHaveBeenCalled())
        // El WebSocket conecta con el primer GET todavía en viaje: TanStack Query
        // no lo cancela (aún no hay datos), así que es ese mismo pedido el que sigue.
        act(() => enchufes.ultimo().abrir())

        act(() => enchufes.ultimo().recibir({ evento: 'estacion.regimen', datos: estadoDe('CA-B', 15, true) }))
        await act(async () => responder([estadoDe('CA-A', 14), estadoDe('CA-B', 14)])) // la foto de antes del evento

        await waitFor(() => expect(filas()).toEqual(['CA-A sin alerta', 'CA-B en alerta']))
        expect(pedirLista).toHaveBeenCalledOnce()
      })
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

    it('anota lo que llega de cada estación desde cualquier pantalla, sin haber pasado por el Panel ni por la Estación', async () => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('usuario')
      // Como después de un F5 en una pantalla que no pide el estado de las estaciones.
      const { enchufes } = montar('/muestrario')
      await screen.findByRole('heading', { name: 'Muestrario de componentes' })
      act(() => enchufes.ultimo().abrir())

      act(() =>
        enchufes.ultimo().recibir({
          evento: 'estacion.regimen',
          datos: {
            estacion: 'CA-HU-04',
            fecha_hora: '2021-12-15T10:00:00-05:00',
            regimen: 2,
            excede_eca: true,
            media_24h: { pm2_5: 88.8, pm10: 120.7, lecturas: 24 },
            alerta: true,
            inca: 'mala',
            ts: '2026-11-21T17:10:00-05:00',
          },
        }),
      )

      // Queda para la curva de la estación (api/historia.ts), que se guarda en sessionStorage.
      await waitFor(() => expect(JSON.parse(sessionStorage.getItem('aire.curvas') ?? '{}')).toHaveProperty('CA-HU-04'))
      expect(JSON.parse(sessionStorage.getItem('aire.curvas')!)['CA-HU-04']).toMatchObject([{ pm10: 120.7, alerta: true }])
    })

    describe('la simulación en curso', () => {
      const lectura = (hora: number) => ({ estacion: 'CA-HU-04', fecha_hora: `2021-12-14T0${hora}:00:00-05:00`, pm2_5: 20, pm10: 90 })
      const TRAMO: Tramo = {
        id: 'dia-de-prueba',
        tipo: 'dia',
        titulo: 'CA-HU-04 · 2021-12-14',
        desde: lectura(0).fecha_hora,
        hasta: lectura(2).fecha_hora,
        lecturas: [lectura(0), lectura(1), lectura(2)],
        estaciones: ['CA-HU-04'],
        esperado: {},
        primera: { 'CA-HU-04': lectura(0).fecha_hora },
        ultima: { 'CA-HU-04': lectura(2).fecha_hora },
      }
      const PREDICCION = {
        estado: 200,
        cuerpo: {
          estacion: 'CA-HU-04',
          regimen: 2,
          excede_eca: false,
          media_24h: { pm2_5: null, pm10: null, lecturas: 1 },
          alerta: false,
          inca: null,
          pm2_5_medio_regimen: 24.4,
          pm10_medio_regimen: 58.2,
          modelo_id: 'sembrado-kmeans-json',
          ms: 0.04,
        },
      }
      const indicador = () => document.querySelector<HTMLElement>('[data-simulacion-en-curso]')

      afterEach(() => simulador.reiniciar())

      it('sin corrida, la barra no dice nada de la simulación', async () => {
        simularApi({ 'GET /api/salud': SALUD_OK })
        sesionDe('usuario')

        montar('/muestrario')

        await screen.findByRole('heading', { name: 'Muestrario de componentes' })
        expect(indicador()).toBeNull()
      })

      it('mientras corre se ve desde cualquier pantalla, con su avance, y lleva a Simulación; al detenerla se va', async () => {
        simularApi({ 'GET /api/salud': SALUD_OK, 'POST /api/predicciones': PREDICCION })
        sesionDe('usuario')
        montar('/muestrario')
        await screen.findByRole('heading', { name: 'Muestrario de componentes' })

        // Lo que hace la pantalla de Simulación: el simulador de la app, que vive fuera de React.
        // A una hora simulada por segundo: manda la primera lectura y espera.
        act(() => simulador.iniciar(TRAMO, 1))

        const enlace = await screen.findByRole('link', { name: /Simulación en curso/ })
        expect(enlace).toHaveAttribute('href', '/simulacion')
        await waitFor(() => expect(indicador()).toHaveTextContent('Simulación en curso1/3'))
        expect(indicador()).toHaveAttribute('data-simulacion-en-curso', 'en_curso')
        expect(enlace).toHaveAttribute('title', expect.stringContaining('CA-HU-04 · 2021-12-14, 1 de 3 lecturas enviadas'))

        act(() => simulador.pausar())
        expect(indicador()).toHaveTextContent('Simulación en pausa')

        await userEvent.click(within(indicador()!).getByRole('link', { name: /Simulación en pausa/ }))
        expect(await screen.findByRole('heading', { name: 'Simulación' })).toBeInTheDocument()
        expect(indicador()).not.toBeNull()

        act(() => simulador.detener())
        expect(indicador()).toBeNull()
      })

      it('si un pedido falla, dice que la simulación se detuvo por un error', async () => {
        simularApi({ 'GET /api/salud': SALUD_OK, 'POST /api/predicciones': { estado: 503, cuerpo: { error: 'MongoDB no disponible' } } })
        sesionDe('usuario')
        montar('/muestrario')
        await screen.findByRole('heading', { name: 'Muestrario de componentes' })

        act(() => simulador.iniciar(TRAMO, 1))

        await waitFor(() => expect(indicador()).toHaveAttribute('data-simulacion-en-curso', 'con_error'))
        expect(indicador()).toHaveTextContent('Simulación detenida por un error0/3')
      })

      it('al salir de la sesión la corrida se corta y el indicador no queda para quien entre después', async () => {
        simularApi({ 'GET /api/salud': SALUD_OK, 'POST /api/predicciones': PREDICCION })
        sesionDe('usuario')
        montar('/muestrario')
        await screen.findByRole('heading', { name: 'Muestrario de componentes' })
        act(() => simulador.iniciar(TRAMO, 1))
        await waitFor(() => expect(indicador()).not.toBeNull())

        await userEvent.click(screen.getByRole('button', { name: 'Salir' }))

        await screen.findByRole('form', { name: 'Ingreso' })
        expect(simulador.leer().fase).toBe('lista')
      })
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

    describe('si una pantalla falla', () => {
      // El armazón con una pantalla que se puede romper y otra sana, como las monta App.tsx.
      function montarCon(pantalla: React.ReactNode) {
        const enchufes = fabricaDeEnchufes()
        return render(
          <Proveedores consultas={crearClienteDeConsultas()} tiempoReal={{ crearEnchufe: enchufes.crearEnchufe }}>
            <MemoryRouter initialEntries={['/simulacion']}>
              <Routes>
                <Route element={<Armazon />}>
                  <Route path="/simulacion" element={pantalla} />
                  <Route path="/" element={<h1>Panel sano</h1>} />
                </Route>
              </Routes>
            </MemoryRouter>
          </Proveedores>,
        )
      }

      it('al dibujarse, se dice en su lugar y la barra y el menú siguen ahí; navegar limpia el error', async () => {
        vi.spyOn(console, 'error').mockImplementation(() => {})
        simularApi({ 'GET /api/salud': SALUD_OK })
        sesionDe('usuario')
        function Rota(): never {
          throw new Error('media_24h es undefined')
        }

        montarCon(<Rota />)

        const alerta = await screen.findByRole('alert')
        expect(within(alerta).getByText('Esta pantalla falló')).toBeInTheDocument()
        expect(within(alerta).getByText('media_24h es undefined')).toBeInTheDocument()
        expect(menu()).toEqual(['Panel', 'Estación', 'Simulación', 'Impacto'])
        expect(screen.getByRole('button', { name: 'Salir' })).toBeInTheDocument()
        expect(screen.getByRole('contentinfo')).toBeInTheDocument()

        await userEvent.click(within(screen.getByRole('navigation', { name: 'Pantallas' })).getByRole('link', { name: 'Panel' }))

        expect(await screen.findByRole('heading', { name: 'Panel sano' })).toBeInTheDocument()
        expect(screen.queryByRole('alert')).not.toBeInTheDocument()
      })

      it('porque su archivo ya no se puede descargar, ofrece recargar la página y no queda en blanco', async () => {
        vi.spyOn(console, 'error').mockImplementation(() => {})
        simularApi({ 'GET /api/salud': SALUD_OK })
        sesionDe('usuario')
        // Lo que lanza React.lazy cuando el import() falla (la imagen se reconstruyó con la pestaña abierta).
        const Perezosa = lazy<React.ComponentType>(() =>
          Promise.reject(
            new TypeError('Failed to fetch dynamically imported module: http://localhost:8081/assets/Simulacion-4OOULP5B.js'),
          ),
        )

        montarCon(<Perezosa />)

        const alerta = await screen.findByRole('alert')
        expect(within(alerta).getByText('Esta pantalla no se pudo cargar')).toBeInTheDocument()
        expect(within(alerta).getByRole('button', { name: 'Recargar la página' })).toBeInTheDocument()
        expect(within(alerta).queryByRole('button', { name: 'Reintentar' })).not.toBeInTheDocument()
        expect(menu()).toEqual(['Panel', 'Estación', 'Simulación', 'Impacto'])
      })
    })

    it('el título del documento sigue a la pantalla', async () => {
      simularApi({ 'GET /api/salud': SALUD_OK, 'POST /api/auth/token': { estado: 200, cuerpo: tokenDe('admin') } })
      montar('/ingreso')
      await screen.findByRole('form', { name: 'Ingreso' })
      expect(document.title).toBe('Ingreso · Aire distribuido')

      await ingresarComo('ana', 'otra-clave')
      await screen.findByRole('heading', { name: 'Panel' })
      expect(document.title).toBe('Panel · Aire distribuido')

      await userEvent.click(screen.getByRole('link', { name: 'Clúster' }))
      await screen.findByRole('heading', { name: 'Clúster' })
      expect(document.title).toBe('Clúster · Aire distribuido')
    })

    it.each([
      ['/estacion/CA-AYAS-01', 'Estación · Aire distribuido'],
      ['/muestrario', 'Muestrario de componentes · Aire distribuido'],
      ['/no-existe', 'No existe · Aire distribuido'],
    ])('el título de %s es «%s»', async (ruta, titulo) => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('usuario')

      montar(ruta)

      await waitFor(() => expect(document.title).toBe(titulo))
    })

    it('al navegar, el foco pasa al contenido; al cargar la página, no', async () => {
      simularApi({ 'GET /api/salud': SALUD_OK })
      sesionDe('usuario')
      montar('/', <Link to="/impacto">ir a impacto por fuera del menú</Link>)
      await screen.findByRole('heading', { name: 'Panel' })
      const contenido = screen.getByRole('main')
      expect(contenido).not.toHaveFocus()

      await userEvent.click(within(screen.getByRole('navigation', { name: 'Pantallas' })).getByRole('link', { name: 'Simulación' }))
      await screen.findByRole('heading', { name: 'Simulación' })

      expect(contenido).toHaveFocus()
      expect(contenido).toHaveAttribute('tabindex', '-1')
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
