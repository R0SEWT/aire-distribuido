// Entrenamiento (spec §5.5, solo admin): lanzar el K-Means distribuido con
// parámetros de la UI, ver sus rondas, su resultado y el historial.
//
// Un entrenamiento real dura unos 50 ms. Las rondas llegan por el WebSocket
// casi juntas (a veces antes que el 202 con el id) y la pantalla las reproduce
// a ritmo humano, diciéndolo. Qué se sigue y qué llegó vive en un reductor
// (entrenamiento/seguimiento.ts); el ritmo, en useReproduccion.

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useReducer, useRef, useState } from 'react'
import { api, claves, pedir } from '../api/cliente'
import { EN_VIVO } from '../api/enVivo'
import { useEstadoWS, useEvento } from '../api/tiempoReal'
import { Encabezado, Estado, EstadoDeError, Tarjeta } from '../componentes/indice'
import { Formulario, type Bloqueo } from './entrenamiento/Formulario'
import { Historial } from './entrenamiento/Historial'
import { PanelDeRondas } from './entrenamiento/PanelDeRondas'
import { Resultado } from './entrenamiento/Resultado'
import { reducir, SIN_SEGUIMIENTO, type Corrida, type Fin, type Pedido } from './entrenamiento/seguimiento'
import { PASO_MS, useReproduccion, type Velocidad } from './entrenamiento/useReproduccion'
import { useVentana } from './entrenamiento/useVentana'

/**
 * Cuánto se espera, como mucho, a que los nodos vuelvan al pool después de un
 * entrenamiento: se desconectan al cerrar la corrida y vuelven a llamar a la
 * API tras su pausa entre intentos (unos 2 s). Pasado este margen, el
 * formulario deja de esperar: un nodo que se cayó no va a volver solo.
 */
const ESPERA_DE_NODOS_MS = 6_000

function pedirDetalle(id: string, signal?: AbortSignal) {
  return pedir(api.GET('/api/entrenamientos/{id}', { params: { path: { id } }, signal }))
}

/** Los nodos de una corrida: los que nombran sus rondas, en orden. */
function nodosDeLasRondas(corrida: Corrida | null): string[] {
  const nombres = new Set<string>()
  for (const ronda of corrida?.rondas ?? []) for (const nodo of Object.keys(ronda.ms_por_nodo)) nombres.add(nodo)
  return [...nombres].sort((a, b) => a.localeCompare(b))
}

