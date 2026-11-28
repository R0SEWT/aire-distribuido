// Graba la SPA de punta a punta para el video del TB2 (docs/video/guion-tb2.md), con Playwright y el
// Chrome del sistema, en docs/video/build/tb2/pantalla/NOMBRE.mp4 (1920×1080, 30 fps). El montaje
// (montar_tb2.py) toma cada grabación por su nombre en la línea `Pantalla: pantalla NOMBRE`.
//
//   node docs/video/pantalla/grabar.mjs                    # todas, en orden, contra el Compose local
//   node docs/video/pantalla/grabar.mjs impacto cluster    # solo esas
//   AIRE_URL=https://aire.rosewt.dev node docs/video/pantalla/grabar.mjs impacto   # el sitio público
//
// Contra el Compose local (AIRE_URL por omisión, http://localhost:8091) las grabaciones escriben: el
// escenario y la simulación mandan lecturas y el entrenamiento cambia el modelo activo. Por eso el
// escenario exige un sistema limpio (`docker compose down -v` y arriba de nuevo; grabar-pantalla.sh
// lo hace) y el orden importa: escenario, simulación, entrenamiento y clúster. Contra un sitio que no
// es local solo se graban las de lectura (impacto y swagger) y se entra como invitado.
//
// Las cuentas salen del .env del repo (API_USUARIOS) o de AIRE_ADMIN=usuario:clave; no se imprimen.

import { abrir, BASE, clic, cuentas, esLocal, grabar, ingresar, moverA } from './comun.mjs'

const pausa = (pagina, s) => pagina.waitForTimeout(s * 1000)

/** Desliza la página hasta que el elemento quede con su borde de arriba a `margen` px, en `ms`. */
async function deslizarA(pagina, locator, { margen = 90, ms = 1200 } = {}) {
  const caja = await locator.boundingBox()
  if (!caja) return
  await pagina.evaluate(async ({ dy, ms }) => {
    const y0 = window.scrollY
    const t0 = performance.now()
    await new Promise((listo) => {
      const paso = (t) => {
        const p = Math.min(1, (t - t0) / ms)
        const s = p < 0.5 ? 2 * p * p : 1 - (-2 * p + 2) ** 2 / 2
        window.scrollTo(0, y0 + dy * s)
        if (p < 1) requestAnimationFrame(paso)
        else listo()
      }
      requestAnimationFrame(paso)
    })
  }, { dy: caja.y - margen, ms })
}

const seccion = (pagina, nombre) => pagina.getByRole('heading', { name: nombre })

// ----------------------------------------------------------------------------- escenas

