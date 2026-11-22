import type { components } from '../../api/contrato'
import { Estado, Etiqueta, fechaConSegundos, type Tono } from '../../componentes/indice'

type Evento = components['schemas']['EventoCluster']

const TIPO: Record<Evento['tipo'], { palabra: string; tono: Tono }> = {
  alta: { palabra: 'Alta', tono: 'buena' },
  baja: { palabra: 'Baja', tono: 'neutro' },
  error: { palabra: 'Error', tono: 'alerta' },
}

/**
 * Los eventos del clúster (spec §5.6), que son los «logs» del enunciado: altas,
 * bajas y errores de nodos, del más reciente al más antiguo.
 */
export function Eventos({ eventos }: { eventos: readonly Evento[] }) {
  if (eventos.length === 0) {
    return (
      <Estado tipo="vacio" titulo="Todavía no hay eventos" className="border-0 py-6">
        Aparecen cuando un nodo entra al pool, sale o falla.
      </Estado>
    )
  }
  return (
    <ol aria-label="Eventos del clúster" className="divide-borde/60 max-h-[21rem] divide-y overflow-y-auto pr-1">
      {eventos.map((e, i) => (
        // Varios eventos pueden compartir instante, tipo y nodo: la posición desempata.
        <li key={`${e.ts}-${e.tipo}-${e.nodo}-${i}`} data-evento={e.tipo} className="grid grid-cols-[auto_4.25rem_minmax(0,1fr)] items-baseline gap-x-3 py-2">
          <time dateTime={e.ts} className="text-suave font-mono text-xs whitespace-nowrap">
            {fechaConSegundos(e.ts)}
          </time>
          <span>
            <Etiqueta tono={TIPO[e.tipo].tono}>{TIPO[e.tipo].palabra}</Etiqueta>
          </span>
          <p className="min-w-0 text-sm leading-snug">
            <span className="text-texto font-mono font-semibold">{e.nodo}</span>{' '}
            <span className="text-suave break-words">{e.detalle}</span>
          </p>
        </li>
      ))}
    </ol>
  )
}
