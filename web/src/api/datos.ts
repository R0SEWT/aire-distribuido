// Los archivos estáticos de /datos/ (web/public/datos/README.md): impacto.json,
// series.json y peru.geojson. No pasan por la API ni por su cliente tipado: los
// sirve el mismo origen, y uno que no existe responde 404 (no la página).
//
// Aquí está todo lo que una pantalla necesita de ellos: cómo se piden, la forma
// de cada uno y un hook por archivo. Las claves de caché están en `claves`
// (api/cliente.ts): el mismo archivo se pide una vez y lo comparten todas las
// pantallas que lo usan.

import { useQuery } from '@tanstack/react-query'
import type { ExtendedFeatureCollection } from 'd3'
import { claves } from './cliente'
import type { components } from './contrato'
import { ErrorApi, errorDeFetch, errorDeRespuesta, esCancelacion } from './errores'

type Lectura = components['schemas']['Lectura']
type BandaINCA = components['schemas']['BandaINCA']

/** Los datos estáticos no cambian mientras la página está abierta: se piden una vez. */
export const DATOS_ESTATICOS = { staleTime: Infinity, gcTime: Infinity } as const

/**
 * Pide un archivo de /datos/ y lo devuelve ya leído. Si no está o no es JSON,
 * lanza un ErrorApi, como pedir(): la pantalla se lo pasa a <EstadoDeError>.
 */
export async function pedirDatos<T>(archivo: string, signal?: AbortSignal): Promise<T> {
  const ruta = `/datos/${archivo}`
  let respuesta: Response
  try {
    respuesta = await globalThis.fetch(new Request(new URL(ruta, globalThis.location.origin), { signal }))
  } catch (e) {
    if (esCancelacion(e)) throw e
    throw errorDeFetch(e)
  }
  if (!respuesta.ok) {
    throw errorDeRespuesta(respuesta.status, { error: `no se pudo leer ${ruta} (respondió ${respuesta.status})` }, true)
  }
  try {
    return (await respuesta.json()) as T
  } catch {
    // En desarrollo, Vite responde la página a un archivo que no existe.
    throw new ErrorApi('inesperado', respuesta.status, `${ruta} no es un JSON válido`)
  }
}

/** Pide un archivo y comprueba que tenga la forma mínima que las pantallas esperan de él. */
async function pedirConForma<T>(archivo: string, tieneLaForma: (x: unknown) => x is T, signal: AbortSignal): Promise<T> {
  const leido = await pedirDatos<unknown>(archivo, signal)
  if (!tieneLaForma(leido)) throw new ErrorApi('inesperado', 200, `/datos/${archivo} no tiene la forma esperada`)
  return leido
}

// ---------------------------------------------------------------------------
// impacto.json. Lo escribe scripts/impacto.py con el gold de OEFA (spec §7):
// ninguna cifra de la pantalla de impacto se escribe a mano.
// ---------------------------------------------------------------------------

export type NombreDeBanda = 'buena' | 'moderada' | 'mala' | 'umbral_de_cuidado'

/** Horas en cada banda INCA. */
export type HorasPorBanda = Record<NombreDeBanda, number>

export interface Fuente {
  ruta: string
  sha256: string
  filas?: number
  que_es?: string
}

export interface Reglas {
  lectura: string
  ventana: string
  ventana_horas: number
  minimo_lecturas: number
  eca_pm2_5: number
  eca_pm10: number
  alerta: string
  inca: { bandas: string[]; cortes_pm10: number[]; cortes_pm2_5: number[]; regla: string }
  dia: string
  implementacion: string
}

/** Lo medido por una estación, un tipo de actividad o un distrito. */
interface Medido {
  primera_fecha: string
  ultima_fecha: string
  lecturas: number
  /** Horas con media de 24 h calculable (18 lecturas o más en la ventana). */
  horas_calculables: number
  horas_sobre_eca: number
  inca: HorasPorBanda
  banda_mas_frecuente: NombreDeBanda
}

export interface PoblacionCensada {
  poblacion: number
  anio: number
  /** La página del INEI de donde sale la cifra. */
  fuente: string
}

export interface EstacionDeImpacto extends Medido {
  estacion: string
  objeto_fiscalizado: string
  tipo_actividad: string
  departamento: string | null
  provincia: string | null
  distrito: string | null
  ubigeo: string | null
  /** null si la fuente no trae el distrito de la estación. */
  poblacion_distrito: PoblacionCensada | null
  dias_con_datos: number
}

