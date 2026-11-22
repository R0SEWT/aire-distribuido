import { Etiqueta, Frase, numero, Tabla, type Columna } from '../../componentes/indice'
import { milisegundos } from '../../graficos/ejeDeRondas'
import { EjeDeLatencia, LeyendaDeLatencia, MarcaDeLatencia, OBJETIVO_MS } from '../../graficos/LatenciaPorRuta'
import { decirLatencia, type LatenciaDeRuta } from './resumen'

const COLUMNAS: Columna<LatenciaDeRuta>[] = [
  {
    clave: 'ruta',
    titulo: 'Ruta',
    celda: (r) => (
      <span className="inline-flex flex-wrap items-center gap-2">
        <span className="font-mono font-semibold whitespace-nowrap">{r.ruta}</span>
        {r.p99_ms >= OBJETIVO_MS && <Etiqueta tono="alerta">p99 sobre 100 ms</Etiqueta>}
      </span>
    ),
  },
  { clave: 'pedidos', titulo: 'Pedidos', numerica: true, celda: (r) => numero(r.pedidos) },
  { clave: 'p50', titulo: 'p50 (ms)', numerica: true, celda: (r) => milisegundos(r.p50_ms) },
  { clave: 'p95', titulo: 'p95 (ms)', numerica: true, celda: (r) => milisegundos(r.p95_ms) },
  { clave: 'p99', titulo: 'p99 (ms)', numerica: true, celda: (r) => milisegundos(r.p99_ms) },
  {
    clave: 'marca',
    titulo: <EjeDeLatencia />,
    celda: (r) => <MarcaDeLatencia p50={r.p50_ms} p95={r.p95_ms} p99={r.p99_ms} />,
  },
]

/** La latencia de la API por ruta (spec §5.6): pedidos, p50, p95 y p99, con la línea de los 100 ms. */
export function Latencia({ rutas }: { rutas: readonly LatenciaDeRuta[] }) {
  const dicho = decirLatencia(rutas)
  return (
    <div className="space-y-3">
      {dicho && <Frase tono={dicho.tono}>{dicho.texto}</Frase>}
      <Tabla
        titulo="Latencia de la API por ruta"
        columnas={COLUMNAS}
        filas={rutas}
        claveDe={(r) => r.ruta}
        vacio="Todavía no se midió ningún pedido."
      />
      <p className="flex flex-wrap items-center justify-between gap-x-6 gap-y-1">
        <LeyendaDeLatencia />
        <span className="text-tenue text-xs">
          Tiempo dentro de la API, desde que entra el pedido hasta que el handler termina. Escala logarítmica.
        </span>
      </p>
    </div>
  )
}
