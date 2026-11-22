import type { components } from '../../api/contrato'
import { Estado, Etiqueta, numero, Tabla, type Columna } from '../../componentes/indice'
import { milisegundos } from '../../graficos/ejeDeRondas'
import type { RespuestaPorRonda } from './resumen'

type Nodo = components['schemas']['Nodo']

interface Props {
  nodos: readonly Nodo[]
  /** Lo que tardó cada nodo por ronda en el último entrenamiento, si hubo uno con rondas. */
  respuesta: RespuestaPorRonda | null
}

/**
 * Los nodos del pool (spec §5.6): nombre, dirección, workers, latencia y si
 * está libre; y, de la carga de cada uno, lo único que se mide: lo que tardó en
 * responder cada ronda del último entrenamiento.
 */
export function Nodos({ nodos, respuesta }: Props) {
  if (nodos.length === 0) {
    return (
      <Estado tipo="vacio" titulo="No hay nodos en el pool" className="border-0 py-6">
        Justo después de un entrenamiento es normal: los nodos se desconectan y vuelven a llamar a la API unos 2
        segundos después. Si no vuelven, se levantan con <code className="font-mono">nodo -coordinador IP:7070</code>.
      </Estado>
    )
  }
  const columnas: Columna<Nodo>[] = [
    {
      clave: 'nombre',
      titulo: 'Nodo y dirección',
      // La dirección va debajo del nombre y no en otra columna: así la tabla entra también en modo proyector.
      celda: (n) => (
        <>
          <span className="block font-mono font-semibold whitespace-nowrap">{n.nombre}</span>
          <span className="text-suave block font-mono text-xs whitespace-nowrap">{n.direccion}</span>
        </>
      ),
    },
    { clave: 'workers', titulo: 'Workers', numerica: true, celda: (n) => numero(n.workers) },
    { clave: 'latencia', titulo: 'Latido (ms)', numerica: true, celda: (n) => milisegundos(n.latencia_ms) },
    {
      clave: 'respuesta',
      titulo: 'Ronda (ms)',
      numerica: true,
      celda: (n) => {
        const r = respuesta?.porNodo[n.nombre]
        return r ? <span title={`Máximo: ${milisegundos(r.maximo)} ms`}>{milisegundos(r.media)}</span> : '—'
      },
    },
    {
      clave: 'libre',
      titulo: 'Estado',
      celda: (n) => (n.libre ? <Etiqueta tono="buena">Libre</Etiqueta> : <Etiqueta tono="acento">Entrenando</Etiqueta>),
    },
  ]
  return (
    <div className="space-y-3">
      <Tabla titulo="Nodos del pool" columnas={columnas} filas={nodos} claveDe={(n) => n.nombre} />
      <p className="text-tenue text-xs leading-snug" data-nota-nodos>
        <strong className="text-suave">Latido</strong>: la ida y vuelta del último PING de la API al nodo; solo se toma a
        los nodos libres. <strong className="text-suave">Ronda</strong>:{' '}
        {respuesta
          ? `lo que tardó en responder cada ronda, en promedio, en el último entrenamiento (${respuesta.id}, ${respuesta.replicas === 1 ? '1 réplica' : `${numero(respuesta.replicas)} réplicas`}, ${respuesta.rondas === 1 ? '1 ronda' : `${respuesta.rondas} rondas`}), medido desde la API y con la red incluida.`
          : 'lo que tarda en responder cada ronda de un entrenamiento, medido desde la API; todavía no hubo ninguno con rondas.'}{' '}
        Es lo que se sabe de la carga de un nodo: su CPU no viaja por el protocolo.
      </p>
    </div>
  )
}
