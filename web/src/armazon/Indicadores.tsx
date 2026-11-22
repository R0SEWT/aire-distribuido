import { NavLink } from 'react-router'
import { useEstadoWS, useReconectarWS } from '../api/tiempoReal'
import type { EstadoWS } from '../api/ws'
import { numero } from '../componentes/formato'
import { enMarcha, simulador as elDeLaApp, useCorrida, type Fase, type Simulador } from '../pantallas/simulacion/motor'
import { RUTAS } from './menu'
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

const SIMULACION: Partial<Record<Fase, { punto: string; palabra: string }>> = {
  en_curso: { punto: 'bg-acento-claro animate-pulse', palabra: 'Simulación en curso' },
  en_pausa: { punto: 'bg-aviso', palabra: 'Simulación en pausa' },
  con_error: { punto: 'bg-falla', palabra: 'Simulación detenida por un error' },
}

/**
 * Mientras hay una corrida sin terminar (spec §5.4), lo dice desde cualquier
 * pantalla y lleva a Simulación: la corrida sigue aunque se cambie de pantalla,
 * y escribe estado compartido. Sin corrida no se dibuja nada.
 *
 * El simulador vive fuera de React (pantallas/simulacion/motor.ts); `simulador`
 * es para las pruebas.
 */
export function SimulacionEnCurso({ simulador = elDeLaApp }: { simulador?: Simulador }) {
  const corrida = useCorrida(simulador)
  const estado = SIMULACION[corrida.fase]
  if (!enMarcha(corrida.fase) || !estado || !corrida.tramo) return null
  const total = corrida.tramo.lecturas.length
  const que = corrida.tramo.tipo === 'escenario' ? 'el escenario' : corrida.tramo.titulo
  return (
    <span role="status" data-simulacion-en-curso={corrida.fase}>
      <NavLink
        to={RUTAS.simulacion}
        title={`${estado.palabra}: ${que}, ${numero(corrida.enviadas)} de ${numero(total)} lecturas enviadas. Ir a Simulación.`}
        className="border-borde bg-superficie text-suave hover:border-tenue inline-flex items-center gap-2 rounded-full border px-3.5 py-1.5 text-sm font-medium whitespace-nowrap no-underline hover:no-underline"
      >
        <span aria-hidden="true" className={`size-2.5 rounded-full ${estado.punto}`} />
        {estado.palabra}
        {/* El avance cambia varias veces por segundo: se ve, pero no se anuncia cada vez. */}
        <span aria-hidden="true" className="text-tenue font-mono text-xs tabular-nums">
          {numero(corrida.enviadas)}/{numero(total)}
        </span>
      </NavLink>
    </span>
  )
}
