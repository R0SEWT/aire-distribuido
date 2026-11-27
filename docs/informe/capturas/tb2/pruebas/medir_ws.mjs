// Cuánto tarda una lectura en llegar al panel por el WebSocket (spec del frontend, §12).
//
// Hace lo mismo que la UI, por el mismo origen (el Caddy del servicio web): entra como
// invitado, abre /ws con el token en Sec-WebSocket-Protocol (aire.jwt) y manda, una por una,
// lecturas REALES de web/public/datos/series.json a POST /api/predicciones. Por cada lectura
// mide dos tiempos desde que sale el pedido:
//
//   - respuesta: hasta tener el cuerpo de la respuesta (lo que ve quien simula);
//   - evento:    hasta que llega el estacion.regimen de esa estación con esa fecha_hora
//                (lo que ve otro que mira el panel). Sale después de que Redis guarda el estado.
//
// Las lecturas van separadas por más de 250 ms, el intervalo con que la API agrupa los eventos
// de una misma estación: así cada lectura tiene su evento. Al final compara el último evento de
// cada día con lo `esperado` del archivo, calculado sin la API (scripts/impacto.py).
//
// Hace falta Node 22 (WebSocket y fetch sin dependencias) y una API con el ingreso de invitado
// (API_INVITADO). Los días tienen que ser posteriores al último estado de sus estaciones.
//
//   node docs/informe/capturas/tb2/pruebas/medir_ws.mjs [http://localhost:8081] > salida.json

import { readFileSync } from 'node:fs'

const BASE = process.argv[2] ?? 'http://localhost:8081'
const DIAS = ['CA-JUPR-01-2022-05-03', 'CA-CG-02-2022-06-02', 'CA-MAC-01-2022-05-26']
const PAUSA_MS = 350

const series = JSON.parse(readFileSync(new URL('../../../../../web/public/datos/series.json', import.meta.url)))
const espera = (ms) => new Promise((r) => setTimeout(r, ms))
const clave = (estacion, fecha) => `${estacion}|${new Date(fecha).getTime()}`

const { token, rol } = await (await fetch(`${BASE}/api/auth/invitado`, { method: 'POST' })).json()
const ws = new WebSocket(BASE.replace(/^http/, 'ws') + '/ws', ['aire.jwt', token])
const llegados = new Map() // estacion|fecha -> [instante, datos]
ws.addEventListener('message', (m) => {
  const ev = JSON.parse(m.data)
  if (ev.evento === 'estacion.regimen') llegados.set(clave(ev.datos.estacion, ev.datos.fecha_hora), [performance.now(), ev.datos])
})
await new Promise((ok, mal) => {
  ws.addEventListener('open', ok, { once: true })
  ws.addEventListener('error', () => mal(new Error('el WebSocket no abrió')), { once: true })
})

const medidas = []
const finales = []
for (const id of DIAS) {
  const dia = series.dias.find((d) => d.id === id)
  for (const l of dia.lecturas) {
    const t0 = performance.now()
    const r = await fetch(`${BASE}/api/predicciones`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify(l),
    })
    await r.json()
    const t1 = performance.now()
    const k = clave(l.estacion, l.fecha_hora)
    while (!llegados.has(k) && performance.now() - t0 < 5000) await espera(1)
    const ev = llegados.get(k)
    medidas.push({ estacion: l.estacion, fecha_hora: l.fecha_hora, http: r.status, respuesta_ms: t1 - t0, evento_ms: ev ? ev[0] - t0 : null })
    await espera(PAUSA_MS)
  }
  const ultimo = llegados.get(clave(dia.estacion, dia.esperado.fecha_hora))?.[1]
  const e = dia.esperado
  const casi = (a, b) => a === b || (a != null && b != null && Math.abs(a - b) < 1e-6)
  finales.push({
    dia: id,
    caso: dia.caso,
    coincide:
      !!ultimo &&
      ultimo.regimen === e.regimen &&
      ultimo.alerta === e.alerta &&
      ultimo.inca === e.inca &&
      casi(ultimo.media_24h?.pm2_5, e.media_24h?.pm2_5) &&
      casi(ultimo.media_24h?.pm10, e.media_24h?.pm10),
    esperado: { regimen: e.regimen, alerta: e.alerta, inca: e.inca, media_24h: e.media_24h },
    evento: ultimo ? { regimen: ultimo.regimen, alerta: ultimo.alerta, inca: ultimo.inca, media_24h: ultimo.media_24h } : null,
  })
}
ws.close()

function resumen(valores) {
  const v = valores.filter((x) => x != null).sort((a, b) => a - b)
  const p = (q) => v[Math.min(v.length - 1, Math.ceil(q * v.length) - 1)] // rango más cercano
  const r = (x) => Math.round(x * 100) / 100
  return { n: v.length, p50: r(p(0.5)), p95: r(p(0.95)), p99: r(p(0.99)), max: r(v[v.length - 1]) }
}

console.log(
  JSON.stringify(
    {
      metadatos: { momento: new Date().toISOString(), base: BASE, rol, node: process.version, dias: DIAS, pausa_ms: PAUSA_MS },
      lecturas: medidas.length,
      errores_http: medidas.filter((m) => m.http !== 200).length,
      sin_evento: medidas.filter((m) => m.evento_ms == null).length,
      respuesta_ms: resumen(medidas.map((m) => m.respuesta_ms)),
      evento_ms: resumen(medidas.map((m) => m.evento_ms)),
      finales,
    },
    null,
    2,
  ),
)
