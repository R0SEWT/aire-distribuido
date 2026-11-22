// Impacto (spec §5.7 y §7): la parte D del enunciado. Una página para leer de
// arriba abajo, con los datos reales del gold de OEFA: cuánto tiempo se superó
// el ECA, dónde y junto a qué actividad, quién vive ahí, si la alerta sirve y
// con cuánta anticipación, y lo que este dato no puede decir.
//
// Todas las cifras salen de /datos/impacto.json, que escribe scripts/impacto.py.
// Lo que §7.2 prohíbe afirmar no se afirma: la población va fila por fila, como
// contexto, y no hay ninguna suma de personas.

import { useQuery } from '@tanstack/react-query'
import { ErrorApi } from '../api/errores'
import { Estado, EstadoDeError, numero } from '../componentes/indice'
import { ConCuantaAnticipacion, LoQueNoPuedeDecir, SirveLaAlerta } from './impacto/Alerta'
import { CuantoTiempo, DondeYJuntoAQue, ElProblema, QuienViveAhi } from './impacto/Medido'
import { esImpacto, type Impacto as DatosDeImpacto } from './impacto/tipos'
import { clavesDatos, DATOS_ESTATICOS, pedirDatos } from './simulacion/datos'
import { fechaDia } from './simulacion/fechas'

export default function Impacto() {
  const impacto = useQuery({
    queryKey: clavesDatos.impacto,
    ...DATOS_ESTATICOS,
    queryFn: async ({ signal }) => {
      const leido = await pedirDatos<unknown>('impacto.json', signal)
      if (!esImpacto(leido)) throw new ErrorApi('inesperado', 200, '/datos/impacto.json no tiene la forma esperada')
      return leido
    },
  })

  return (
    <div className="space-y-5">
      <header>
        <h1 className="text-texto text-2xl font-semibold">Impacto</h1>
        <p className="text-suave mt-1">
          Qué dicen las mediciones reales de OEFA sobre el aire junto a las actividades que fiscaliza, y qué no pueden
          decir.
        </p>
      </header>
      {impacto.isPending ? (
        <Estado tipo="cargando" titulo="Cargando los datos de impacto…" />
      ) : impacto.isError ? (
        <EstadoDeError error={impacto.error} alReintentar={() => void impacto.refetch()} />
      ) : (
        <Pagina impacto={impacto.data} />
      )}
    </div>
  )
}

function Pagina({ impacto }: { impacto: DatosDeImpacto }) {
  const { alcance, totales, metadatos } = impacto
  return (
    <>
      <p className="text-tenue text-sm">
        {numero(totales.lecturas)} lecturas horarias de {numero(alcance.estaciones_con_pm_horario)} estaciones con PM por
        hora, de las {numero(alcance.estaciones_en_la_ficha)} de la ficha de OEFA · calculado el {fechaDia(metadatos.generado)}{' '}
        por <code className="font-mono">{metadatos.generador}</code>
      </p>
      <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
        <ElProblema impacto={impacto} />
        <CuantoTiempo impacto={impacto} />
      </div>
      <DondeYJuntoAQue impacto={impacto} />
      <QuienViveAhi impacto={impacto} />
      <SirveLaAlerta impacto={impacto} />
      <ConCuantaAnticipacion impacto={impacto} />
      <LoQueNoPuedeDecir impacto={impacto} />
    </>
  )
}