export interface DistritoDeTipo extends PoblacionCensada {
  ubigeo: string
  departamento: string
  provincia: string
  distrito: string
}

export interface TipoDeActividad extends Medido {
  tipo_actividad: string
  objetos_fiscalizados: string[]
  distritos: DistritoDeTipo[]
  estaciones_sin_distrito: string[]
  estaciones: string[]
  /** Suma los días de cada estación: dos que midieron el mismo día cuentan dos. */
  dias_estacion_con_datos: number
}

export interface DistritoDeImpacto extends Medido {
  ubigeo: string | null
  departamento: string | null
  provincia: string | null
  distrito: string | null
  poblacion: number | null
  anio: number | null
  fuente: string | null
  tipos_actividad: string[]
  estaciones: string[]
  dias_estacion_con_datos: number
}

export interface PoblacionDeDistrito extends PoblacionCensada {
  ubigeo: string
  distrito: string
  departamento?: string
  provincia?: string
  /** Todas las estaciones del distrito. */
  estaciones: string[]
  estaciones_con_pm_horario?: string[]
}

export interface Senal {
  /** Horas con media calculable en que la señal estaba encendida. */
  horas: number
  /** En cuántas de esas la media de 24 h superaba el ECA. */
  aciertos: number
  /** aciertos / horas. */
  precision: number
  /** aciertos / horas sobre el ECA. */
  cobertura: number
}

export interface Cuartiles {
  n: number
  minimo: number
  q1: number
  mediana: number
  q3: number
  maximo: number
}

export interface AnticipacionDeSenal {
  con_senal: number
  sin_senal: number
  con_senal_previa: number
  /** Episodios en que la señal ya estaba encendida en la primera hora de la ventana. */
  en_el_tope: number
  /** Horas desde la primera vez que la señal se enciende en la ventana; null sin episodios. */
  horas_antes: Cuartiles | null
  /** Horas seguidas encendida justo antes del cruce: la cota baja. */
  horas_seguidas: Cuartiles | null
}

export interface GrupoDeAnticipacion {
  episodios: number
  con_media_previa: number
  sin_media_previa: number
  sin_senal_previa: number
  horas_sobre_eca: number
  lectura_horaria: AnticipacionDeSenal
  regimen: AnticipacionDeSenal
}

export interface Episodio {
  estacion: string
  inicio: string
  fin: string
  horas_sobre_eca: number
  media_previa_calculable: boolean
  horas_antes_lectura_horaria: number
  horas_antes_regimen: number | null
  horas_seguidas_lectura_horaria: number
  horas_seguidas_regimen: number
}

export interface Impacto {
  metadatos: {
    generado: string
    generador: string
    fuentes: Record<string, Fuente>
    reglas: Reglas
    notas: string[]
  }
  alcance: {
    estaciones_en_la_ficha: number
    estaciones_con_pm_horario: number
    estaciones_sin_pm_horario: string[]
    distritos_con_pm_horario: number
    estaciones_sin_distrito: string[]
  }
  totales: { lecturas: number; horas_calculables: number; horas_sobre_eca: number; inca: HorasPorBanda }
  estaciones: EstacionDeImpacto[]
  tipos_actividad: TipoDeActividad[]
  distritos: DistritoDeImpacto[]
  /** La población censada de cada distrito con estaciones, midan PM por hora o no. */
  poblacion_distritos: PoblacionDeDistrito[]
  senales: {
    nota: string
    horas_calculables: number
    horas_sobre_eca: number
    regimen_alto: Senal & { regimen: number }
    lectura_horaria: Senal
  }
  anticipacion: {
    definicion: Record<string, string | number>
    lo_que_no_es: string
    regimen_que_excede: number
    cruces_hacia_arriba: number
    total: GrupoDeAnticipacion
    solo_con_media_previa: GrupoDeAnticipacion
    por_estacion: (GrupoDeAnticipacion & { estacion: string; cruces_hacia_arriba: number })[]
    episodios: Episodio[]
  }
}

