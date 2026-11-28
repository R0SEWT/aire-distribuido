// Cuánto tarda una lectura que manda un cliente en llegarle a OTRO por el WebSocket (spec del
// frontend, §12): lo que ve quien mira el panel mientras otro simula.
//
// Son dos sesiones distintas, por el mismo origen que la UI (el Caddy del servicio web):
//
//   - el emisor, una cuenta con usuario y clave (por omisión admin, como quien carga el escenario
//     desde Simulación), manda una por una lecturas REALES de web/public/datos/series.json a
//     POST /api/predicciones. No abre ningún WebSocket;
//   - el receptor, el invitado (POST /api/auth/invitado, rol usuario), solo escucha /ws con su
//     token en Sec-WebSocket-Protocol (aire.jwt), como el panel de otra persona. No manda nada.
//
// Por cada lectura mide, desde que el emisor manda el pedido:
//
//   - respuesta: hasta que el emisor tiene el cuerpo de la respuesta;
//   - evento:    hasta que al receptor le llega el estacion.regimen de esa estación con esa
//                fecha_hora. La API lo emite después de que Redis guarda el estado.
//
// Las lecturas van separadas por más de 250 ms, el intervalo con que la API agrupa los eventos de
// una misma estación: así cada lectura tiene su evento. Al final compara el último evento de cada
// día con lo `esperado` del archivo, calculado sin la API (scripts/impacto.py).
//
// Sale con 1 si algún pedido no respondió 200, si a alguna lectura no le llegó su evento o si el
// último estado de algún día no coincide con lo esperado; con 2 si no pudo ni empezar. El informe
// sale por la salida estándar en los tres casos.
//
// Hace falta Node 22 (WebSocket y fetch sin dependencias), una API con el ingreso de invitado
// (API_INVITADO) y los días posteriores al último estado de sus estaciones (un Compose limpio).
// La clave del emisor va en una variable de entorno, no en la línea de comandos:
//
//   EMISOR_CLAVE=… node docs/informe/capturas/tb2/pruebas/medir_ws.mjs [http://localhost:8081] > salida.json
//
// EMISOR_USUARIO cambia la cuenta del emisor (admin por omisión).

import { readFileSync } from 'node:fs'

const BASE = process.argv[2] ?? 'http://localhost:8081'
const EMISOR = process.env.EMISOR_USUARIO ?? 'admin'
const DIAS = ['CA-JUPR-01-2022-05-03', 'CA-CG-02-2022-06-02', 'CA-MAC-01-2022-05-26']
const PAUSA_MS = 350
const PLAZO_PEDIDO_MS = 5_000 // cada fetch, con su cuerpo
const PLAZO_APRETON_MS = 5_000 // el apretón de manos del WebSocket
const PLAZO_EVENTO_MS = 5_000 // la llegada del evento de cada lectura

const series = JSON.parse(readFileSync(new URL('../../../../../web/public/datos/series.json', import.meta.url)))
const espera = (ms) => new Promise((r) => setTimeout(r, ms))
const clave = (estacion, fecha) => `${estacion}|${new Date(fecha).getTime()}`

/** Un pedido con plazo: el mismo AbortSignal corta la conexión y la lectura del cuerpo. */
async function pedir(ruta, opciones = {}) {
  const signal = AbortSignal.timeout(PLAZO_PEDIDO_MS)
  const r = await fetch(`${BASE}${ruta}`, { ...opciones, signal })
  const cuerpo = await r.json()
  return { status: r.status, cuerpo }
}

/** Corta una promesa que tarda más que el plazo. */
function conPlazo(promesa, ms, que) {
  let reloj
  const vencido = new Promise((_, mal) => {
    reloj = setTimeout(() => mal(new Error(`${que}: más de ${ms} ms`)), ms)
  })
  return Promise.race([promesa, vencido]).finally(() => clearTimeout(reloj))
}

function abortar(mensaje) {
  console.error(mensaje)
  process.exit(2)
}

// Las dos sesiones.
const claveEmisor = process.env.EMISOR_CLAVE
if (!claveEmisor) abortar('falta EMISOR_CLAVE: la clave de la cuenta que manda las lecturas')
let emisor, receptor
try {
  emisor = await pedir('/api/auth/token', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ usuario: EMISOR, clave: claveEmisor }),
  })
  receptor = await pedir('/api/auth/invitado', { method: 'POST' })
} catch (e) {
  abortar(`la API no respondió el ingreso en ${BASE}: ${e.name} ${e.message}`)
}
if (emisor.status !== 200) abortar(`el emisor no pudo ingresar: ${emisor.status} ${JSON.stringify(emisor.cuerpo)}`)
if (receptor.status !== 200) abortar(`el receptor no pudo entrar como invitado: ${receptor.status} ${JSON.stringify(receptor.cuerpo)}`)
if (emisor.cuerpo.token === receptor.cuerpo.token) abortar('el emisor y el receptor tienen el mismo token')

