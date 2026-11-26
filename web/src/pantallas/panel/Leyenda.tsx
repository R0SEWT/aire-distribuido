import { useId } from 'react'
import { BANDAS, type EstadoBanda } from '../../componentes/indice'
import { GlifoEstacion, TramaNoMide } from '../../graficos/MapaPeru'
import type { FilaEstacion } from './situacion'

function Marca({ banda, alerta = false }: { banda: EstadoBanda; alerta?: boolean }) {
  const trama = useId()
  return (
    <svg viewBox="-12 -12 24 24" aria-hidden="true" className="size-5 shrink-0">
      <defs>
        <TramaNoMide id={trama} />
      </defs>
      <GlifoEstacion banda={banda} alerta={alerta} trama={trama} />
    </svg>
  )
}

const ORDEN: readonly EstadoBanda[] = ['buena', 'moderada', 'mala', 'umbral_de_cuidado', 'sin_datos', 'no_mide']

/**
 * La leyenda del mapa: cada marca con su palabra y cuántas estaciones hay así
 * en el último estado recibido. Los tres estados se distinguen por la forma, no
 * solo por el color.
 */
export function Leyenda({ filas }: { filas: readonly FilaEstacion[] }) {
  const cuantas = new Map<EstadoBanda, number>()
  for (const f of filas) cuantas.set(f.banda, (cuantas.get(f.banda) ?? 0) + 1)
  const enAlerta = filas.filter((f) => f.alerta).length
  return (
    <ul aria-label="Leyenda del mapa" className="text-suave space-y-1 text-xs">
      {ORDEN.map((banda) => (
        <li key={banda} className="flex items-center gap-2" data-leyenda={banda}>
          <Marca banda={banda} />
          <span>
            {banda === 'sin_datos' || banda === 'no_mide' ? BANDAS[banda].palabra : `INCA ${BANDAS[banda].palabra.toLowerCase()}`}
            <span className="text-texto ml-1.5 font-mono font-semibold tabular-nums">{cuantas.get(banda) ?? 0}</span>
          </span>
        </li>
      ))}
      <li className="flex items-center gap-2" data-leyenda="alerta">
        <Marca banda="mala" alerta />
        <span>
          Con anillo: en alerta
          <span className="text-texto ml-1.5 font-mono font-semibold tabular-nums">{enAlerta}</span>
        </span>
      </li>
    </ul>
  )
}
