// El historial de entrenamientos con su estado (spec §5.5), del más reciente
// al más antiguo. Cada fila se puede abrir para volver a ver sus rondas y su
// resultado completo (motivo de paro, diferencia con el secuencial): la tabla
// lleva lo justo para entrar en la pantalla también en modo proyector.

import { useState } from 'react'
import { duracion, Etiqueta, numero, Tabla, type Columna, type Tono } from '../../componentes/indice'
import type { Resumen } from './seguimiento'

interface Props {
  entrenamientos: readonly Resumen[]
  modeloActivo: string | undefined
  /** El entrenamiento que se está mostrando arriba, si es uno de estos. */
  aLaVista: string | undefined
  /** El que se está pidiendo para abrirlo. */
  abriendo: string | undefined
  alVer: (id: string) => void
}

/** Cuántos se muestran sin pedirlo: en una demostración se lanzan muchos y la lista no tiene que tapar lo demás. */
const A_LA_VISTA = 10

const ESTADO: Record<Resumen['estado'], { palabra: string; tono: Tono }> = {
  en_curso: { palabra: 'En curso', tono: 'acento' },
  terminado: { palabra: 'Terminado', tono: 'buena' },
  abortado: { palabra: 'Abortado', tono: 'alerta' },
}

/** Qué pasó con el modelo de un entrenamiento, en una etiqueta y con sus palabras. */
function Modelo({ e, modeloActivo }: { e: Resumen; modeloActivo: string | undefined }) {
  if (e.estado === 'abortado') {
    return <span className="text-suave block max-w-[24rem] text-xs leading-snug break-words">{e.error}</span>
  }
  if (e.estado === 'en_curso') return <span className="text-tenue">—</span>
  if (e.modelo_id) {
    return e.modelo_id === modeloActivo ? (
      <Etiqueta tono="buena">Es el modelo activo</Etiqueta>
    ) : (
      <Etiqueta>Activó su modelo</Etiqueta>
    )
  }
  if (e.aviso) {
    return (
      <span title={e.aviso}>
        <Etiqueta tono="alerta">No se activó</Etiqueta>
      </span>
    )
  }
  return <Etiqueta>Medición: no cambia el modelo</Etiqueta>
}

export function Historial({ entrenamientos, modeloActivo, aLaVista, abriendo, alVer }: Props) {
  const [todos, setTodos] = useState(false)
  const visibles = todos ? entrenamientos : entrenamientos.slice(0, A_LA_VISTA)
  const columnas: Columna<Resumen>[] = [
    {
      clave: 'id',
      titulo: 'Entrenamiento',
      celda: (e) => <span className="font-mono font-semibold whitespace-nowrap">{e.id}</span>,
    },
    { clave: 'estado', titulo: 'Estado', celda: (e) => <Etiqueta tono={ESTADO[e.estado].tono}>{ESTADO[e.estado].palabra}</Etiqueta> },
    {
      clave: 'nodos',
      titulo: 'Nodos',
      numerica: true,
      celda: (e) => <span title={e.nodos.join(', ')}>{e.nodos.length}</span>,
    },
    { clave: 'replicas', titulo: 'Réplicas', numerica: true, celda: (e) => numero(e.replicas) },
    { clave: 'iteraciones', titulo: 'Iteraciones', numerica: true, celda: (e) => numero(e.iteraciones) },
    {
      clave: 'ms_total',
      titulo: 'Tiempo real',
      numerica: true,
      celda: (e) => (e.ms_total === undefined ? '—' : duracion(e.ms_total)),
    },
    { clave: 'modelo', titulo: 'Modelo o error', celda: (e) => <Modelo e={e} modeloActivo={modeloActivo} /> },
    {
      clave: 'ver',
      titulo: <span className="sr-only">Acciones</span>,
      celda: (e) =>
        e.id === aLaVista ? (
          <span className="text-tenue text-xs whitespace-nowrap">a la vista</span>
        ) : (
          <button
            type="button"
            className="pildora px-2.5 py-1 text-xs"
            aria-label={`Ver las rondas de ${e.id}`}
            disabled={abriendo !== undefined}
            onClick={() => alVer(e.id)}
          >
            {abriendo === e.id ? 'Abriendo…' : 'Ver rondas'}
          </button>
        ),
    },
  ]
  return (
    <div className="space-y-3">
      <Tabla
        titulo="Historial de entrenamientos"
        columnas={columnas}
        filas={visibles}
        claveDe={(e) => e.id}
        vacio="Todavía no se lanzó ningún entrenamiento."
      />
      {entrenamientos.length > A_LA_VISTA && (
        <p className="text-suave flex flex-wrap items-center gap-3 text-sm">
          {todos
            ? `Se muestran los ${entrenamientos.length} entrenamientos.`
            : `Se muestran los ${A_LA_VISTA} más recientes de ${entrenamientos.length}.`}
          <button type="button" className="pildora px-2.5 py-1 text-xs" onClick={() => setTodos(!todos)}>
            {todos ? 'Ver solo los más recientes' : 'Ver todos'}
          </button>
        </p>
      )}
    </div>
  )
}
