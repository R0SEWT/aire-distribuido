// Las reglas de presentación del Panel (spec §5.2 y §12): en cuál de los tres
// estados está cada estación, el orden por gravedad, las cifras de arriba y la
// frase. No calcula nada del modelo: la alerta, la banda y el régimen son los
// que manda la API.

import type { EstadoEstacion } from '../../api/enVivo'
import { BANDAS, estadoDeBanda, type EstadoBanda, type Tono } from '../../componentes/indice'
import type { Estacion, RegimenEstacion } from './datos'

/** Las lecturas que la ventana de 24 horas necesita para dar una media (contrato, Media24h). */
export const MINIMO_LECTURAS = 18

/**
 * Los tres estados de una estación en el panel:
 * - `con_estado`: tiene media de 24 horas y banda INCA.
 * - `sin_datos`: mide PM por hora, pero todavía no recibió 18 lecturas en la ventana (o ninguna).
 * - `no_mide`: no mide PM por hora; nunca va a tener estado.
 */
export type Situacion = 'con_estado' | 'sin_datos' | 'no_mide'

export interface FilaEstacion {
  id: string
  estacion: Estacion
  situacion: Situacion
  /** La banda INCA, o cuál de los dos estados sin banda: para <Banda> y para el color. */
  banda: EstadoBanda
  alerta: boolean
  /** El último estado recibido; null si todavía no recibió lecturas o no mide. */
  estado: RegimenEstacion | null
}

/**
 * La fila de una estación. Si la API tiene un estado de ella, manda ese estado
 * aunque la ficha diga que no mide PM por hora.
 */
export function filaDe(estacion: Estacion, estado: RegimenEstacion | null | undefined, midePM: boolean): FilaEstacion {
  const comun = { id: estacion.id, estacion, estado: estado ?? null }
  if (estado) {
    const banda = estadoDeBanda(estado.inca)
    return { ...comun, situacion: banda === 'sin_datos' ? 'sin_datos' : 'con_estado', banda, alerta: estado.alerta }
  }
  return midePM
    ? { ...comun, situacion: 'sin_datos', banda: 'sin_datos', alerta: false }
    : { ...comun, situacion: 'no_mide', banda: 'no_mide', alerta: false }
}

/** Las filas de todo el catálogo, ya ordenadas por gravedad. */
export function filasDe(
  estaciones: readonly Estacion[],
  estados: readonly EstadoEstacion[],
  sinPMHorario: ReadonlySet<string>,
): FilaEstacion[] {
  const porCodigo = new Map(estados.map((e) => [e.estacion, e]))
  return estaciones.map((e) => filaDe(e, porCodigo.get(e.id), !sinPMHorario.has(e.id))).sort(porGravedad)
}

/**
 * El orden de la lista: primero las que están en alerta, después de la banda
 * INCA peor a la mejor, después las que no tienen datos suficientes y al final
 * las que no miden. A igual gravedad, por código.
 */
export function porGravedad(a: FilaEstacion, b: FilaEstacion): number {
  return (
    Number(b.alerta) - Number(a.alerta) ||
    BANDAS[b.banda].gravedad - BANDAS[a.banda].gravedad ||
    // Entre dos sin datos suficientes, primero la que ya recibió lecturas.
    Number(b.estado !== null) - Number(a.estado !== null) ||
    a.id.localeCompare(b.id)
  )
}

export interface Resumen {
  /** Estaciones cuya media de 24 horas supera el ECA. */
  enAlerta: number
  /** Estaciones con media de 24 horas y banda INCA. */
  conEstado: number
  sinDatos: number
  noMiden: number
  /** Las que miden PM por hora: las que pueden llegar a tener estado. */
  miden: number
  /** El estado que la API recibió último (el de `ts` más reciente), con su estación. */
  ultimo: { id: string; estado: RegimenEstacion } | null
}

export function resumir(filas: readonly FilaEstacion[]): Resumen {
  const r: Resumen = { enAlerta: 0, conEstado: 0, sinDatos: 0, noMiden: 0, miden: 0, ultimo: null }
  for (const f of filas) {
    if (f.alerta) r.enAlerta++
    if (f.situacion === 'con_estado') r.conEstado++
    else if (f.situacion === 'sin_datos') r.sinDatos++
    else r.noMiden++
    if (f.estado && (!r.ultimo || masReciente(f.estado, r.ultimo.estado))) r.ultimo = { id: f.id, estado: f.estado }
  }
  r.miden = r.conEstado + r.sinDatos
  return r
}

function masReciente(a: RegimenEstacion, b: RegimenEstacion): boolean {
  const ta = Date.parse(a.ts)
  const tb = Date.parse(b.ts)
  // `ts` va al segundo: si empatan, vale la lectura de fecha más nueva.
  return ta !== tb ? ta > tb : Date.parse(a.fecha_hora) > Date.parse(b.fecha_hora)
}

/** La frase del panel, con plantilla fija (spec §5.2). */
export function frasePanel(r: Resumen): { tono: Tono; texto: string } {
  if (r.enAlerta > 0) {
    const cuantas = r.enAlerta === 1 ? 'hay 1 estación' : `hay ${r.enAlerta} estaciones`
    return {
      tono: 'alerta',
      texto: `En el último estado recibido ${cuantas} en alerta: su media de 24 horas supera el ECA.`,
    }
  }
  if (r.conEstado > 0) {
    return {
      tono: 'buena',
      texto: 'En el último estado recibido ninguna estación está en alerta: ninguna media de 24 horas supera el ECA.',
    }
  }
  return {
    tono: 'acento',
    texto: `Ninguna estación tiene todavía una media de 24 horas: hacen falta ${MINIMO_LECTURAS} lecturas en la ventana para calcularla.`,
  }
}

/** Qué se dice de una estación sin banda: por qué no la tiene. */
export function detalleSinBanda(fila: FilaEstacion): string {
  if (fila.situacion === 'no_mide') return 'no mide PM2.5 y PM10 por hora'
  if (!fila.estado) return 'todavía no recibió lecturas'
  const n = fila.estado.media_24h.lecturas
  return `${n} de ${MINIMO_LECTURAS} lecturas en la ventana`
}

/** El tono de una tarjeta o de una frase para una estación: el de su alerta o el de su banda. */
export function tonoDe(fila: Pick<FilaEstacion, 'alerta' | 'banda'>): Tono {
  if (fila.alerta || fila.banda === 'umbral_de_cuidado') return 'alerta'
  if (fila.banda === 'buena' || fila.banda === 'moderada' || fila.banda === 'mala') return fila.banda
  return 'neutro'
}

/** «Lurigancho» a partir de «LURIGANCHO»: los nombres de la ficha vienen en mayúsculas. */
export function nombrePropio(nombre: string | null | undefined): string | null {
  if (!nombre) return null
  const menores = new Set(['de', 'del', 'la', 'las', 'los', 'y'])
  return nombre
    .toLocaleLowerCase('es')
    .split(' ')
    .map((p, i) => (i > 0 && menores.has(p) ? p : p.charAt(0).toLocaleUpperCase('es') + p.slice(1)))
    .join(' ')
}