const ESCENAS = {
  /** Bloque 05: el contrato en Swagger UI, servido por la API en /docs. Solo lectura. */
  async swagger(pagina) {
    await pagina.goto(`${BASE}/docs`)
    await pagina.locator('.swagger-ui .opblock').first().waitFor({ timeout: 90_000 })  // Swagger UI viene de un CDN
    // Swagger UI escribe chico: se agranda para que se lea en el video.
    await pagina.evaluate(() => { document.body.style.zoom = '1.35' })
    await grabar(pagina, 'swagger', async () => {
      await pausa(pagina, 2.5)
      const prediccion = pagina.locator('.opblock-post').filter({ hasText: '/api/predicciones' }).first()
      await deslizarA(pagina, prediccion, { margen: 160, ms: 2200 })
      await pausa(pagina, 1)
      await clic(pagina, prediccion.locator('.opblock-summary'))
      await pausa(pagina, 1.5)
      await deslizarA(pagina, prediccion.locator('.opblock-body'), { margen: 120, ms: 2000 })
      await pausa(pagina, 3)
      await pagina.mouse.wheel(0, 500)
      await pausa(pagina, 3)
    })
  },

  /** Bloque 07: ingreso, panel vacío, el escenario y el panel que se llena por WebSocket. */
  async escenario(pagina) {
    const { admin } = cuentas()
    await pagina.goto(`${BASE}/ingreso`)
    await pagina.getByLabel('Usuario').waitFor()
    await grabar(pagina, 'escenario', async () => {
      await pausa(pagina, 1.2)
      await ingresar(pagina, admin, { lento: true })
      const vacio = pagina.getByText('Ninguna estación tiene estado todavía')
      if (!(await vacio.isVisible().catch(() => false))) {
        await vacio.waitFor({ timeout: 4000 }).catch(() => {
          throw new Error('el panel no está vacío: el escenario se graba sobre un sistema limpio (docker compose down -v)')
        })
      }
      await pausa(pagina, 2.5)
      await clic(pagina, pagina.getByRole('button', { name: 'Ir a cargar el escenario' }))
      await pagina.getByRole('button', { name: 'Cargar el escenario' }).waitFor()
      await pausa(pagina, 2)
      await clic(pagina, pagina.getByRole('button', { name: 'Cargar el escenario' }))
      await pausa(pagina, 2.5)
      // La corrida sigue en la pestaña: en el panel se ve llegar cada estado por el WebSocket.
      await clic(pagina, pagina.getByRole('navigation', { name: 'Pantallas' }).getByRole('link', { name: 'Panel' }))
      await pausa(pagina, 9)
      await deslizarA(pagina, pagina.locator('#lista-de-estaciones'), { margen: 70, ms: 1800 })
      await pausa(pagina, 3.5)
    })
  },

  /** Bloque 08: un día de una estación que cruza el ECA, lectura por lectura. */
  async simulacion(pagina) {
    await pagina.goto(`${BASE}/simulacion`)
    await pagina.getByRole('button', { name: 'Un día de una estación', exact: true }).waitFor()
    // Si el escenario sigue corriendo (se grabó recién), se espera a que termine.
    await pagina.getByRole('button', { name: 'Un día de una estación', exact: true }).isEnabled()
    await pagina.waitForFunction(
      () => ![...document.querySelectorAll('button')].some((b) => b.textContent?.trim() === 'Pausar'),
      null, { timeout: 60_000 },
    )
    await grabar(pagina, 'simulacion', async () => {
      await pausa(pagina, 1)
      await clic(pagina, pagina.getByRole('button', { name: 'Un día de una estación', exact: true }))
      await pausa(pagina, 1)
      const dia = pagina.getByRole('combobox', { name: 'Estación y día' })
      await moverA(pagina, dia)
      const opcion = await dia.locator('option').filter({ hasText: 'Cruza el ECA' }).first().textContent()
      await dia.selectOption({ label: opcion.trim() })
      await pausa(pagina, 2)
      await clic(pagina, pagina.getByRole('button', { name: 'Simular este día' }))
      await pausa(pagina, 0.8)
      await deslizarA(pagina, pagina.getByText(/^Progreso · /), { margen: 80, ms: 1500 })
      await pagina.getByText('Terminó', { exact: true }).first().waitFor({ timeout: 40_000 }).catch(() => {})
      await pausa(pagina, 2.5)
    })
  },

  /** Bloque 09: un entrenamiento con los tres nodos y sus rondas reproducidas. */
  async entrenamiento(pagina) {
    await pagina.goto(`${BASE}/entrenamiento`)
    const lanzar = pagina.getByRole('button', { name: 'Lanzar entrenamiento' })
    await lanzar.waitFor()
    await pagina.waitForFunction(() => document.body.innerText.includes('3 nodos'), null, { timeout: 30_000 })
    await grabar(pagina, 'entrenamiento', async () => {
      await pausa(pagina, 1.5)
      await clic(pagina, lanzar)
      await pausa(pagina, 1.2)
      await deslizarA(pagina, seccion(pagina, 'Rondas del entrenamiento'), { margen: 80, ms: 1200 })
      await seccion(pagina, 'Resultado').waitFor({ timeout: 40_000 })
      await pausa(pagina, 1)
      await deslizarA(pagina, seccion(pagina, 'Resultado'), { margen: 80, ms: 1200 })
      await pausa(pagina, 3)
    })
  },

  /** Bloque 09: el panel del administrador con nodos, CPU, latencia por ruta y eventos. */
  async cluster(pagina) {
    await pagina.goto(`${BASE}/cluster`)
    await seccion(pagina, 'Nodos del pool').waitFor()
    await pausa(pagina, 1.5)
    await grabar(pagina, 'cluster', async () => {
      await pausa(pagina, 2.5)
      await deslizarA(pagina, seccion(pagina, 'Uso de CPU y memoria'), { margen: 90, ms: 1500 })
      await pausa(pagina, 2)
      await deslizarA(pagina, seccion(pagina, 'Latencia de la API por ruta'), { margen: 90, ms: 1500 })
      await pausa(pagina, 2.5)
      await deslizarA(pagina, seccion(pagina, /^Eventos/), { margen: 90, ms: 1500 })
      await pausa(pagina, 2.5)
    })
  },

  /** Bloque 10: la página de impacto, en el orden en que se narra. Solo lectura. */
  async impacto(pagina) {
    await pagina.goto(`${BASE}/impacto`)
    await seccion(pagina, 'Las horas en que la media de 24 horas superó el ECA').waitFor()
    await pausa(pagina, 1.5)
    await grabar(pagina, 'impacto', async () => {
      await pausa(pagina, 1.5)
      await deslizarA(pagina, seccion(pagina, 'Las horas en que la media de 24 horas superó el ECA'), { margen: 80, ms: 1500 })
      await pausa(pagina, 6)
      await deslizarA(pagina, seccion(pagina, 'Quiénes tendrían un aviso que hoy no tienen'), { margen: 80, ms: 2200 })
      await pausa(pagina, 6)
      await deslizarA(pagina, seccion(pagina, 'Depende de cómo se cuente: las dos medidas van juntas'), { margen: 80, ms: 2200 })
      await pausa(pagina, 11)
      await deslizarA(pagina, seccion(pagina, 'Lo que este dato no puede decir'), { margen: 80, ms: 2000 })
      await pausa(pagina, 7)
    })
  },
}

