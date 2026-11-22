// Clúster (spec §5.6, solo admin): lo que el enunciado pide del panel de
// administrador («uso CPU, latencia, número de nodos, logs»), cada cosa con su
// nombre verdadero:
//
//   - nodos            GET /api/cluster/nodos, y lo que tardó cada uno por ronda
//                      en el último entrenamiento (lo único que se mide de su carga)
//   - uso de CPU       el del PROCESO DE LA API, que es el coordinador; no el de
//                      los nodos, que el protocolo no transporta. /api/metricas
//                      publica el CPU acumulado y el uso sale de dos consultas
//                      seguidas (cluster/cpu.ts)
//   - latencia         de la API, por ruta, con la línea de los 100 ms
//   - escrituras       de predicciones, por almacén
//   - eventos          las altas, bajas y errores de nodos: los «logs»
//
// Todo se vuelve a pedir cada pocos segundos mientras la pantalla está abierta.

import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useState, type ReactNode } from 'react'
import { api, claves, pedir } from '../api/cliente'
import { EN_VIVO } from '../api/enVivo'
import { useEvento } from '../api/tiempoReal'
import { Cifra, Encabezado, Estado, EstadoDeError, horaConSegundos, milisegundos, numero, Pildoras, Tarjeta } from '../componentes/indice'
import { agregarMuestra, sinMedicionDeCpu, usoDeCpu, usoPromedio, type Proceso as Muestra } from './cluster/cpu'
import { Escrituras } from './cluster/Escrituras'
import { Eventos } from './cluster/Eventos'
import { Latencia } from './cluster/Latencia'
import { Nodos } from './cluster/Nodos'
import { Proceso } from './cluster/Proceso'
import { respuestaPorRonda, rutasPorLatencia } from './cluster/resumen'
import { Salud } from './cluster/Salud'

const CADA_MS = 2_000
const CADA_EVENTOS_MS = 3_000

type Limite = '50' | '200'

interface Consulta<T> {
  data: T | undefined
  error: unknown
  isPending: boolean
  refetch: () => unknown
}

/** Lo de adentro de una tarjeta según cómo va su consulta: cargando, error con reintento o los datos. */
function ConDatos<T>({ consulta, children }: { consulta: Consulta<T>; children: (datos: T) => ReactNode }) {
  // Con datos ya recibidos, un refresco que falla no los borra: la barra de arriba dice que la API no responde.
  if (consulta.data !== undefined) return <>{children(consulta.data)}</>
  if (consulta.isPending) return <Estado tipo="cargando" compacto />
  return <EstadoDeError error={consulta.error} alReintentar={() => void consulta.refetch()} compacto />
}

