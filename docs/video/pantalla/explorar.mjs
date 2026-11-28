// Explorador: entra como admin y deja, por cada ruta, una captura y el árbol de accesibilidad (YAML)
// en docs/video/build/tb2/pantalla/explorar/. Sirve para escribir los pasos de grabar.mjs con los
// nombres reales de los botones y campos. No escribe nada en la API.
//
//   node docs/video/pantalla/explorar.mjs / /simulacion /entrenamiento /cluster /impacto
//   node docs/video/pantalla/explorar.mjs '/simulacion#Un día de una estación'   # y hace clic en ese botón

import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { abrir, BASE, cuentas, ingresar, SALIDA } from './comun.mjs'

const rutas = process.argv.slice(2)
if (rutas.length === 0) rutas.push('/')
const dir = join(SALIDA, 'explorar')
mkdirSync(dir, { recursive: true })

const { navegador, pagina } = await abrir()
try {
  await ingresar(pagina, cuentas().admin)
  for (const pedido of rutas) {
    const [ruta, boton] = pedido.split('#')
    await pagina.goto(`${BASE}${ruta}`)
    await pagina.waitForLoadState('networkidle').catch(() => {})
    await pagina.waitForTimeout(1500)
    if (boton) {
      await pagina.getByRole('button', { name: boton, exact: true }).click()
      await pagina.waitForTimeout(1000)
    }
    const nombre = pedido.normalize('NFD').replace(/[\u0300-\u036f]/g, '').replace(/[^a-z0-9]+/gi, '-').replace(/^-|-$/g, '') || 'panel'
    await pagina.screenshot({ path: join(dir, `${nombre}.png`), fullPage: true })
    writeFileSync(join(dir, `${nombre}.yml`), await pagina.locator('body').ariaSnapshot())
    console.log(`explorada: ${ruta} → ${join(dir, nombre)}.{png,yml}`)
  }
} finally {
  await navegador.close()
}
