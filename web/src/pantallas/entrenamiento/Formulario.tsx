// El formulario de un entrenamiento (spec §5.5): nodos, réplicas y semilla, con
// los nodos libres a la vista. Valida lo que se sabe sin preguntar y muestra los
// rechazos de la API con sus palabras.

import { useId, useState, type FormEvent } from 'react'
import type { components } from '../../api/contrato'
import { errorDeExcepcion, presentarError, type TipoError } from '../../api/errores'
import { Cifra, Etiqueta, Pildoras, Tarjeta } from '../../componentes/indice'
import { cuantosNodos, REPLICAS_MEDICION, SEMILLA_POR_OMISION, validar, type Campos, type Errores } from './formulario'
import type { Pedido } from './seguimiento'

type Nodo = components['schemas']['Nodo']

/** Por qué no se puede lanzar ahora, dicho a la persona. */
export interface Bloqueo {
  motivo: 'lanzando' | 'en_curso' | 'volviendo'
  texto: string
}

interface Props {
  /** Los nodos del pool; undefined mientras se consultan. */
  nodos: readonly Nodo[] | undefined
  /** El error de la consulta de nodos, si falló. */
  errorDeNodos: unknown
  alReintentarNodos: () => void
  /** Cuántos nodos proponer mientras el campo no se tocó. */
  nodosSugeridos: number
  modeloActivo: string | undefined
  bloqueo: Bloqueo | null
  /** El rechazo del último pedido, si lo hubo. */
  rechazo: unknown
  /** El WebSocket no está conectado: las rondas no se van a ver en vivo. */
  sinTiempoReal: boolean
  alLanzar: (pedido: Pedido) => void
}

// El título de cada rechazo, propio de este formulario; el detalle son las palabras de la API.
const TITULO_DE_RECHAZO: Partial<Record<TipoError, string>> = {
  conflicto: 'Ya hay un entrenamiento en curso',
  pedido_invalido: 'La API rechazó el pedido',
  no_disponible: 'La API no puede entrenar ahora',
}

const QUE_HACER: Partial<Record<TipoError, string>> = {
  conflicto: 'La API entrena de a uno. Cuando termine, el historial lo muestra y se puede lanzar otro.',
  no_disponible: 'No es un problema del formulario: hay que corregirlo del lado de la API.',
}

function Rechazo({ error }: { error: unknown }) {
  const e = errorDeExcepcion(error)
  const p = presentarError(e)
  return (
    <div role="alert" className="bg-relieve border-l-inca-cuidado rounded-lg border-l-4 px-4 py-3 text-sm">
      <p className="text-texto font-semibold">{TITULO_DE_RECHAZO[e.tipo] ?? p.titulo}</p>
      {e.detalle ? (
        <p className="text-texto mt-1">
          La API dice: <span className="font-semibold">«{e.detalle}»</span>
        </p>
      ) : (
        p.detalle && <p className="text-suave mt-1">{p.detalle}</p>
      )}
      {QUE_HACER[e.tipo] && <p className="text-suave mt-1">{QUE_HACER[e.tipo]}</p>}
    </div>
  )
}

type Preajuste = 'modelo' | 'medicion' | 'otro'

