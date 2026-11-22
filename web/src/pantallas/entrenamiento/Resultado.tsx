// El resultado de un entrenamiento (spec §5.5): iteraciones, motivo de paro,
// diferencia con el secuencial, tiempo total y qué pasó con el modelo; si
// abortó, el error con el nodo que falló. Los números son los del detalle
// (GET /api/entrenamientos/{id}); mientras llega, los del evento de fin.

import type { ReactNode } from 'react'
import { presentarError } from '../../api/errores'
import { duracion, Etiqueta, Frase, numero, Tarjeta } from '../../componentes/indice'
import { cuantosNodos } from './formulario'
import type { Corrida, Detalle, Fin } from './seguimiento'
import { compuerta, cuantasRondas, diferencia, motivoDeParo, nodosNombrados } from './textos'

interface Props {
  corrida: Corrida & { fin: Fin }
  /** El detalle pedido a la API, si ya llegó. */
  detalle: Detalle | undefined
  /** Si el detalle no se pudo pedir, por qué. */
  errorDeDetalle: unknown
  /** Los nodos del entrenamiento, en orden. */
  nodos: readonly string[]
  modeloActivo: string | undefined
}

function Dato({ rotulo, children, titulo }: { rotulo: string; children: ReactNode; titulo?: string }) {
  return (
    <div title={titulo}>
      <dt className="rotulo mb-1">{rotulo}</dt>
      <dd className="text-texto font-mono text-lg leading-tight font-semibold tabular-nums">{children}</dd>
    </div>
  )
}

function NotaDelDetalle({ detalle, error }: Pick<Props, 'detalle'> & { error: unknown }) {
  if (detalle) return null
  if (error) {
    return (
      <p className="text-aviso text-xs font-semibold">
        No se pudo pedir el detalle a la API ({presentarError(error).titulo.toLowerCase()}): estos son los datos del evento
        de fin.
      </p>
    )
  }
  return <p className="text-tenue text-xs">Pidiendo el detalle a la API…</p>
}

export function Resultado({ corrida, detalle, errorDeDetalle, nodos, modeloActivo }: Props) {
  const { fin } = corrida
  const replicas = detalle?.replicas ?? corrida.pedido?.replicas
  const reparto = (
    <p className="text-suave text-sm">
      {nodos.length > 0 ? (
        <>
          {cuantosNodos(nodos.length)}: <span className="font-mono">{nodos.join(', ')}</span>
        </>
      ) : (
        'Nodos: sin dato'
      )}
      {replicas !== undefined && <> · {replicas === 1 ? '1 réplica' : `${numero(replicas)} réplicas`}</>}
    </p>
  )

  if (fin.abortado) {
    const error = detalle?.error ?? fin.error
    const culpables = nodosNombrados(error, nodos)
    return (
      <Tarjeta
        acento="alerta"
        rotulo={<span className="font-mono normal-case">{corrida.id}</span>}
        titulo="El entrenamiento abortó"
        aLaDerecha={<Etiqueta tono="alerta">Abortado</Etiqueta>}
      >
        <div className="grid gap-x-5 gap-y-3 md:grid-cols-2" data-resultado="abortado">
          <div className="space-y-3">
            <Frase tono="alerta">
              {culpables.length === 1
                ? `Falló el nodo ${culpables[0]}.`
                : culpables.length > 1
                  ? `Fallaron los nodos ${culpables.join(', ')}.`
                  : 'La corrida se cortó.'}{' '}
              No se activó ningún modelo: sigue el anterior.
            </Frase>
            <p className="text-suave text-sm">
              {corrida.rondas.length === 0
                ? 'No llegó a completar ninguna ronda.'
                : `Llegó a completar ${cuantasRondas(corrida.rondas.length)}.`}
            </p>
            {reparto}
            <NotaDelDetalle detalle={detalle} error={errorDeDetalle} />
          </div>
          <div>
            <p className="rotulo mb-1">Lo que dice la API</p>
            <p className="bg-fondo border-borde text-texto rounded-lg border px-3 py-2 font-mono text-xs leading-relaxed break-words">
              {error}
            </p>
          </div>
        </div>
      </Tarjeta>
    )
  }

  // El detalle manda; si todavía no llegó, los mismos campos vienen en el evento de fin.
  const datos = detalle?.estado === 'terminado' ? { ...fin, ...detalle } : fin
  const puerta = compuerta({ ...datos, replicas }, modeloActivo)
  return (
    <Tarjeta
      acento={puerta?.tono ?? 'buena'}
      rotulo={<span className="font-mono normal-case">{corrida.id}</span>}
      titulo="Resultado"
      aLaDerecha={<Etiqueta tono="buena">Terminado</Etiqueta>}
    >
      <div className="grid gap-x-5 gap-y-3 md:grid-cols-2" data-resultado="terminado">
        <div className="space-y-3">
          <dl className="grid grid-cols-2 gap-x-4 gap-y-3">
            <Dato rotulo="Iteraciones">{numero(datos.iteraciones)}</Dato>
            <Dato rotulo="Tiempo total real">{duracion(datos.ms_total ?? 0)}</Dato>
            <Dato
              rotulo="Motivo de paro"
              titulo={datos.paro === 'tolerancia' ? 'Los centroides dejaron de moverse (tolerancia)' : 'Llegó a max_iter sin converger'}
            >
              {motivoDeParo(datos.paro)}
            </Dato>
            <Dato rotulo="Dif. con el secuencial" titulo="La mayor diferencia entre los centroides distribuidos y los del K-Means secuencial">
              {diferencia(datos.dif_secuencial)}
            </Dato>
          </dl>
          {reparto}
          <NotaDelDetalle detalle={detalle} error={errorDeDetalle} />
        </div>
        {puerta && (
          <div>
            <Frase tono={puerta.tono}>
              {puerta.titulo}.{' '}
              <span className="text-suave mt-1 block text-sm font-normal break-words">{puerta.detalle}</span>
            </Frase>
          </div>
        )}
      </div>
    </Tarjeta>
  )
}
