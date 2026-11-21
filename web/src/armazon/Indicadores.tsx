import { useEstadoWS, useReconectarWS } from '../api/tiempoReal'
import type { EstadoWS } from '../api/ws'
import { useSalud, type NivelDeSalud } from './salud'

const PUNTO: Record<NivelDeSalud, string> = {
  consultando: 'bg-tenue',
  ok: 'bg-ok',
  degradada: 'bg-aviso',
  caida: 'bg-falla',
}

/** El punto de estado de la API con su palabra. El detalle va en el título (al pasar el cursor). */
export function EstadoApi() {
  const salud = useSalud()
  return (
    <span
      role="status"
      title={salud.detalle}
      data-salud={salud.nivel}
      className="border-borde bg-superficie text-suave inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-sm font-medium whitespace-nowrap"
    >
      <span aria-hidden="true" className={`size-2.5 rounded-full ${PUNTO[salud.nivel]}`} />
      {salud.palabra}
      <span className="sr-only">. {salud.detalle}</span>
    </span>
  )
}

const TIEMPO_REAL: Record<Exclude<EstadoWS, 'inactiva' | 'vencida'>, { punto: string; palabra: string }> = {
  conectando: { punto: 'bg-tenue', palabra: 'Tiempo real: conectando…' },
  conectada: { punto: 'bg-ok', palabra: 'Tiempo real: conectado' },
  reconectando: { punto: 'bg-aviso', palabra: 'Tiempo real: reconectando…' },
  cortada: { punto: 'bg-falla', palabra: 'Tiempo real: desconectado' },
}

/** El estado de la conexión de tiempo real (spec §9.4), a la vista junto al de la API. */
export function EstadoTiempoReal() {
  const estado = useEstadoWS()
  const reconectar = useReconectarWS()
  // Sin sesión no hay conexión; con el token vencido la UI ya está volviendo al ingreso.
  if (estado === 'inactiva' || estado === 'vencida') return null
  const { punto, palabra } = TIEMPO_REAL[estado]
  return (
    <span
      role="status"
      data-tiempo-real={estado}
      className="border-borde bg-superficie text-suave inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-sm font-medium whitespace-nowrap"
    >
      <span aria-hidden="true" className={`size-2.5 rounded-full ${punto}`} />
      {palabra}
      {estado === 'cortada' && (
        <button type="button" className="text-acento-claro font-semibold hover:underline" onClick={reconectar}>
          Reconectar
        </button>
      )}
    </span>
  )
}
