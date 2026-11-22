import { Cifra, Frase, numero } from '../../componentes/indice'
import { UsoDeCpuEnElTiempo } from '../../graficos/UsoDeCpuEnElTiempo'
import { serieDeUso, sinMedicionDeCpu, usoDeCpu, usoPromedio, type Proceso as Muestra } from './cpu'
import { lapso } from './formato'

/** Los segundos que abarca la curva del uso de CPU. */
const VENTANA_S = 180

/**
 * El CPU y la memoria del proceso de la API, que es el coordinador (spec §5.6).
 * No son de los nodos: el protocolo del clúster no transporta su CPU.
 */
export function Proceso({ muestras }: { muestras: readonly Muestra[] }) {
  const actual = muestras[muestras.length - 1]
  if (!actual) return null
  const anterior = muestras[muestras.length - 2]
  const entreDos = anterior ? usoDeCpu(anterior, actual) : null
  const uso = entreDos ?? usoPromedio(actual)
  const sinCpu = sinMedicionDeCpu(actual)

  return (
    <div className="space-y-4">
      {sinCpu ? (
        <Cifra rotulo="Uso de CPU" valor={null} tamano="mediana" sinDato="esta plataforma no mide el tiempo de CPU" />
      ) : (
        <div data-uso-de-cpu={entreDos ? 'entre-dos-muestras' : 'promedio'}>
          <Cifra valor={uso?.porcentaje} decimales={1} unidad="% de un núcleo" sinDato="sin dato" />
          <p className="text-suave mt-2 text-sm">
            {entreDos
              ? `Entre las dos últimas consultas (${numero(entreDos.intervaloS, 1)} s).`
              : 'Promedio desde que arrancó la API: hace falta una segunda consulta para medir el uso de ahora.'}{' '}
            100 % es un núcleo ocupado todo ese tiempo; el tope son {numero(actual.nucleos * 100)} % ({actual.nucleos}{' '}
            núcleos).
          </p>
        </div>
      )}

      {!sinCpu && <UsoDeCpuEnElTiempo serie={serieDeUso(muestras)} ventanaS={VENTANA_S} />}

      <div className="border-borde grid grid-cols-2 gap-x-4 gap-y-3 border-t pt-4 sm:grid-cols-4">
        <Cifra tamano="chica" rotulo="Heap vivo" valor={actual.heap_mb} decimales={1} unidad="MB" />
        <Cifra tamano="chica" rotulo="Pedida al sistema" valor={actual.sistema_mb} decimales={1} unidad="MB" />
        <Cifra
          tamano="chica"
          rotulo="Máx. residente"
          valor={actual.no_disponible?.includes('max_rss_mb') ? null : actual.max_rss_mb}
          decimales={1}
          unidad="MB"
          sinDato="no se mide"
        />
        <Cifra tamano="chica" rotulo="Goroutines" valor={actual.goroutines} />
      </div>

      <p className="text-tenue text-xs leading-snug">
        En marcha hace {lapso(actual.en_marcha_s)}
        {!sinCpu && (
          <>
            {' '}
            · CPU acumulado: {numero(actual.cpu_usuario_s, 2)} s de usuario y {numero(actual.cpu_sistema_s, 2)} s de sistema
          </>
        )}
        .
      </p>

      <Frase>
        Es el CPU del proceso de la API, que coordina el clúster; no el de los nodos.{' '}
        <span className="text-suave mt-1 block text-sm font-normal">
          El protocolo del clúster no transporta el CPU de cada nodo. De ellos se mide lo que tardan en responder cada
          ronda (en Entrenamiento) y la latencia de su latido.
        </span>
      </Frase>
    </div>
  )
}
