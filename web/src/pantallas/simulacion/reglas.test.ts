// Las dos reglas que la pantalla de Simulación aplica sin la API: cuándo una
// respuesta «coincide» con lo esperado y cuándo el sistema no está limpio.

import { describe, expect, it } from 'vitest'
import type { EstadoEstacion } from '../../api/enVivo'
import { SERIES } from './apiDePrueba'
import { compararEstacion, compararTramo, resumir } from './comparar'
import { diaDisponible, revisarSistema } from './limpieza'
import type { Seguimiento } from './motor'
import type { Esperado } from '../../api/datos'
import { estacionesConPM, tramoDelDia, tramoDelEscenario, type Prediccion } from './series'
import type { LecturaVista } from '../../api/vistas'

const DIA = SERIES.dias.find((d) => d.id === 'CA-ESP-01-2021-12-21')!
const ESPERADO = DIA.esperado

function respuestaDe(e: Esperado, cambios: Partial<Prediccion> = {}): Prediccion {
  return {
    estacion: 'CA-ESP-01',
    regimen: e.regimen,
    excede_eca: e.excede_eca,
    media_24h: e.media_24h,
    alerta: e.alerta,
    inca: e.inca,
    pm2_5_medio_regimen: 9,
    pm10_medio_regimen: 17,
    modelo_id: 'sembrado-kmeans-json',
    ms: 0.04,
    ...cambios,
  }
}

function seguimiento(ultima: Prediccion, cambios: Partial<Seguimiento> = {}, enviadas = ESPERADO.lecturas_enviadas): Seguimiento {
  return {
    estacion: 'CA-ESP-01',
    puntos: Array.from({ length: enviadas }, () => ({}) as LecturaVista),
    ultima,
    primeraMedia: ESPERADO.primera_media,
    primeraAlerta: ESPERADO.primera_alerta,
    ...cambios,
  }
}

function estado(estacion: string, fecha_hora: string): EstadoEstacion {
  return { estacion, fecha_hora, regimen: 0, excede_eca: false, media_24h: { pm2_5: 1, pm10: 2, lecturas: 24 }, alerta: false, inca: 'buena', ts: fecha_hora }
}

describe('comparar con lo esperado', () => {
  it('coincide si la última respuesta trae lo mismo que el archivo', () => {
    const c = compararEstacion('CA-ESP-01', ESPERADO, seguimiento(respuestaDe(ESPERADO)))

    expect(c.veredicto).toBe('coincide')
    expect(c.diferencias).toEqual([])
  })

  it('una diferencia de redondeo en la media no es una diferencia', () => {
    const media = { ...ESPERADO.media_24h, pm10: ESPERADO.media_24h.pm10! + 1e-12 }

    expect(compararEstacion('CA-ESP-01', ESPERADO, seguimiento(respuestaDe(ESPERADO, { media_24h: media }))).veredicto).toBe('coincide')
  })

  it('mientras falten lecturas no hay veredicto', () => {
    const c = compararEstacion('CA-ESP-01', ESPERADO, seguimiento(respuestaDe(ESPERADO, { alerta: false }), {}, 30))

    expect(c.veredicto).toBe('incompleta')
    expect(c.diferencias).toEqual([])
    expect(compararEstacion('CA-ESP-01', ESPERADO, undefined).veredicto).toBe('incompleta')
  })

  it('dice en qué campos difiere', () => {
    const media = { ...ESPERADO.media_24h, pm2_5: 30 }
    const c = compararEstacion('CA-ESP-01', ESPERADO, seguimiento(respuestaDe(ESPERADO, { media_24h: media, inca: 'mala' }), { primeraAlerta: null }))

    expect(c.veredicto).toBe('difiere')
    expect(c.diferencias).toEqual(['media_pm2_5', 'inca', 'primera_alerta'])
  })

  it('una media que falta donde se esperaba una (o al revés) difiere', () => {
    const sinMedia = { pm2_5: null, pm10: null, lecturas: 17 }

    expect(compararEstacion('CA-ESP-01', ESPERADO, seguimiento(respuestaDe(ESPERADO, { media_24h: sinMedia }))).diferencias).toEqual([
      'media_pm2_5',
      'media_pm10',
      'lecturas_en_ventana',
    ])
  })

  it('si solo cambia el régimen, es «solo el régimen»: depende del modelo activo', () => {
    const c = compararEstacion('CA-ESP-01', ESPERADO, seguimiento(respuestaDe(ESPERADO, { regimen: 2, excede_eca: true })))

    expect(c.veredicto).toBe('solo_regimen')
    expect(c.diferencias).toEqual(['regimen', 'excede_eca'])
  })

  it('el resumen cuenta cada veredicto y nombra las estaciones en alerta', () => {
    const tramo = tramoDelEscenario(SERIES.escenario)
    const porEstacion: Record<string, Seguimiento> = {}
    for (const estacion of tramo.estaciones) {
      const e = tramo.esperado[estacion]!
      porEstacion[estacion] = { ...seguimiento(respuestaDe(e), { primeraMedia: e.primera_media, primeraAlerta: e.primera_alerta }, e.lecturas_enviadas), estacion }
    }
    delete porEstacion['CA-TAL-01']
    porEstacion['CA-SOC-01'] = { ...porEstacion['CA-SOC-01']!, ultima: { ...porEstacion['CA-SOC-01']!.ultima!, regimen: 0, excede_eca: false } }

    const resumen = resumir(compararTramo(tramo, porEstacion))

    expect(resumen).toEqual({ estaciones: 14, completas: 13, coinciden: 12, soloRegimen: 1, difieren: 0, enAlerta: ['CA-HU-04', 'CA-HU-09'] })
  })
})

