// La forma de /datos/impacto.json. Lo escribe scripts/impacto.py con el gold de
// OEFA (spec §7): ninguna cifra de la pantalla de impacto se escribe a mano.

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
    typeof i.totales === 'object' &&
    typeof i.senales === 'object' &&
    typeof i.anticipacion === 'object' &&
    typeof i.metadatos?.reglas === 'object'
  )
}