const ORDEN = ['swagger', 'escenario', 'simulacion', 'entrenamiento', 'cluster', 'impacto']
const SOLO_LECTURA = new Set(['swagger', 'impacto'])
const NECESITAN_ADMIN = new Set(['escenario', 'simulacion', 'entrenamiento', 'cluster'])

const pedidas = process.argv.slice(2)
for (const p of pedidas) if (!ESCENAS[p]) throw new Error(`escena desconocida: ${p} (hay: ${ORDEN.join(', ')})`)
let lista = ORDEN.filter((e) => pedidas.length === 0 || pedidas.includes(e))
if (!esLocal()) {
  const fuera = lista.filter((e) => !SOLO_LECTURA.has(e))
  if (fuera.length) console.log(`${BASE} no es local: solo lectura, se saltan ${fuera.join(', ')}`)
  lista = lista.filter((e) => SOLO_LECTURA.has(e))
}

const { navegador, pagina } = await abrir()
try {
  // Para las de solo lectura basta el invitado si la API lo tiene; para las otras, admin.
  let adentro = false
  for (const escena of lista) {
    if (escena !== 'escenario' && escena !== 'swagger' && !adentro) {
      if (NECESITAN_ADMIN.has(escena) || esLocal()) {
        await ingresar(pagina, cuentas().admin)
      } else {
        await pagina.goto(`${BASE}/ingreso`)
        await clic(pagina, pagina.getByRole('button', { name: 'No pienso crearme una cuenta aquí' }))
        await clic(pagina, pagina.getByRole('dialog').getByRole('button').filter({ hasText: /entrar/i }).first())
        await pagina.waitForURL((u) => !u.pathname.startsWith('/ingreso'))
      }
      adentro = true
    }
    console.log(`escena ${escena}`)
    await ESCENAS[escena](pagina)
    if (escena === 'escenario') adentro = true
  }
} finally {
  await navegador.close()
}
