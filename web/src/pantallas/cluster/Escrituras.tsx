import type { components } from '../../api/contrato'
import { Frase, numero, Tabla, type Columna } from '../../componentes/indice'
import { decirEscrituras } from './resumen'

type Metricas = components['schemas']['Metricas']
type Almacen = components['schemas']['EscrituraAlmacen'] & { nombre: string }

function conAviso(valor: number) {
  return valor > 0 ? <span className="text-falla font-semibold">{numero(valor)}</span> : numero(valor)
}

const COLUMNAS: Columna<Almacen>[] = [
  { clave: 'nombre', titulo: 'Almacén', celda: (a) => <span className="font-semibold">{a.nombre}</span> },
  { clave: 'escritas', titulo: 'Escritas', numerica: true, celda: (a) => numero(a.escritas) },
  { clave: 'descartadas', titulo: 'Descartadas', numerica: true, celda: (a) => conAviso(a.descartadas) },
  { clave: 'fallidas', titulo: 'Fallidas', numerica: true, celda: (a) => conAviso(a.fallidas) },
  { clave: 'lotes', titulo: 'Lotes', numerica: true, celda: (a) => numero(a.lotes) },
]

/** Las escrituras de predicciones por almacén (spec §5.6): escritas, descartadas y fallidas. */
export function Escrituras({ metricas }: { metricas: Metricas }) {
  const dicho = decirEscrituras(metricas)
  const puntos = metricas.puntos_de_control
  const filas: Almacen[] = [
    { nombre: 'MongoDB', ...metricas.escritura.mongo },
    { nombre: 'Redis', ...metricas.escritura.redis },
  ]
  return (
    <div className="space-y-3">
      <Frase tono={dicho.tono}>{dicho.texto}</Frase>
      <Tabla titulo="Escrituras de predicciones por almacén" columnas={COLUMNAS} filas={filas} claveDe={(a) => a.nombre} />
      <p className="text-tenue text-xs leading-snug">
        La predicción responde sin esperar al almacén: se encola y se escribe por lotes. Descartada: la cola estaba llena.
        Fallida: el almacén rechazó el lote.
      </p>
      <p className="border-borde text-suave border-t pt-3 text-sm" data-puntos-de-control>
        Puntos de control de los entrenamientos (Redis, uno por ronda):{' '}
        <span className="text-texto font-mono">{numero(puntos.escritos)}</span> escritos,{' '}
        <span className="font-mono">{conAviso(puntos.fallidos)}</span> fallidos y{' '}
        <span className="text-texto font-mono">{numero(puntos.omitidos)}</span> omitidos.
      </p>
    </div>
  )
}