// Solo el receptor abre el WebSocket.
const ws = new WebSocket(BASE.replace(/^http/, 'ws') + '/ws', ['aire.jwt', receptor.cuerpo.token])
const llegados = new Map() // estacion|fecha -> [instante, datos]
ws.addEventListener('message', (m) => {
  const ev = JSON.parse(m.data)
  if (ev.evento === 'estacion.regimen') llegados.set(clave(ev.datos.estacion, ev.datos.fecha_hora), [performance.now(), ev.datos])
})
try {
  await conPlazo(
    new Promise((ok, mal) => {
      ws.addEventListener('open', ok, { once: true })
      ws.addEventListener('error', () => mal(new Error('el WebSocket no abrió')), { once: true })
    }),
    PLAZO_APRETON_MS,
    'el apretón de manos del WebSocket',
  )
} catch (e) {
  ws.close()
  abortar(e.message)
}

const medidas = []
const finales = []
for (const id of DIAS) {
  const dia = series.dias.find((d) => d.id === id)
  for (const l of dia.lecturas) {
    const t0 = performance.now()
    let http
    let t1 = null
    try {
      ;({ status: http } = await pedir('/api/predicciones', {
        method: 'POST',
        headers: { Authorization: `Bearer ${emisor.cuerpo.token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify(l),
      }))
      t1 = performance.now()
    } catch (e) {
      http = `sin respuesta (${e.name})`
    }
    const k = clave(l.estacion, l.fecha_hora)
    while (!llegados.has(k) && performance.now() - t0 < PLAZO_EVENTO_MS) await espera(1)
    const ev = llegados.get(k)
    medidas.push({
      estacion: l.estacion,
      fecha_hora: l.fecha_hora,
      http,
      respuesta_ms: t1 === null ? null : t1 - t0,
      evento_ms: ev ? ev[0] - t0 : null,
    })
    await espera(PAUSA_MS)
  }
  const ultimo = llegados.get(clave(dia.estacion, dia.esperado.fecha_hora))?.[1]
  const e = dia.esperado
  const casi = (a, b) => a === b || (a != null && b != null && Math.abs(a - b) < 1e-6)
  const media = (m) => m && { pm2_5: m.pm2_5, pm10: m.pm10, lecturas: m.lecturas }
  finales.push({
    dia: id,
    caso: dia.caso,
    coincide:
      !!ultimo &&
      ultimo.regimen === e.regimen &&
      ultimo.excede_eca === e.excede_eca &&
      ultimo.alerta === e.alerta &&
      ultimo.inca === e.inca &&
      casi(ultimo.media_24h?.pm2_5, e.media_24h?.pm2_5) &&
      casi(ultimo.media_24h?.pm10, e.media_24h?.pm10) &&
      ultimo.media_24h?.lecturas === e.media_24h?.lecturas,
    esperado: { regimen: e.regimen, excede_eca: e.excede_eca, alerta: e.alerta, inca: e.inca, media_24h: media(e.media_24h) },
    evento: ultimo
      ? { regimen: ultimo.regimen, excede_eca: ultimo.excede_eca, alerta: ultimo.alerta, inca: ultimo.inca, media_24h: media(ultimo.media_24h) }
      : null,
  })
}
ws.close()

function resumen(valores) {
  const v = valores.filter((x) => x != null).sort((a, b) => a - b)
  if (v.length === 0) return { n: 0 }
  const p = (q) => v[Math.min(v.length - 1, Math.ceil(q * v.length) - 1)] // rango más cercano
  const r = (x) => Math.round(x * 100) / 100
  return { n: v.length, p50: r(p(0.5)), p95: r(p(0.95)), p99: r(p(0.99)), max: r(v[v.length - 1]) }
}

const informe = {
  metadatos: {
    momento: new Date().toISOString(),
    base: BASE,
    emisor: { usuario: EMISOR, rol: emisor.cuerpo.rol, abre_websocket: false },
    receptor: { usuario: 'invitado (API_INVITADO)', rol: receptor.cuerpo.rol, abre_websocket: true, manda_lecturas: false },
    mismo_token: false,
    node: process.version,
    dias: DIAS,
    pausa_ms: PAUSA_MS,
    plazos_ms: { pedido: PLAZO_PEDIDO_MS, apreton: PLAZO_APRETON_MS, evento: PLAZO_EVENTO_MS },
  },
  lecturas: medidas.length,
  errores_http: medidas.filter((m) => m.http !== 200).length,
  sin_evento: medidas.filter((m) => m.evento_ms == null).length,
  finales_que_no_coinciden: finales.filter((f) => !f.coincide).length,
  respuesta_ms: resumen(medidas.map((m) => m.respuesta_ms)),
  evento_ms: resumen(medidas.map((m) => m.evento_ms)),
  finales,
}
informe.cumple = informe.errores_http === 0 && informe.sin_evento === 0 && informe.finales_que_no_coinciden === 0
console.log(JSON.stringify(informe, null, 2))
process.exitCode = informe.cumple ? 0 : 1