export default function Entrenamiento() {
  const consultas = useQueryClient()
  const tiempoReal = useEstadoWS()
  const [seguimiento, despachar] = useReducer(reducir, SIN_SEGUIMIENTO)
  const [velocidad, setVelocidad] = useState<Velocidad>('normal')
  const [abriendo, setAbriendo] = useState<string>()
  const [errorAlAbrir, setErrorAlAbrir] = useState<unknown>(null)
  const panel = useRef<HTMLDivElement>(null)

  const { corrida } = seguimiento
  const enCurso = corrida !== null && corrida.fin === null
  const esperandoNodos = useVentana(corrida?.finEn ?? null, ESPERA_DE_NODOS_MS)

  const nodos = useQuery({
    queryKey: claves.nodos,
    meta: EN_VIVO,
    queryFn: ({ signal }) => pedir(api.GET('/api/cluster/nodos', { signal })),
    // Mientras los nodos vuelven al pool se pregunta seguido, para soltar el botón apenas estén.
    refetchInterval: esperandoNodos ? 500 : 3_000,
  })
  const modelo = useQuery({
    queryKey: claves.modeloActivo,
    queryFn: ({ signal }) => pedir(api.GET('/api/modelos/activo', { signal })),
  })
  const historial = useQuery({
    queryKey: claves.entrenamientos,
    meta: EN_VIVO,
    queryFn: ({ signal }) => pedir(api.GET('/api/entrenamientos', { signal })),
    // Con uno en curso que esta pantalla no sigue, se pregunta hasta verlo terminar.
    refetchInterval: (consulta) => (consulta.state.data?.[0]?.estado === 'en_curso' ? 2_000 : false),
  })
  // El detalle se pide al terminar (spec §5.5). Con el WebSocket caído se pide
  // también mientras corre: es la única forma de enterarse de las rondas y del fin.
  const detalle = useQuery({
    queryKey: claves.entrenamiento(corrida?.id ?? ''),
    enabled: corrida !== null && (corrida.fin !== null || tiempoReal !== 'conectada'),
    queryFn: async ({ signal, queryKey }) => {
      const d = await pedirDetalle(queryKey[1], signal)
      despachar({ tipo: 'detalle', detalle: d, ahora: Date.now() })
      return d
    },
    refetchInterval: enCurso ? 1_000 : false,
  })

  /** Lo que un entrenamiento cambia fuera de esta pantalla: historial, modelo activo, pool, métricas y eventos. */
  function refrescar() {
    for (const clave of [claves.entrenamientos, claves.modeloActivo, claves.nodos, claves.salud, claves.metricas, claves.eventosCluster]) {
      void consultas.invalidateQueries({ queryKey: clave })
    }
  }

  useEvento('entrenamiento.ronda', (evento) => despachar({ tipo: 'ronda', evento }))
  useEvento('entrenamiento.fin', (evento) => {
    despachar({ tipo: 'fin', evento, ahora: Date.now() })
    refrescar()
  })

  const lanzar = useMutation({
    mutationFn: (pedido: Pedido) => pedir(api.POST('/api/entrenamientos', { body: pedido })),
    onMutate: (pedido) => despachar({ tipo: 'lanzar', pedido }),
    onSuccess: (aceptado) => {
      despachar({ tipo: 'aceptado', id: aceptado.id })
      refrescar()
    },
    onError: () => {
      despachar({ tipo: 'rechazado' })
      refrescar()
    },
  })

  const reproduccion = useReproduccion(
    corrida?.id ?? null,
    corrida?.rondas.length ?? 0,
    PASO_MS[velocidad],
    corrida?.origen === 'historial',
  )

  async function ver(id: string) {
    setAbriendo(id)
    setErrorAlAbrir(null)
    try {
      const d = await consultas.fetchQuery({ queryKey: claves.entrenamiento(id), queryFn: () => pedirDetalle(id), staleTime: 0 })
      despachar({ tipo: 'ver', detalle: d })
      panel.current?.scrollIntoView?.({ behavior: 'smooth', block: 'start' })
    } catch (error) {
      setErrorAlAbrir(error)
    } finally {
      setAbriendo(undefined)
    }
  }

  const delDetalle = detalle.data?.id === corrida?.id ? detalle.data : undefined
  const nodosDeLaCorrida = delDetalle?.nodos ?? nodosDeLasRondas(corrida)
  const libres = nodos.data?.filter((n) => n.libre).length ?? 0
  // Cuántos nodos tienen que volver: los que usó el entrenamiento que acaba de terminar.
  const porVolver = nodosDeLaCorrida.length || corrida?.pedido?.nodos || 1
  const volviendo = esperandoNodos && libres < porVolver

  const bloqueo: Bloqueo | null = seguimiento.lanzando
    ? { motivo: 'lanzando', texto: 'Enviando el pedido a la API…' }
    : enCurso
      ? {
          motivo: 'en_curso',
          texto: `Hay un entrenamiento en curso (${corrida.id}). La API entrena de a uno: se puede lanzar otro cuando termine.`,
        }
      : volviendo
        ? {
            motivo: 'volviendo',
            texto: `Los nodos están volviendo al pool: al terminar un entrenamiento se desconectan y vuelven a llamar a la API unos 2 segundos después. Volvieron ${libres} de ${porVolver}.`,
          }
        : null

  const termino = corrida !== null && corrida.fin !== null && reproduccion.mostradas >= corrida.rondas.length

  return (
    <div className="space-y-5">
      <Encabezado
        titulo="Entrenamiento"
        bajada="El K-Means se reparte entre los nodos del clúster. Un entrenamiento real dura unos 50 milisegundos: sus rondas se reproducen aquí más despacio."
      />

      <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,22rem)_minmax(0,1fr)]">
        <Formulario
            nodos={nodos.data}
            errorDeNodos={nodos.error}
            alReintentarNodos={() => void nodos.refetch()}
            nodosSugeridos={libres > 0 ? libres : porVolver}
            modeloActivo={modelo.data?.id}
            bloqueo={bloqueo}
            rechazo={lanzar.error}
            sinTiempoReal={tiempoReal !== 'conectada'}
            alLanzar={(pedido) => lanzar.mutate(pedido)}
        />

        <div ref={panel} className="scroll-mt-4 space-y-5">
          {errorAlAbrir !== null && <EstadoDeError error={errorAlAbrir} />}
          {/* El resultado va arriba de las rondas: al terminar la reproducción es lo primero que se lee. */}
          {termino && (
            <Resultado
              corrida={corrida as Corrida & { fin: Fin }}
              detalle={delDetalle}
              errorDeDetalle={detalle.error}
              nodos={nodosDeLaCorrida}
              modeloActivo={modelo.data?.id}
            />
          )}
          {corrida ? (
            <PanelDeRondas
              corrida={corrida}
              nodos={nodosDeLaCorrida}
              reproduccion={reproduccion}
              velocidad={velocidad}
              alCambiarVelocidad={setVelocidad}
            />
          ) : seguimiento.lanzando ? (
            <Estado tipo="cargando" titulo="Enviando el pedido a la API…" />
          ) : (
            <Estado tipo="vacio" titulo="Todavía no hay un entrenamiento a la vista">
              Lanza uno con el formulario o abre uno del historial. Sus rondas se van a ver aquí: la inercia, que baja
              hasta converger, y lo que tardó cada nodo en responder.
            </Estado>
          )}
        </div>
      </div>

      <Tarjeta titulo="Historial de entrenamientos" aLaDerecha="Del más reciente al más antiguo">
        {historial.isPending ? (
          <Estado tipo="cargando" titulo="Cargando el historial…" compacto />
        ) : historial.isError ? (
          <EstadoDeError error={historial.error} alReintentar={() => void historial.refetch()} compacto />
        ) : historial.data.length === 0 ? (
          <Estado tipo="vacio" titulo="Todavía no se lanzó ningún entrenamiento" compacto>
            El primero que termine con 1 réplica va a reemplazar al modelo sembrado.
          </Estado>
        ) : (
          <Historial
            entrenamientos={historial.data}
            modeloActivo={modelo.data?.id}
            aLaVista={corrida?.id}
            abriendo={abriendo}
            alVer={(id) => void ver(id)}
          />
        )}
      </Tarjeta>
    </div>
  )
}
