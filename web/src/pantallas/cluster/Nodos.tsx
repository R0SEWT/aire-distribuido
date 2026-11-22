import type { components } from '../../api/contrato'
import { Estado, Etiqueta, numero, Tabla, type Columna } from '../../componentes/indice'
import { milisegundos } from '../../graficos/ejeDeRondas'

type Nodo = components['schemas']['Nodo']

const COLUMNAS: Columna<Nodo>[] = [
  { clave: 'nombre', titulo: 'Nodo', celda: (n) => <span className="font-mono font-semibold">{n.nombre}</span> },
  { clave: 'direccion', titulo: 'Dirección', celda: (n) => <span className="text-suave font-mono">{n.direccion}</span> },
  { clave: 'workers', titulo: 'Workers', numerica: true, celda: (n) => numero(n.workers) },
  { clave: 'latencia', titulo: 'Latido (ms)', numerica: true, celda: (n) => milisegundos(n.latencia_ms) },
  {
    clave: 'libre',
    titulo: 'Estado',
    celda: (n) => (n.libre ? <Etiqueta tono="buena">Libre</Etiqueta> : <Etiqueta tono="acento">En un entrenamiento</Etiqueta>),
  },
]

/** Los nodos del pool (spec §5.6): nombre, dirección, workers, latencia y si está libre. */
export function Nodos({ nodos }: { nodos: readonly Nodo[] }) {
  if (nodos.length === 0) {
    return (
      <Estado tipo="vacio" titulo="No hay nodos en el pool" className="border-0 py-6">
        Justo después de un entrenamiento es normal: los nodos se desconectan y vuelven a llamar a la API unos 2
        segundos después. Si no vuelven, se levantan con <code className="font-mono">nodo -coordinador IP:7070</code>.
      </Estado>
    )
  }
  return (
    <div className="space-y-3">
      <Tabla titulo="Nodos del pool" columnas={COLUMNAS} filas={nodos} claveDe={(n) => n.nombre} />
      <p className="text-tenue text-xs leading-snug">
        El latido es la ida y vuelta del último PING de la API al nodo; solo se toma a los nodos libres. Los workers son
        los núcleos que el nodo declaró al presentarse.
      </p>
    </div>
  )
}
