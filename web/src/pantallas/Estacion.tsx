// Estación (spec §5.3): el último estado RECIBIDO de una estación, la curva de
// lo que esta pestaña recibió de ella y le envió durante la sesión, y su ficha.
// No hay una serie histórica en la API: la curva no inventa una.
//
// Se actualiza sola: el estado sale de la caché en vivo (web/README.md).

import { useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { useImpacto, type Impacto } from '../api/datos'
import { useEstaciones, useEstado, type Estacion as FichaEstacion, type RegimenEstacion } from '../api/estaciones'
import { useHistoria } from '../api/historia'
import { useLecturasVistas } from '../api/vistas'
import { RUTAS } from '../armazon/menu'
import {
  Banda,
  Cifra,
  decimalesFrenteA,
  Desplegable,
  Encabezado,
  Estado,
  EstadoDeError,
  Etiqueta,
  fechaHora,
  Frase,
  nombrePropio,
  Pildoras,
  Tarjeta,
} from '../componentes/indice'
import { CurvaSesion } from '../graficos/CurvaSesion'
import { armarCurva, textosDeCurva } from './estacion/curva'
import { Ficha } from './estacion/Ficha'
import { contaminanteInicial, fraseDelRegimen, fraseEstacion, NORMA, type Contaminante } from './estacion/norma'
import { filaDe, MINIMO_LECTURAS, tonoDe } from './panel/situacion'

const enlaceDe = (id: string) => `${RUTAS.estacion}/${encodeURIComponent(id)}`

function Selector({
  estaciones,
  sinPM,
  elegida,
}: {
  estaciones: readonly FichaEstacion[]
  sinPM: ReadonlySet<string>
  elegida: string | undefined
}) {
  const navegar = useNavigate()
  const opcion = (e: FichaEstacion) => (
    <option key={e.id} value={e.id}>
      {e.id} · {nombrePropio(e.distrito) ?? 'distrito sin dato'}
    </option>
  )
  return (
    <label className="flex items-center gap-3">
      <span className="rotulo">Elegir estación</span>
      <select
        className="campo w-auto min-w-64"
        value={elegida && estaciones.some((e) => e.id === elegida) ? elegida : ''}
        onChange={(e) => e.target.value && void navegar(enlaceDe(e.target.value))}
      >
        <option value="">— elige una —</option>
        <optgroup label="Miden PM por hora">{estaciones.filter((e) => !sinPM.has(e.id)).map(opcion)}</optgroup>
        <optgroup label="No miden PM por hora">{estaciones.filter((e) => sinPM.has(e.id)).map(opcion)}</optgroup>
      </select>
    </label>
  )
}

/** La regla y la norma detrás de las cifras, y por qué el régimen no es la alerta. */
function DeDondeSale() {
  return (
    <Desplegable titulo="¿De dónde sale este número?">
      <p>
        Cada cifra es la <strong>media de las lecturas horarias</strong> de la estación en las 24 horas que terminan en la
        fecha del estado. Con menos de {MINIMO_LECTURAS} lecturas en esa ventana no se calcula: no hay media, ni banda,
        ni alerta.
      </p>
      <p>
        Hay <strong>alerta</strong> cuando la media supera el ECA de 24 horas (DS 003-2017-MINAM): más de{' '}
        {NORMA.pm2_5.eca} µg/m³ de PM2.5 o más de {NORMA.pm10.eca} µg/m³ de PM10.
      </p>
      <p>
        La <strong>banda INCA</strong> (RM 181-2016-MINAM) se informa aparte del ECA y es la peor entre la de PM10 (cortes
        en {NORMA.pm10.cortes.join(', ')} µg/m³) y la de PM2.5 (cortes en {NORMA.pm2_5.cortes.join(', ')} µg/m³).
      </p>
      <p>
        El <strong>régimen</strong> no es la alerta: es el grupo de lecturas parecidas al que el modelo asignó la última
        lectura horaria. Que un régimen «exceda el ECA» describe la media de todas las lecturas de ese grupo, no la de esta
        estación: una estación puede estar en alerta con su última lectura en un grupo que no lo excede, y al revés. Se
        muestra como quedó guardado con el estado: si después se entrenó otro modelo, los números de régimen pueden haber
        cambiado.
      </p>
    </Desplegable>
  )
}

function TarjetaPrincipal({ estacion, estado, midePM }: { estacion: FichaEstacion; estado: RegimenEstacion; midePM: boolean }) {
  const fila = filaDe(estacion, estado, midePM)
  const { pm2_5, pm10, lecturas } = estado.media_24h
  const distrito = nombrePropio(estacion.distrito)
  return (
    <Tarjeta
      acento={tonoDe(fila)}
      rotulo={`${estacion.id} · último estado recibido`}
      titulo={distrito ? `${distrito}, ${nombrePropio(estacion.departamento)}` : (estacion.objeto_fiscalizado ?? estacion.id)}
      aLaDerecha={
        <>
          Estado del <span className="text-texto font-mono font-semibold">{fechaHora(estado.fecha_hora)}</span>
        </>
      }
    >
      <div className="space-y-4">
        <div className="flex flex-wrap items-start gap-x-10 gap-y-4">
          {/* Con un decimal, o con los que hagan falta para que se vea de qué lado del ECA cae (50.03, no 50.0). */}
          <Cifra
            rotulo="PM2.5 · media de 24 h"
            valor={pm2_5}
            decimales={decimalesFrenteA(pm2_5, NORMA.pm2_5.eca)}
            unidad="µg/m³"
            sinDato="sin datos suficientes"
          />
          <Cifra
            rotulo="PM10 · media de 24 h"
            valor={pm10}
            decimales={decimalesFrenteA(pm10, NORMA.pm10.eca)}
            unidad="µg/m³"
            sinDato="sin datos suficientes"
          />
          <dl className="grid grid-cols-[auto_1fr] items-center gap-x-3 gap-y-1.5 text-sm" data-estado-estacion>
            <dt className="rotulo">Banda</dt>
            <dd className="flex flex-wrap items-center gap-2">
              <Banda valor={fila.banda} conPrefijo />
              {estado.alerta ? <Etiqueta tono="alerta">En alerta</Etiqueta> : <Etiqueta>Sin alerta</Etiqueta>}
            </dd>
            <dt className="rotulo">Ventana</dt>
            <dd className="text-texto">
              <span className="font-mono font-semibold tabular-nums">{lecturas}</span> de 24 lecturas
            </dd>
            <dt className="rotulo">Régimen</dt>
            <dd className="text-texto max-w-md">
              <span className="font-mono font-semibold tabular-nums">{estado.regimen}</span>, el grupo de la última lectura
              horaria
              <span className="text-suave mt-0.5 block text-xs leading-snug" data-regimen-eca={estado.excede_eca}>
                {fraseDelRegimen(estado.excede_eca)}
              </span>
            </dd>
          </dl>
        </div>
        <Frase tono={estado.alerta ? 'alerta' : fila.situacion === 'con_estado' ? tonoDe(fila) : 'acento'}>
          {fraseEstacion(estado)}
        </Frase>
        <DeDondeSale />
      </div>
    </Tarjeta>
  )
}

const OPCIONES = [
  { valor: 'pm2_5', etiqueta: 'PM2.5' },
  { valor: 'pm10', etiqueta: 'PM10' },
] as const

function TarjetaCurva({ id, estado }: { id: string; estado: RegimenEstacion | null }) {
  const historia = useHistoria(id)
  const vistas = useLecturasVistas(id)
  const [elegido, setElegido] = useState<Contaminante | null>(null)
  const contaminante = elegido ?? contaminanteInicial(estado)
  const norma = NORMA[contaminante]
  const curva = useMemo(() => armarCurva(historia, vistas, contaminante), [historia, vistas, contaminante])
  const textos = textosDeCurva(id, curva, contaminante)

  return (
    <Tarjeta
      rotulo={textos.rotulo}
      titulo={textos.titulo}
      aLaDerecha={<Pildoras etiqueta="Contaminante de la curva" opciones={OPCIONES} valor={contaminante} alCambiar={setElegido} />}
    >
      {curva.vacia ? (
        <p className="text-suave py-8 text-center text-sm">
          Esta pestaña todavía no recibió ningún estado de {id} ni le envió lecturas: la curva se arma con lo que vaya
          llegando.
        </p>
      ) : (
        <>
          <CurvaSesion
            puntos={curva.medias}
            lecturas={curva.lecturas}
            eca={norma.eca}
            cortes={norma.cortes}
            className="h-80"
            titulo={textos.descripcion}
          />
          <p className="text-suave mt-3 text-xs" data-curva-pie data-curva={curva.lecturas.length > 0 ? 'lecturas-y-media' : 'media'}>
            <span className="text-texto font-semibold">{textos.cuenta}</span>
            {textos.pie}
          </p>
        </>
      )}
    </Tarjeta>
  )
}

function Detalle({ estacion, impacto }: { estacion: FichaEstacion; impacto: Impacto }) {
  const navegar = useNavigate()
  const midePM = !impacto.alcance.estaciones_sin_pm_horario.includes(estacion.id)
  const estado = useEstado(estacion.id)

  return (
    <div className="space-y-5">
      {estado.data ? (
        <TarjetaPrincipal estacion={estacion} estado={estado.data} midePM={midePM} />
      ) : estado.isError ? (
        <EstadoDeError error={estado.error} alReintentar={() => void estado.refetch()} />
      ) : estado.isPending ? (
        <Estado tipo="cargando" titulo={`Cargando el último estado de ${estacion.id}…`} />
      ) : midePM ? (
        <Estado
          tipo="vacio"
          titulo={`${estacion.id} todavía no tiene estado`}
          accion={{ etiqueta: 'Ir a simulación', alHacer: () => void navegar(RUTAS.simulacion) }}
        >
          El sistema no recibió ninguna lectura de esta estación desde que se levantó. Cuando llegue una, esta página se
          actualiza sola.
        </Estado>
      ) : (
        <Estado tipo="vacio" titulo={`${estacion.id} no mide PM por hora`}>
          Es de muestreo manual de 24 horas o no tiene PM2.5 y PM10 horarios: no entra al modelo y nunca va a tener
          estado. Que no mida no quiere decir que ahí el aire esté limpio.
        </Estado>
      )}
      <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(0,24rem)]">
        {(midePM || estado.data) && <TarjetaCurva id={estacion.id} estado={estado.data ?? null} />}
        <div className={midePM || estado.data ? '' : 'lg:col-span-2'}>
          <Ficha estacion={estacion} impacto={impacto} midePM={midePM} />
        </div>
      </div>
    </div>
  )
}