describe('el sistema antes de simular', () => {
  const tramo = tramoDelDia(DIA)

  it('está limpio si la estación no tiene estado o tiene uno anterior al tramo', () => {
    expect(revisarSistema(tramo, [])).toEqual({ limpio: true, choques: [] })
    expect(revisarSistema(tramo, [estado('CA-ESP-01', '2021-12-16T00:00:00-05:00')]).limpio).toBe(true)
  })

  it('el estado de otras estaciones no cuenta', () => {
    expect(revisarSistema(tramo, [estado('CA-HUA-01', '2022-12-31T00:00:00-05:00')]).limpio).toBe(true)
  })

  it.each([
    ['dentro', '2021-12-20T01:00:00-05:00'],
    ['dentro', '2021-12-21T12:00:00-05:00'],
    ['misma', '2021-12-22T00:00:00-05:00'],
    ['posterior', '2021-12-22T01:00:00-05:00'],
  ] as const)('un estado %s del tramo (%s) es un choque', (tipo, fecha) => {
    const revision = revisarSistema(tramo, [estado('CA-ESP-01', fecha)])

    expect(revision.limpio).toBe(false)
    expect(revision.choques).toEqual([{ estacion: 'CA-ESP-01', guardada: fecha, tipo }])
  })

  it('compara instantes, no textos: la misma hora escrita en UTC es la misma hora', () => {
    expect(revisarSistema(tramo, [estado('CA-ESP-01', '2021-12-22T05:00:00Z')]).choques[0]?.tipo).toBe('misma')
  })

  it('un día está disponible mientras su estación no tenga un estado de esas horas o posterior', () => {
    expect(diaDisponible(DIA, [])).toBe(true)
    expect(diaDisponible(DIA, [estado('CA-ESP-01', '2021-12-16T00:00:00-05:00')])).toBe(true)
    expect(diaDisponible(DIA, [estado('CA-ESP-01', '2021-12-20T01:00:00-05:00')])).toBe(false)
  })
})

describe('series.json', () => {
  it('el escenario son 48 horas, 14 estaciones y 667 lecturas, y cada día trae lo que se espera de él', () => {
    const tramo = tramoDelEscenario(SERIES.escenario)

    expect(tramo.lecturas).toHaveLength(667)
    expect(tramo.estaciones).toHaveLength(14)
    expect(Object.keys(tramo.esperado).sort()).toEqual([...tramo.estaciones])
    expect(SERIES.dias).toHaveLength(9)
    for (const d of SERIES.dias) expect(tramoDelDia(d).esperado[d.estacion]?.lecturas_enviadas).toBe(d.lecturas.length)
  })

  it('las estaciones con PM por hora son 17: las del escenario y las tres que no midieron en él', () => {
    const estaciones = estacionesConPM(SERIES)

    expect(estaciones).toHaveLength(17)
    expect(estaciones).toEqual(expect.arrayContaining(['CA-CG-02', 'CA-JUPR-01', 'CA-RZC-02']))
  })
})
