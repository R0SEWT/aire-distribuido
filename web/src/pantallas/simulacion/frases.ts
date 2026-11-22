// Lo que la marcha de una simulación dice con palabras, sin React: la frase del
// escenario, la de un día suelto y por cuál contaminante se encendió la alerta.

import type { ReglasDeSeries } from '../../api/datos'
import type { LecturaVista } from '../../api/vistas'
import { enumerar, fechaHora, numero, type Tono } from '../../componentes/indice'
import type { Corrida, Seguimiento } from './motor'
import type { Tramo } from './series'

export function fraseDelEscenario(tramo: Tramo, corrida: Corrida, reglas: ReglasDeSeries): { tono: Tono; texto: string } {
  const seguimientos = Object.values(corrida.porEstacion)
  const conMedia = seguimientos.filter((s) => s.ultima?.inca).length
  const enAlerta = seguimientos.filter((s) => s.ultima?.alerta).map((s) => s.estacion)
  const n = tramo.estaciones.length
  const alertas =
    enAlerta.length === 0
      ? 'ninguna está en alerta'
      : `${enAlerta.length === 1 ? 'está en alerta' : 'están en alerta'} ${enumerar(enAlerta)}`
  if (conMedia === 0) {
    return {
      tono: 'acento',
      texto: `Todavía ninguna estación tiene media de 24 horas: la API la da recién cuando la ventana junta ${reglas.minimo_lecturas} lecturas.`,
    }
  }
  // «Su media supera el ECA» solo se dice de las que están en alerta: si no hay ninguna, sería falso.
  const porque = enAlerta.length === 0 ? 'ninguna media supera el ECA' : 'su media supera el ECA'
  return {
    tono: enAlerta.length > 0 ? 'alerta' : 'buena',
    texto: `${conMedia} de ${n} estaciones ${conMedia === 1 ? 'tiene' : 'tienen'} media de 24 horas y ${alertas}: ${porque}.`,
  }
}

export function fraseDelDia(s: Seguimiento | undefined, reglas: ReglasDeSeries): { tono: Tono; texto: string } {
  const r = s?.ultima
  if (!s || !r) return { tono: 'acento', texto: 'Todavía no se mandó ninguna lectura.' }
  if (r.media_24h.pm2_5 === null || r.media_24h.pm10 === null) {
    const faltan = reglas.minimo_lecturas - r.media_24h.lecturas
    return {
      tono: 'acento',
      texto: `La ventana de 24 horas tiene ${r.media_24h.lecturas} ${r.media_24h.lecturas === 1 ? 'lectura' : 'lecturas'}: ${
        faltan === 1 ? 'falta 1' : `faltan ${faltan}`
      } para que la API calcule la media. Sin media no hay alerta ni banda INCA.`,
    }
  }
  const medias = `La media de las últimas 24 horas es ${numero(r.media_24h.pm2_5, 1)} µg/m³ de PM2.5 y ${numero(r.media_24h.pm10, 1)} µg/m³ de PM10`
  if (!r.alerta) {
    return { tono: 'buena', texto: `${medias}: no supera el ECA (${reglas.eca_pm2_5} y ${reglas.eca_pm10}).` }
  }
  const cual = [
    r.media_24h.pm2_5 > reglas.eca_pm2_5 ? `el de PM2.5 (${reglas.eca_pm2_5})` : null,
    r.media_24h.pm10 > reglas.eca_pm10 ? `el de PM10 (${reglas.eca_pm10})` : null,
  ].filter((x): x is string => x !== null)
  const desde = s.primeraAlerta ? ` Hay alerta desde el ${fechaHora(s.primeraAlerta)}.` : ''
  return { tono: 'alerta', texto: `${medias}: supera el ECA, ${enumerar(cual)}.${desde}` }
}

/**
 * Por cuál contaminante se encendió la alerta de una estación: los que tenían
 * la media de 24 horas sobre su ECA en la lectura en que la API respondió
 * `alerta` por primera vez. La alerta es de la estación, no de un contaminante:
 * la curva de PM2.5 no tiene que dar a entender que fue por PM2.5 si fue por PM10.
 */
export function causaDeLaAlerta(puntos: readonly LecturaVista[], primeraAlerta: string | null, reglas: ReglasDeSeries): string[] {
  if (primeraAlerta === null) return []
  const t = Date.parse(primeraAlerta)
  const punto = puntos.find((p) => Date.parse(p.fecha_hora) === t)
  if (!punto) return []
  return [
    punto.media_pm2_5 !== null && punto.media_pm2_5 > reglas.eca_pm2_5 ? 'PM2.5' : null,
    punto.media_pm10 !== null && punto.media_pm10 > reglas.eca_pm10 ? 'PM10' : null,
  ].filter((x): x is string => x !== null)
}