export default function Cluster() {
  const [limite, setLimite] = useState<Limite>('50')
  // Las muestras de proceso_api de esta visita: de cada par sale un punto del uso de CPU.
  const [muestras, setMuestras] = useState<Muestra[]>([])

  const nodos = useQuery({
    queryKey: claves.nodos,
    meta: EN_VIVO,
    queryFn: ({ signal }) => pedir(api.GET('/api/cluster/nodos', { signal })),
    refetchInterval: CADA_MS,
  })
  const metricas = useQuery({
    queryKey: claves.metricas,
    meta: EN_VIVO,
    queryFn: async ({ signal }) => {
      const m = await pedir(api.GET('/api/metricas', { signal }))
      setMuestras((antes) => agregarMuestra(antes, m.proceso_api))
      return m
    },
    refetchInterval: CADA_MS,
  })
  const eventos = useQuery({
    queryKey: [...claves.eventosCluster, limite] as const,
    meta: EN_VIVO,
    queryFn: ({ signal }) => pedir(api.GET('/api/cluster/eventos', { params: { query: { limite: Number(limite) } }, signal })),
    refetchInterval: CADA_EVENTOS_MS,
    // Al cambiar de 50 a 200 se siguen viendo los 50 hasta que lleguen los 200.
    placeholderData: keepPreviousData,
  })
  const salud = useQuery({
    queryKey: claves.salud,
    queryFn: ({ signal }) => pedir(api.GET('/api/salud', { signal })),
    refetchInterval: 10_000,
    retry: false,
  })

  // De la carga de cada nodo solo se mide lo que tardó en responder cada ronda: sale del último entrenamiento.
  const ultimoId = metricas.data?.ultimo_entrenamiento ?? null
  const ultimo = useQuery({
    queryKey: claves.entrenamiento(ultimoId ?? ''),
    enabled: ultimoId !== null,
    queryFn: ({ signal }) => pedir(api.GET('/api/entrenamientos/{id}', { params: { path: { id: ultimoId! } }, signal })),
    // Mientras corre, sus rondas van cambiando; terminado, ya no cambia.
    refetchInterval: (consulta) => (consulta.state.data?.estado === 'en_curso' ? CADA_MS : false),
  })

  // Un entrenamiento que termina mueve los nodos y deja eventos: no se espera al próximo refresco.
  useEvento('entrenamiento.fin', () => {
    void nodos.refetch()
    void eventos.refetch()
    void metricas.refetch()
    void ultimo.refetch()
  })

  const m = metricas.data
  const actual = muestras[muestras.length - 1] ?? m?.proceso_api
  const anterior = muestras[muestras.length - 2]
  const uso = actual ? ((anterior ? usoDeCpu(anterior, actual) : null) ?? usoPromedio(actual)) : null
  const rutas = m ? rutasPorLatencia(m.endpoints) : []
  const libres = nodos.data?.filter((n) => n.libre).length
  const perdidas = m ? m.escrituras_descartadas + m.escrituras_fallidas : undefined

  return (
    <div className="space-y-5">
      <Encabezado
        titulo="Clúster"
        bajada="Los nodos que entrenan, cómo se porta la API que los coordina y lo que fue pasando con ellos."
      >
        <p role="status" className="text-tenue text-sm" data-actualizado>
          {metricas.dataUpdatedAt > 0
            ? `Actualizado a las ${horaConSegundos(metricas.dataUpdatedAt)} · se refresca cada ${CADA_MS / 1000} s`
            : 'Consultando la API…'}
          {metricas.isError && m !== undefined && <span className="text-falla font-semibold"> · el último refresco falló</span>}
        </p>
      </Encabezado>

      <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-4" data-resumen>
        <Tarjeta acento="acento">
          <Cifra
            tamano="mediana"
            rotulo="Nodos en el pool"
            valor={nodos.data?.length}
            unidad={libres === undefined ? undefined : libres === 1 ? '· 1 libre' : `· ${libres} libres`}
            sinDato={nodos.isError ? 'sin respuesta' : 'consultando…'}
          />
        </Tarjeta>
        <Tarjeta acento="acento">
          <Cifra
            tamano="mediana"
            rotulo="CPU del proceso de la API"
            valor={actual && sinMedicionDeCpu(actual) ? null : uso?.porcentaje}
            decimales={1}
            unidad="% de un núcleo"
            sinDato={actual ? 'no se mide aquí' : metricas.isError ? 'sin respuesta' : 'consultando…'}
          />
        </Tarjeta>
        <Tarjeta acento="acento">
          <Cifra
            tamano="mediana"
            rotulo="Latencia de la API · peor p99"
            valor={rutas[0]?.p99_ms}
            decimales={rutas[0] && rutas[0].p99_ms < 10 ? 2 : 0}
            unidad="ms"
            sinDato={m ? 'sin pedidos' : metricas.isError ? 'sin respuesta' : 'consultando…'}
          />
        </Tarjeta>
        <Tarjeta acento={perdidas ? 'alerta' : 'acento'}>
          <Cifra
            tamano="mediana"
            rotulo="Escrituras perdidas"
            valor={perdidas}
            unidad={m ? `de ${numero(m.escritura.mongo.escritas + m.escritura.redis.escritas + (perdidas ?? 0))}` : undefined}
            sinDato={metricas.isError ? 'sin respuesta' : 'consultando…'}
          />
        </Tarjeta>
      </div>

      <div className="grid items-start gap-5 lg:grid-cols-2">
        <div className="space-y-5">
          <Tarjeta rotulo="Número de nodos" titulo="Nodos del pool">
            <ConDatos consulta={nodos}>
              {(datos) => <Nodos nodos={datos} respuesta={ultimo.data ? respuestaPorRonda(ultimo.data) : null} />}
            </ConDatos>
          </Tarjeta>
          <Tarjeta rotulo="GET /api/salud" titulo="Salud de cada componente">
            <Salud salud={salud.isError ? undefined : salud.data} consultando={salud.isPending} />
          </Tarjeta>
        </div>

        <Tarjeta
          rotulo="Los «logs» del clúster"
          titulo="Eventos: altas, bajas y errores de nodos"
          aLaDerecha={
            <Pildoras<Limite>
              etiqueta="Cuántos eventos traer"
              valor={limite}
              alCambiar={setLimite}
              opciones={[
                { valor: '50', etiqueta: 'Últimos 50' },
                { valor: '200', etiqueta: '200' },
              ]}
            />
          }
        >
          <ConDatos consulta={eventos}>{(datos) => <Eventos eventos={datos} />}</ConDatos>
          <p className="text-tenue mt-3 text-xs leading-snug">
            Del más reciente al más antiguo. Después de cada entrenamiento es normal ver una baja y un alta por nodo: el
            nodo deja el pool al cerrar la corrida y vuelve a llamar a la API unos 2 segundos después.
          </p>
        </Tarjeta>
      </div>

      <div className="grid items-start gap-5 lg:grid-cols-2">
        <Tarjeta rotulo="Proceso de la API (el coordinador), no los nodos" titulo="Uso de CPU y memoria">
          <ConDatos consulta={metricas}>{(datos) => <Proceso muestras={muestras.length > 0 ? muestras : [datos.proceso_api]} />}</ConDatos>
        </Tarjeta>
        <Tarjeta rotulo="Predicciones que llegaron a cada almacén" titulo="Escrituras">
          <ConDatos consulta={metricas}>{(datos) => <Escrituras metricas={datos} />}</ConDatos>
        </Tarjeta>
      </div>

      <Tarjeta
        rotulo="Tiempo de respuesta dentro de la API"
        titulo="Latencia de la API por ruta"
        aLaDerecha={rutas[0] ? `la más lenta: ${milisegundos(rutas[0].p99_ms)} ms en p99` : undefined}
      >
        <ConDatos consulta={metricas}>{() => <Latencia rutas={rutas} />}</ConDatos>
      </Tarjeta>
    </div>
  )
}
