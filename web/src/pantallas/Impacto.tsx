// Impacto (spec §5.7 y §7): la parte D del enunciado. Una página para leer de
// arriba abajo, con los datos reales del gold de OEFA: cuánto tiempo se superó
// el ECA, dónde y junto a qué actividad, quién vive ahí, si la alerta sirve y
// con cuánta anticipación, y lo que este dato no puede decir.
//
// Todas las cifras salen de /datos/impacto.json, que escribe scripts/impacto.py.
// Lo que §7.2 prohíbe afirmar no se afirma: la población va fila por fila, como
// contexto, y no hay ninguna suma de personas.

import { useImpacto, type Impacto as DatosDeImpacto } from '../api/datos'
import { Encabezado, Estado, EstadoDeError, fechaDia, numero } from '../componentes/indice'
import { ConCuantaAnticipacion, LoQueNoPuedeDecir, SirveLaAlerta } from './impacto/Alerta'
import { CuantoTiempo, DondeYJuntoAQue, ElProblema, QuienViveAhi } from './impacto/Medido'

export default function Impacto() {
  const impacto = useImpacto()

  return (
    <div className="space-y-5">
      <Encabezado
        titulo="Impacto"
        bajada="Qué dicen las mediciones reales de OEFA sobre el aire junto a las actividades que fiscaliza, y qué no pueden decir."
      />
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
