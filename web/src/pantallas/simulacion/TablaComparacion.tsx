// Lo que la API respondió en la última lectura de cada estación, contra lo
// «esperado» de series.json. Cada celda trae las dos cosas: arriba lo que
// respondió la API y debajo lo que dice el archivo.

import type { ReactNode } from 'react'
import { Banda, BANDAS, estadoDeBanda, Etiqueta, fechaHora, numero, Tabla, type Columna } from '../../componentes/indice'
import { NOMBRE_DE_CAMPO, type Campo, type Comparacion } from './comparar'
import { enumerar } from './fechas'

/** Un valor de la API con el esperado debajo; si difieren, el esperado va marcado. */
function Par({
  api,
  esperado,
  igual,
  pendiente,
  aLaDerecha = false,
}: {
  api: ReactNode
  esperado: ReactNode
  igual: boolean
  pendiente: boolean
  aLaDerecha?: boolean
}) {
  return (
    <span className={`inline-flex flex-col gap-0.5 leading-tight whitespace-nowrap ${aLaDerecha ? 'items-end' : 'items-start'}`}>
      <span>{api}</span>
      <span className={`font-sans text-xs ${igual || pendiente ? 'text-tenue' : 'text-falla font-semibold'}`}>
        {igual || pendiente ? 'esperado ' : 'difiere: esperado '}
        {esperado}
      </span>
    </span>
  )
}

function cuando(iso: string | null): string {
  return iso === null ? 'nunca' : fechaHora(iso)
}

function Veredicto({ c }: { c: Comparacion }) {
  switch (c.veredicto) {
    case 'incompleta': {
      const faltan = c.esperado.lecturas_enviadas - c.enviadas
      return <Etiqueta>{faltan === 1 ? 'Falta 1 lectura' : `Faltan ${faltan} lecturas`}</Etiqueta>
    }
    case 'coincide':
      return <Etiqueta tono="buena">Coincide</Etiqueta>
    case 'solo_regimen':
      return <Etiqueta tono="moderada">Solo difiere el régimen</Etiqueta>
    case 'difiere':
      return (
        <span className="inline-flex flex-col items-start gap-1">
          <Etiqueta tono="alerta">Difiere</Etiqueta>
          <span className="text-falla text-xs">{enumerar(c.diferencias.map((d) => NOMBRE_DE_CAMPO[d]))}</span>
        </span>
      )
  }
}

export function TablaComparacion({ comparaciones }: { comparaciones: readonly Comparacion[] }) {
  const columnas: Columna<Comparacion>[] = [
    { clave: 'estacion', titulo: 'Estación', celda: (c) => <span className="font-mono font-semibold whitespace-nowrap">{c.estacion}</span> },
    {
      clave: 'lecturas',
      titulo: 'Lecturas',
      numerica: true,
      celda: (c) => (
        <Par
          api={c.enviadas}
          esperado={c.esperado.lecturas_enviadas}
          igual={c.enviadas === c.esperado.lecturas_enviadas}
          pendiente={c.veredicto === 'incompleta'}
          aLaDerecha
        />
      ),
    },
    media('media_pm2_5', 'PM2.5 · 24 h', 'pm2_5'),
    media('media_pm10', 'PM10 · 24 h', 'pm10'),
    {
      clave: 'ventana',
      titulo: 'En la ventana',
      numerica: true,
      celda: (c) => (
        <Par
          api={c.seguimiento?.ultima?.media_24h.lecturas ?? '—'}
          esperado={c.esperado.media_24h.lecturas}
          igual={!difiere(c, 'lecturas_en_ventana')}
          pendiente={c.veredicto === 'incompleta'}
          aLaDerecha
        />
      ),
    },
    {
      clave: 'inca',
      titulo: 'Banda INCA',
      celda: (c) => (
        <Par
          api={c.seguimiento?.ultima ? <Banda valor={c.seguimiento.ultima.inca} /> : '—'}
          esperado={BANDAS[estadoDeBanda(c.esperado.inca)].palabra.toLowerCase()}
          igual={!difiere(c, 'inca')}
          pendiente={c.veredicto === 'incompleta'}
        />
      ),
    },
    {
      clave: 'alerta',
      titulo: 'Alerta',
      celda: (c) => {
        const r = c.seguimiento?.ultima
        return (
          <Par
            api={
              !r ? (
                '—'
              ) : r.alerta ? (
                <Etiqueta tono="alerta">En alerta</Etiqueta>
              ) : (
                <span className="text-suave">Sin alerta</span>
              )
            }
            esperado={c.esperado.alerta ? 'en alerta' : 'sin alerta'}
            igual={!difiere(c, 'alerta')}
            pendiente={c.veredicto === 'incompleta'}
          />
        )
      },
    },
    {
      clave: 'primera_alerta',
      titulo: 'Primera alerta',
      celda: (c) => (
        <Par
          api={c.seguimiento ? cuando(c.seguimiento.primeraAlerta) : '—'}
          esperado={cuando(c.esperado.primera_alerta)}
          igual={!difiere(c, 'primera_alerta')}
          pendiente={c.veredicto === 'incompleta'}
        />
      ),
    },
    {
      clave: 'regimen',
      titulo: 'Régimen',
      numerica: true,
      celda: (c) => (
        <Par
          api={c.seguimiento?.ultima?.regimen ?? '—'}
          esperado={c.esperado.regimen}
          igual={!difiere(c, 'regimen') && !difiere(c, 'excede_eca')}
          pendiente={c.veredicto === 'incompleta'}
          aLaDerecha
        />
      ),
    },
    { clave: 'veredicto', titulo: 'Resultado', celda: (c) => <Veredicto c={c} /> },
  ]
  return (
    <Tabla
      titulo="Lo que respondió la API en la última lectura de cada estación, contra lo esperado"
      columnas={columnas}
      filas={comparaciones}
      claveDe={(c) => c.estacion}
    />
  )
}

function difiere(c: Comparacion, campo: Campo): boolean {
  return c.diferencias.includes(campo)
}

function media(campo: 'media_pm2_5' | 'media_pm10', titulo: string, de: 'pm2_5' | 'pm10'): Columna<Comparacion> {
  const sinMedia = (v: number | null) => (v === null ? 'sin media' : numero(v, 1))
  return {
    clave: campo,
    titulo,
    numerica: true,
    celda: (c) => (
      <Par
        api={c.seguimiento?.ultima ? sinMedia(c.seguimiento.ultima.media_24h[de]) : '—'}
        esperado={sinMedia(c.esperado.media_24h[de])}
        igual={!difiere(c, campo)}
        pendiente={c.veredicto === 'incompleta'}
        aLaDerecha
      />
    ),
  }
}