export default function Estacion() {
  const { id } = useParams()
  const estaciones = useEstaciones()
  const impacto = useImpacto()
  const consultas = [estaciones, impacto]
  const fallida = consultas.find((c) => c.data === undefined && c.isError)
  const sinPM = useMemo(() => new Set(impacto.data?.alcance.estaciones_sin_pm_horario), [impacto.data])
  const estacion = id ? estaciones.data?.find((e) => e.id === id) : undefined

  return (
    <div className="space-y-5">
      <Encabezado
        titulo="Estación"
        junto={estacion && <p className="text-acento-claro font-mono text-xl font-semibold">{estacion.id}</p>}
        bajada="El último estado recibido de una estación, la curva de lo que esta sesión vio de ella y su ficha."
      >
        {estaciones.data && impacto.data && <Selector estaciones={estaciones.data} sinPM={sinPM} elegida={id} />}
      </Encabezado>
      {fallida ? (
        <EstadoDeError
          error={fallida.error}
          alReintentar={() => consultas.filter((c) => c.isError).forEach((c) => void c.refetch())}
        />
      ) : !estaciones.data || !impacto.data ? (
        <Estado tipo="cargando" titulo="Cargando las estaciones…" />
      ) : !id ? (
        <Estado tipo="vacio" titulo="Elige una estación">
          Con el selector de arriba, o desde el mapa y la lista del panel. Se ve su último estado recibido, la curva de lo
          que llegó en esta sesión y su ficha.
        </Estado>
      ) : !estacion ? (
        <Estado tipo="vacio" titulo={`No hay ninguna estación «${id}»`}>
          El catálogo tiene {estaciones.data.length} estaciones y esa no es una de ellas. Elige otra con el selector de
          arriba.
        </Estado>
      ) : (
        // Con otra estación, todo empieza de nuevo: el contaminante elegido es de la que se estaba mirando.
        <Detalle key={estacion.id} estacion={estacion} impacto={impacto.data} />
      )}
    </div>
  )
}