export function Formulario({
  nodos,
  errorDeNodos,
  alReintentarNodos,
  nodosSugeridos,
  modeloActivo,
  bloqueo,
  rechazo,
  sinTiempoReal,
  alLanzar,
}: Props) {
  const id = useId()
  // null: el campo no se tocó y sigue a los nodos libres.
  const [nodosEscritos, setNodosEscritos] = useState<string | null>(null)
  const [replicas, setReplicas] = useState('1')
  const [semilla, setSemilla] = useState(String(SEMILLA_POR_OMISION))
  const [errores, setErrores] = useState<Errores>({})

  const campos: Campos = { nodos: nodosEscritos ?? String(nodosSugeridos), replicas, semilla }
  const libres = nodos?.filter((n) => n.libre) ?? []
  const preajuste: Preajuste = replicas.trim() === '1' ? 'modelo' : replicas.trim() === String(REPLICAS_MEDICION) ? 'medicion' : 'otro'
  const replicasPedidas = /^\d+$/.test(replicas.trim()) ? Number(replicas.trim()) : null
  const nodosPedidos = /^\d+$/.test(campos.nodos.trim()) ? Number(campos.nodos.trim()) : null
  // Con un entrenamiento en curso o los nodos volviendo, que falten nodos es lo esperado: no se avisa.
  const faltanNodos = nodos !== undefined && nodosPedidos !== null && nodosPedidos > libres.length && bloqueo === null

  function enviar(evento: FormEvent) {
    evento.preventDefault()
    if (bloqueo) return
    const resultado = validar(campos)
    setErrores(resultado.errores ?? {})
    if (resultado.pedido) alLanzar(resultado.pedido)
  }

  function campo(nombre: keyof Campos, etiqueta: string, valor: string, alCambiar: (v: string) => void) {
    const error = errores[nombre]
    return (
      <div>
        <label htmlFor={`${id}-${nombre}`} className="rotulo mb-1 block">
          {etiqueta}
        </label>
        <input
          id={`${id}-${nombre}`}
          name={nombre}
          className="campo"
          inputMode="numeric"
          autoComplete="off"
          value={valor}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? `${id}-${nombre}-error` : undefined}
          onChange={(e) => {
            alCambiar(e.target.value)
            if (error) setErrores((antes) => ({ ...antes, [nombre]: undefined }))
          }}
        />
        {error && (
          <p id={`${id}-${nombre}-error`} className="text-falla mt-1 text-xs font-semibold">
            {error}
          </p>
        )}
      </div>
    )
  }

  return (
    <Tarjeta acento="acento" rotulo="K-Means distribuido" titulo="Nuevo entrenamiento">
      <form aria-label="Nuevo entrenamiento" noValidate onSubmit={enviar} className="space-y-4">
        <div data-pool={bloqueo?.motivo === 'volviendo' ? 'volviendo' : 'quieto'}>
          {errorDeNodos && nodos === undefined ? (
            <p role="alert" className="text-falla text-sm font-semibold">
              No se pudo consultar el pool de nodos: {presentarError(errorDeNodos).titulo.toLowerCase()}.{' '}
              <button type="button" className="text-acento-claro underline" onClick={alReintentarNodos}>
                Reintentar
              </button>
            </p>
          ) : bloqueo?.motivo === 'volviendo' ? (
            <Cifra tamano="mediana" rotulo="Nodos libres en el pool" valor={null} sinDato="volviendo al pool…" />
          ) : (
            <Cifra
              tamano="mediana"
              rotulo="Nodos libres en el pool"
              valor={nodos === undefined ? null : libres.length}
              unidad={nodos && nodos.length !== libres.length ? `de ${nodos.length} en el pool` : libres.length === 1 ? 'nodo' : 'nodos'}
              sinDato="consultando…"
            />
          )}
          {libres.length > 0 && bloqueo?.motivo !== 'volviendo' && (
            <ul aria-label="Nodos libres" className="mt-2 flex flex-wrap gap-1.5">
              {libres.map((n) => (
                <li key={n.nombre}>
                  <Etiqueta>
                    <span className="font-mono">{n.nombre}</span>
                    <span className="text-tenue ml-1.5 font-normal">{n.workers} workers</span>
                  </Etiqueta>
                </li>
              ))}
            </ul>
          )}
          {nodos !== undefined && nodos.length === 0 && bloqueo?.motivo !== 'volviendo' && (
            <p className="text-suave mt-2 text-sm">
              No hay ningún nodo conectado. Se levantan con <code className="font-mono">nodo -coordinador IP:7070</code>.
            </p>
          )}
        </div>

        <Pildoras<Preajuste>
          etiqueta="Preajuste"
          valor={preajuste}
          alCambiar={(p) => setReplicas(p === 'medicion' ? String(REPLICAS_MEDICION) : '1')}
          opciones={[
            { valor: 'modelo', etiqueta: 'Entrenar el modelo' },
            { valor: 'medicion', etiqueta: 'Medición larga' },
          ]}
        />

        <div className="grid grid-cols-3 gap-3">
          {campo('nodos', 'Nodos', campos.nodos, setNodosEscritos)}
          {campo('replicas', 'Réplicas', replicas, setReplicas)}
          {campo('semilla', 'Semilla', semilla, setSemilla)}
        </div>

        <p className="text-suave text-sm leading-snug" data-nota-replicas>
          {replicasPedidas !== null && replicasPedidas > 1 ? (
            <>
              Con <strong className="text-texto">{replicasPedidas} réplicas es una medición</strong>: {replicasPedidas}{' '}
              copias del gold, para verla avanzar. <strong className="text-texto">No cambia el modelo activo</strong>.
            </>
          ) : (
            <>Con 1 réplica se entrena sobre el gold: si coincide con el K-Means secuencial, el modelo nuevo queda activo.</>
          )}
        </p>

        {faltanNodos && (
          <p className="text-aviso text-sm font-semibold" data-aviso="faltan-nodos">
            Hay {cuantosNodos(libres.length)} {libres.length === 1 ? 'libre' : 'libres'} y pides {nodosPedidos}: la API lo va a
            rechazar.
          </p>
        )}
        {sinTiempoReal && (
          <p className="text-aviso text-sm font-semibold">
            El tiempo real está desconectado: las rondas no se verán llegar; se piden a la API cuando termine.
          </p>
        )}

        {rechazo !== null && rechazo !== undefined && <Rechazo error={rechazo} />}

        <button type="submit" className="boton w-full" disabled={bloqueo !== null}>
          {bloqueo?.motivo === 'lanzando' ? 'Enviando el pedido…' : 'Lanzar entrenamiento'}
        </button>
        {bloqueo && bloqueo.motivo !== 'lanzando' && (
          <p role="status" className="text-suave text-sm leading-snug" data-bloqueo={bloqueo.motivo}>
            {bloqueo.texto}
          </p>
        )}

        <p className="border-borde text-tenue border-t pt-3 text-xs">
          Modelo activo: <span className="text-suave font-mono">{modeloActivo ?? '—'}</span>
        </p>
      </form>
    </Tarjeta>
  )
}