/** Si lo leído tiene la forma mínima de impacto.json: lo demás se confía al script que lo escribe. */
export function esImpacto(x: unknown): x is Impacto {
  if (typeof x !== 'object' || x === null) return false
  const i = x as Partial<Impacto>
  return (
    Array.isArray(i.estaciones) &&
    Array.isArray(i.distritos) &&
    Array.isArray(i.poblacion_distritos) &&
    Array.isArray(i.alcance?.estaciones_sin_pm_horario) &&
    typeof i.totales === 'object' &&
    typeof i.senales === 'object' &&
    typeof i.anticipacion === 'object' &&
    typeof i.metadatos?.reglas === 'object'
  )
}

// ---------------------------------------------------------------------------
// series.json. Lo escribe scripts/impacto.py (spec §7.3): las lecturas reales
// del escenario y de los días sueltos de la simulación, con lo que la API tiene
// que responder al final.
// ---------------------------------------------------------------------------

/** Lo que la API tiene que responder en la última lectura de una estación del tramo. */
export interface Esperado {
  fecha_hora: string
  lecturas_enviadas: number
  regimen: number
  excede_eca: boolean
  media_24h: { pm2_5: number | null; pm10: number | null; lecturas: number }
  alerta: boolean
  inca: BandaINCA
  /** La fecha_hora de la primera lectura que devuelve media; null si ninguna. */
  primera_media: string | null
  /** La fecha_hora de la primera lectura que devuelve alerta = true; null si ninguna. */
  primera_alerta: string | null
}

export interface ReglasDeSeries {
  ventana_horas: number
  minimo_lecturas: number
  eca_pm2_5: number
  eca_pm10: number
  alerta: string
  inca: { bandas: string[]; cortes_pm10: number[]; cortes_pm2_5: number[]; regla: string }
  dia: string
}

export interface EscenarioDeSeries {
  id: string
  desde: string
  hasta: string
  dias: string[]
  horas: number
  criterio: string
  estaciones: { estacion: string; lecturas: number; con_media_al_final: boolean }[]
  estaciones_sin_lecturas: string[]
  lecturas: Lectura[]
  esperado: Record<string, Esperado>
}

export interface DiaDeSeries {
  id: string
  estacion: string
  dia: string
  caso: string
  descripcion: string
  desde: string
  hasta: string
  horas: number
  huecos: number
  lecturas: Lectura[]
  esperado: Esperado
}

export interface Series {
  metadatos: {
    generado: string
    generador: string
    reglas: ReglasDeSeries
    como_se_usa: { envio: string; esperado: string; regimen: string; orden: string }
  }
  escenario: EscenarioDeSeries
  dias: DiaDeSeries[]
}

/** Si lo leído tiene la forma mínima de series.json: lo demás se confía al script que lo escribe. */
export function esSeries(x: unknown): x is Series {
  if (typeof x !== 'object' || x === null) return false
  const s = x as Partial<Series>
  return Array.isArray(s.dias) && Array.isArray(s.escenario?.lecturas) && typeof s.metadatos?.reglas === 'object'
}

// ---------------------------------------------------------------------------
// peru.geojson. Lo escribe scripts/mapa_peru.py.
// ---------------------------------------------------------------------------

/** Los 26 polígonos de peru.geojson, con el departamento de cada uno. */
export type MapaDelPeru = ExtendedFeatureCollection

// ---------------------------------------------------------------------------
// Un hook por archivo.
// ---------------------------------------------------------------------------

/** impacto.json: lo medido por estación, tipo de actividad y distrito, las señales y la anticipación. */
export function useImpacto() {
  return useQuery({
    queryKey: claves.datosImpacto,
    ...DATOS_ESTATICOS,
    queryFn: ({ signal }) => pedirConForma('impacto.json', esImpacto, signal),
  })
}

/** series.json: el escenario y los días sueltos de la simulación. */
export function useSeries() {
  return useQuery({
    queryKey: claves.datosSeries,
    ...DATOS_ESTATICOS,
    queryFn: ({ signal }) => pedirConForma('series.json', esSeries, signal),
  })
}

/** peru.geojson: el contorno de los departamentos, para el mapa del Panel. */
export function useMapaDelPeru() {
  return useQuery({
    queryKey: claves.datosMapa,
    ...DATOS_ESTATICOS,
    queryFn: ({ signal }) => pedirDatos<MapaDelPeru>('peru.geojson', signal),
  })
}
