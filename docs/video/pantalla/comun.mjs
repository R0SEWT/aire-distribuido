// Lo que comparten el grabador y el explorador: abrir Chrome, entrar a la SPA y grabar la pantalla
// con el screencast de Chrome (CDP) a 1920×1080, más nítido que el recordVideo de Playwright.

import { spawnSync } from 'node:child_process'
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { basename, dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright-core'

const aqui = dirname(fileURLToPath(import.meta.url))
export const RAIZ = resolve(aqui, '../../..')
export const SALIDA = join(RAIZ, 'docs/video/build/tb2/pantalla')

export const BASE = (process.env.AIRE_URL ?? 'http://localhost:8091').replace(/\/$/, '')
// 1536×864 a 1,25 = 1920×1080 píxeles: la SPA se ve como en una laptop y el texto queda nítido.
export const VISTA = { width: 1536, height: 864 }
export const ESCALA = 1.25
const FFMPEG = process.env.FFMPEG ?? 'ffmpeg'
// Con exodia-ff (skill ffmpeg-exodia) el mp4 se arma en exodia y aquí no se codifica nada;
// FFMPEG_LOCAL=1 lo arma en esta máquina.
const EN_EXODIA = process.env.FFMPEG_LOCAL !== '1' && spawnSync('sh', ['-c', 'command -v exodia-ff']).status === 0

/** ¿La URL es un Compose local (donde se puede escribir) o un sitio público (solo lectura)? */
export function esLocal(url = BASE) {
  const h = new URL(url).hostname
  return h === 'localhost' || h === '127.0.0.1' || h === '[::1]' || h.endsWith('.localhost') || h.endsWith('.test')
}

/**
 * Las cuentas salen de AIRE_ADMIN y AIRE_USUARIO («usuario:clave») o del .env del repo (API_USUARIOS, el
 * mismo que lee el Compose). Nunca se imprimen.
 */
export function cuentas() {
  const leer = (v) => {
    if (!v) return undefined
    const [usuario, ...resto] = v.split(':')
    return { usuario, clave: resto.join(':') }
  }
  let admin = leer(process.env.AIRE_ADMIN)
  let usuario = leer(process.env.AIRE_USUARIO)
  const env = join(RAIZ, '.env')
  if ((!admin || !usuario) && existsSync(env)) {
    const linea = readFileSync(env, 'utf8').split('\n').find((l) => l.startsWith('API_USUARIOS='))
    for (const c of (linea ?? '').slice('API_USUARIOS='.length).split(',')) {
      const [u, clave, rol] = c.trim().split(':')
      if (!u || !clave) continue
      if (rol === 'admin' && !admin) admin = { usuario: u, clave }
      if (rol === 'usuario' && !usuario) usuario = { usuario: u, clave }
    }
  }
  return { admin, usuario }
}

export async function abrir() {
  const navegador = await chromium.launch({
    executablePath: process.env.CHROME ?? '/usr/bin/google-chrome',
    headless: true,
    args: ['--hide-scrollbars', '--font-render-hinting=none'],
  })
  const contexto = await navegador.newContext({
    viewport: VISTA,
    deviceScaleFactor: ESCALA,
    locale: 'es-PE',
    timezoneId: 'America/Lima',
    colorScheme: 'dark',
    reducedMotion: 'no-preference',
    // La CSP de Caddy no deja estilos en línea: el cursor falso de abajo los necesita.
    bypassCSP: true,
  })
  // El modo proyector de la SPA (letra más grande y más contraste, spec del frontend §8): en un video a
  // 1080p es lo que se lee. Se guarda en localStorage con la clave de web/src/armazon/modo.ts.
  if (process.env.PROYECTOR !== '0') {
    await contexto.addInitScript(() => {
      try { localStorage.setItem('aire.modo', 'proyector') } catch { /* sin almacenamiento */ }
    })
  }
  const pagina = await contexto.newPage()
  pagina.setDefaultTimeout(20_000)
  return { navegador, contexto, pagina }
}

/** Entra con usuario y clave por la pantalla de ingreso, como lo haría una persona. */
export async function ingresar(pagina, cuenta, { lento = false } = {}) {
  if (!cuenta) throw new Error('falta la cuenta: AIRE_ADMIN=usuario:clave o API_USUARIOS en el .env del repo')
  if (!new URL(pagina.url() === 'about:blank' ? BASE : pagina.url()).pathname.startsWith('/ingreso')) {
    await pagina.goto(`${BASE}/ingreso`)
  }
  await pagina.getByLabel('Usuario').waitFor()
  await cursor(pagina)
  await escribir(pagina, pagina.getByLabel('Usuario'), cuenta.usuario, lento)
  await escribir(pagina, pagina.getByLabel('Clave'), cuenta.clave, lento)
  await clic(pagina, pagina.getByRole('button', { name: 'Ingresar' }))
  await pagina.waitForURL((u) => !u.pathname.startsWith('/ingreso'))
}

// ----------------------------------------------------------------------------- cursor falso

/**
 * El screencast no trae el puntero del sistema: se dibuja uno en la página y se mueve hasta cada
 * elemento antes de hacer clic, para que se vea qué se toca.
 */
export async function cursor(pagina) {
  await pagina.evaluate(() => {
    if (document.getElementById('cursor-video')) return
    const c = document.createElement('div')
    c.id = 'cursor-video'
    Object.assign(c.style, {
      position: 'fixed', left: '0px', top: '0px', width: '22px', height: '22px', marginLeft: '-11px',
      marginTop: '-11px', borderRadius: '50%', background: 'rgba(255,255,255,0.85)',
      border: '3px solid rgba(42,120,214,0.95)', boxShadow: '0 0 0 6px rgba(42,120,214,0.25)',
      zIndex: '2147483647', pointerEvents: 'none', transition: 'left 0.45s ease, top 0.45s ease, transform 0.12s',
    })
    c.style.left = `${window.innerWidth * 0.55}px`
    c.style.top = `${window.innerHeight * 0.6}px`
    document.body.appendChild(c)
  })
}

export async function moverA(pagina, locator) {
  await locator.scrollIntoViewIfNeeded()
  const caja = await locator.boundingBox()
  if (!caja) return
  await cursor(pagina)
  await pagina.evaluate(({ x, y }) => {
    const c = document.getElementById('cursor-video')
    if (c) { c.style.left = `${x}px`; c.style.top = `${y}px` }
  }, { x: caja.x + caja.width / 2, y: caja.y + caja.height / 2 })
  await pagina.waitForTimeout(500)
}

export async function clic(pagina, locator) {
  await moverA(pagina, locator)
  await pagina.evaluate(() => {
    const c = document.getElementById('cursor-video')
    if (c) { c.style.transform = 'scale(0.7)'; setTimeout(() => { c.style.transform = 'scale(1)' }, 150) }
  })
  await locator.click()
}

export async function escribir(pagina, locator, texto, lento) {
  await clic(pagina, locator)
  await locator.fill('')
  await locator.pressSequentially(texto, { delay: lento ? 70 : 25 })
}

// ----------------------------------------------------------------------------- grabación

/**
 * Graba con Page.startScreencast: Chrome manda un cuadro JPEG cada vez que la pantalla cambia, con su
 * marca de tiempo. Al terminar, ffmpeg arma un mp4 a 30 fps que respeta esas marcas.
 */
export async function grabar(pagina, nombre, accion) {
  mkdirSync(SALIDA, { recursive: true })
  const cdp = await pagina.context().newCDPSession(pagina)
  const cuadros = []
  cdp.on('Page.screencastFrame', async (f) => {
    cuadros.push({ t: f.metadata.timestamp, datos: f.data })
    try { await cdp.send('Page.screencastFrameAck', { sessionId: f.sessionId }) } catch { /* ya se cerró */ }
  })
  await cdp.send('Page.startScreencast', {
    format: 'jpeg', quality: 92, maxWidth: VISTA.width * ESCALA, maxHeight: VISTA.height * ESCALA, everyNthFrame: 1,
  })
  const inicio = Date.now() / 1000
  try {
    await accion()
  } finally {
    // Un último cuadro para fijar el final: si la pantalla no cambió, Chrome no manda nada.
    await pagina.waitForTimeout(400)
    await cdp.send('Page.stopScreencast')
  }
  const fin = Date.now() / 1000
  if (cuadros.length === 0) throw new Error(`${nombre}: el screencast no mandó cuadros`)
  const dir = join(SALIDA, `${nombre}-cuadros`)
  rmSync(dir, { recursive: true, force: true })
  mkdirSync(dir, { recursive: true })
  // La lista lleva solo el nombre de cada cuadro: ffmpeg los busca junto a ella, aquí o en exodia.
  const lineas = []
  cuadros.forEach((c, i) => {
    const archivo = `${String(i).padStart(6, '0')}.jpg`
    writeFileSync(join(dir, archivo), Buffer.from(c.datos, 'base64'))
    const siguiente = i + 1 < cuadros.length ? cuadros[i + 1].t : Math.max(fin, c.t + 0.04)
    lineas.push(`file '${archivo}'`, `duration ${Math.max(0.001, siguiente - c.t).toFixed(4)}`)
  })
  lineas.push(`file '${String(cuadros.length - 1).padStart(6, '0')}.jpg'`)
  writeFileSync(join(dir, 'lista.txt'), lineas.join('\n') + '\n')
  const mp4 = join(SALIDA, `${nombre}.mp4`)
  const filtro = ['-vf', 'scale=1920:1080:flags=lanczos,fps=30,format=yuv420p', '-c:v', 'libx264', '-preset', 'medium',
    '-crf', '18', '-movflags', '+faststart']
  const r = EN_EXODIA
    ? spawnSync('exodia-ff', ['run', '-w', '-n', `pantalla-${nombre}`, '--in', dir, '-f', SALIDA, '--sh',
      `ffmpeg -nostdin -y -loglevel error -f concat -safe 0 -i in/${basename(dir)}/lista.txt ${filtro.join(' ')} out/${nombre}.mp4`],
    { stdio: 'inherit' })
    : spawnSync(FFMPEG, ['-y', '-loglevel', 'error', '-f', 'concat', '-safe', '0', '-i', join(dir, 'lista.txt'), ...filtro, mp4],
      { stdio: 'inherit' })
  if (r.status !== 0 || !existsSync(mp4)) throw new Error(`${nombre}: no se armó el mp4 (${EN_EXODIA ? 'exodia-ff' : 'ffmpeg'})`)
  rmSync(dir, { recursive: true, force: true })
  const segundos = (cuadros.at(-1).t - cuadros[0].t).toFixed(1)
  console.log(`grabada: ${mp4} (${cuadros.length} cuadros, ${segundos} s de pantalla, ${(fin - inicio).toFixed(1)} s en total)`)
  return mp4
}
