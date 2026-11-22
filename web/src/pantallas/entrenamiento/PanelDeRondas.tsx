// Las rondas de un entrenamiento, reproducidas a ritmo humano (spec §5.5): la
// inercia que baja y lo que tardó cada nodo, con la frase que dice qué se está
// viendo de verdad.

import { Etiqueta, Frase, Pildoras, Tarjeta } from '../../componentes/indice'
import { InerciaPorRonda } from '../../graficos/InerciaPorRonda'
import { TiemposPorNodo } from '../../graficos/TiemposPorNodo'
import type { Corrida } from './seguimiento'
import { describirAvance } from './textos'
import { PASO_MS, type Reproduccion, type Velocidad } from './useReproduccion'

const ORIGEN: Record<Corrida['origen'], string> = {
  propia: 'lanzado desde aquí',
  ajena: 'lanzado desde otra sesión',
  historial: 'del historial',
}

interface Props {
  corrida: Corrida
  /** Los nodos del entrenamiento, en orden. */
  nodos: readonly string[]
  reproduccion: Reproduccion
  velocidad: Velocidad
  alCambiarVelocidad: (v: Velocidad) => void
}

export function PanelDeRondas({ corrida, nodos, reproduccion, velocidad, alCambiarVelocidad }: Props) {
  const { mostradas, irAlFinal, repetir } = reproduccion
  const avance = describirAvance(corrida, mostradas, PASO_MS[velocidad])
  const total = corrida.fin && !corrida.fin.abortado ? corrida.fin.iteraciones : undefined
  const hayRondas = corrida.rondas.length > 0

  return (
    <Tarjeta
      rotulo={
        <>
          <span className="font-mono normal-case">{corrida.id}</span> · {ORIGEN[corrida.origen]}
        </>
      }
      titulo="Rondas del entrenamiento"
      aLaDerecha={
        <span data-avance={avance.fase} role="status">
          <Etiqueta tono={avance.tono}>{avance.etiqueta}</Etiqueta>
        </span>
      }
    >
      <div className="space-y-4">
        <Frase tono={avance.tono}>{avance.frase}</Frase>

        {hayRondas && (
          <div className="flex flex-wrap items-center justify-between gap-3">
            <Pildoras<Velocidad>
              etiqueta="Velocidad de la reproducción"
              valor={velocidad}
              alCambiar={alCambiarVelocidad}
              opciones={[
                { valor: 'lenta', etiqueta: 'Lenta' },
                { valor: 'normal', etiqueta: 'Normal' },
                { valor: 'rapida', etiqueta: 'Rápida' },
              ]}
            />
            <div className="flex gap-2">
              <button type="button" className="pildora" onClick={irAlFinal} disabled={avance.fase !== 'reproduciendo'}>
                Ir al final
              </button>
              <button type="button" className="pildora" onClick={repetir} disabled={mostradas === 0}>
                Repetir
              </button>
            </div>
          </div>
        )}

        {hayRondas && (
          <>
            <figure>
              <figcaption className="text-suave mb-1 text-sm">
                <strong className="text-texto">Inercia por ronda.</strong> Baja mientras el modelo mejora y se aplana
                cuando converge.
              </figcaption>
              <InerciaPorRonda rondas={corrida.rondas} mostradas={mostradas} total={total} />
            </figure>
            <figure>
              <figcaption className="text-suave mb-1 text-sm">
                <strong className="text-texto">Tiempo de respuesta de cada nodo por ronda.</strong> Medido desde la API,
                con la red incluida; no es el CPU del nodo.
              </figcaption>
              <TiemposPorNodo nodos={nodos} rondas={corrida.rondas} mostradas={mostradas} total={total} />
            </figure>
          </>
        )}
      </div>
    </Tarjeta>
  )
}
