// Panel (spec §5.2): el último estado RECIBIDO de cada estación, en el mapa y
// en una lista ordenada por gravedad. No hay un «ahora»: los datos de OEFA van
// de 2018 a 2022 y el sistema muestra lo que alguien le mandó, con su fecha.
//
// Se actualiza sola: la lista de estados sale de la caché en vivo, que el
// armazón mantiene al día con el evento `estacion.regimen` (web/README.md).

import { useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { RUTAS } from '../armazon/menu'
import type { EstadoEstacion } from '../api/enVivo'
import {
  Banda,
  BANDAS,
  Cifra,
  Desplegable,
  Estado,
  EstadoDeError,
  Etiqueta,
  fechaHora,
  Frase,
  numero,
  Tabla,
  Tarjeta,
  type Columna,
} from '../componentes/indice'
import { MapaPeru, type LugarEnMapa, type MarcaEnMapa } from '../graficos/MapaPeru'
import { SOLO_ADMIN, puede } from '../sesion/roles'
import { useSesion } from '../sesion/useSesion'
import { useRegistroDeSesion } from './estacion/historia'
import { useEstaciones, useEstados, useImpacto, useMapaDelPeru, type Estacion, type Impacto } from './panel/datos'
import { Leyenda } from './panel/Leyenda'
import {
  detalleSinBanda,
  filasDe,
  frasePanel,
  nombrePropio,
  resumir,
  type FilaEstacion,
  type Resumen,
} from './panel/situacion'

// El mapa ocupa lo que queda de la pantalla bajo la barra, para que entre entero sin desplazarse.
const ALTO_DEL_MAPA = 'h-[clamp(21rem,calc(100dvh-19rem),42rem)]'

const enlaceDe = (id: string) => `${RUTAS.estacion}/${encodeURIComponent(id)}`

/** «Lurigancho · ladrillera»: dónde está la estación y qué vigila. */
function lugarDe(e: Estacion): string {
  const distrito = nombrePropio(e.distrito) ?? 'distrito sin dato'
  // La ficha pone «no lo dice el nombre» cuando el tipo no se pudo deducir.
  const actividad = !e.tipo_actividad || e.tipo_actividad === 'no lo dice el nombre' ? 'actividad sin clasificar' : e.tipo_actividad
  return `${distrito} · ${actividad}`
}

/** Lo que se lee de una estación en el mapa: su código, su distrito y su estado con palabras. */
function nombreDeMarca(f: FilaEstacion): string {
  const palabra =
    f.situacion === 'con_estado' ? `INCA ${BANDAS[f.banda].palabra.toLowerCase()}` : BANDAS[f.banda].palabra.toLowerCase()
  return `${f.id}, ${nombrePropio(f.estacion.distrito) ?? 'distrito sin dato'}: ${palabra}${f.alerta ? ', en alerta' : ''}`
}

/** «16 dic 2021» y debajo «00:00»: la fecha de un estado sin ensanchar la tabla. */
function FechaEnDosLineas({ iso }: { iso: string }) {
  const [dia, horaDelDia] = fechaHora(iso).split(', ')
  return (
    <span className="text-suave whitespace-nowrap">
      {dia}
      <span className="block font-mono text-xs tabular-nums">{horaDelDia}</span>
    </span>
  )
}

const COLUMNAS: Columna<FilaEstacion>[] = [
  {
    clave: 'estacion',
    titulo: 'Estación',
    celda: (f) => (
      <div className="min-w-36">
        <Link to={enlaceDe(f.id)} className="font-mono font-semibold whitespace-nowrap">
          {f.id}
        </Link>
        <p className="text-suave mt-0.5 text-xs">{lugarDe(f.estacion)}</p>
      </div>
    ),
  },
  {
    clave: 'pm2_5',
    titulo: (
      <>
        PM2.5
        <br />
        24 h
      </>
    ),
    numerica: true,
    celda: (f) => numero(f.estado?.media_24h.pm2_5, 1),
  },
  {
    clave: 'pm10',
    titulo: (
      <>
        PM10
        <br />
        24 h
      </>
    ),
    numerica: true,
    celda: (f) => numero(f.estado?.media_24h.pm10, 1),
  },
  {
    clave: 'banda',
    titulo: 'Banda INCA',
    celda: (f) => (
      <div>
        <div className="flex flex-wrap items-center gap-1.5">
          <Banda valor={f.banda} />
          {f.alerta && <Etiqueta tono="alerta">En alerta</Etiqueta>}
        </div>
        {f.situacion !== 'con_estado' && <p className="text-suave mt-1 text-xs">{detalleSinBanda(f)}</p>}
      </div>
    ),
  },
  {
    clave: 'regimen',
    titulo: 'Régimen',
    celda: (f) =>
      f.estado ? (
        <div className="whitespace-nowrap">
          <span className="font-mono font-semibold tabular-nums">{f.estado.regimen}</span>
          {f.estado.excede_eca && <p className="text-suave mt-0.5 text-xs">excede el ECA</p>}
        </div>
      ) : (
        '—'
      ),
  },
  {
    clave: 'fecha',
    titulo: 'Estado del',
    celda: (f) => (f.estado ? <FechaEnDosLineas iso={f.estado.fecha_hora} /> : '—'),
  },
]

function Cifras({ resumen }: { resumen: Resumen }) {
  const { ultimo } = resumen
  return (
    <div className="grid gap-4 @6xl:grid-cols-[0.85fr_1.1fr_1.15fr]" data-cifras>
      <Tarjeta acento={resumen.enAlerta > 0 ? 'alerta' : resumen.conEstado > 0 ? 'buena' : undefined} className="p-4!">
        <Cifra
          tamano="mediana"
          rotulo="Estaciones en alerta"
          valor={resumen.enAlerta}
          unidad={`de ${resumen.conEstado} con estado`}
        />
      </Tarjeta>
      <Tarjeta className="p-4!">
        <Cifra
          tamano="mediana"
          rotulo="Estaciones con estado"
          valor={resumen.conEstado}
          unidad={`de ${resumen.miden} que miden PM por hora`}
        />
      </Tarjeta>
      <Tarjeta className="p-4!">
        <p className="rotulo mb-1">Último estado recibido</p>
        {ultimo ? (
          <>
            <p className="text-texto font-mono text-lg leading-8 font-semibold whitespace-nowrap tabular-nums">
              {fechaHora(ultimo.estado.fecha_hora)}
            </p>
            <p className="text-suave mt-1 text-xs">
              <span className="font-mono">{ultimo.id}</span> · llegó el {fechaHora(ultimo.estado.ts)}
            </p>
          </>
        ) : (
          <p className="text-suave">todavía ninguno</p>
        )}
      </Tarjeta>
    </div>
  )
}

/** Por qué no hay nada, y qué hacer (spec §5.2): un sistema recién levantado no tiene estados. */
function SinEstados() {
  const { sesion } = useSesion()
  const navegar = useNavigate()
  const admin = puede(sesion?.rol, SOLO_ADMIN)
  return (
    <Estado
      tipo="vacio"
      titulo="Ninguna estación tiene estado todavía"
      accion={{
        etiqueta: admin ? 'Ir a cargar el escenario' : 'Ir a simulación',
        alHacer: () => void navegar(RUTAS.simulacion),
      }}
      className="py-14"
    >
      <div className="space-y-2">
        <p>
          El sistema no recibe lecturas en vivo: muestra el último estado de las que alguien le manda, y desde que se
          levantó no recibió ninguna. Los datos de OEFA van de 2018 a 2022.
        </p>
        <p>
          {admin
            ? 'Carga el escenario, 48 horas reales de diciembre de 2021, desde Simulación: este panel se llena solo a medida que llegan los estados.'
            : 'Simula un día de una estación, o pide a un administrador que cargue el escenario: este panel se llena solo a medida que llegan los estados.'}
        </p>
      </div>
    </Estado>
  )
}

function TarjetaDelMapa({ filas, lugares }: { filas: readonly FilaEstacion[]; lugares: readonly LugarEnMapa[] }) {
  const mapa = useMapaDelPeru()
  const [senalada, setSenalada] = useState<string | null>(null)
  const marcas = useMemo(
    () => new Map<string, MarcaEnMapa>(filas.map((f) => [f.id, { banda: f.banda, alerta: f.alerta, nombre: nombreDeMarca(f) }])),
    [filas],
  )
  const fila = senalada ? filas.find((f) => f.id === senalada) : undefined

  return (
    <Tarjeta rotulo="Mapa" titulo={`Las ${filas.length} estaciones de OEFA`} className="@6xl:sticky @6xl:top-4">
      {mapa.data ? (
        <MapaPeru
          geo={mapa.data}
          lugares={lugares}
          marcas={marcas}
          enlaceDe={enlaceDe}
          senalada={senalada}
          alSenalar={setSenalada}
          titulo={`Mapa del Perú con las ${filas.length} estaciones y su último estado recibido`}
          alinear="derecha"
          className={ALTO_DEL_MAPA}
        >
          {/* La leyenda va sobre el mar del suroeste; la estación señalada, sobre el del noroeste. */}
          <div className="pointer-events-none absolute bottom-0 left-0 w-[10.5rem]" data-sobre-mapa>
            <Leyenda filas={filas} />
          </div>
          <div
            className="pointer-events-none absolute top-0 left-0 w-[11rem] space-y-1 text-sm"
            data-senalada={fila?.id}
            aria-hidden="true"
          >
            {fila ? (
              <>
                <p className="text-texto font-mono font-semibold">{fila.id}</p>
                <p className="text-suave text-xs">{lugarDe(fila.estacion)}</p>
                <Banda valor={fila.banda} />
                {fila.alerta && (
                  <p>
                    <Etiqueta tono="alerta">En alerta</Etiqueta>
                  </p>
                )}
              </>
            ) : (
              <p className="text-tenue text-xs">Cada marca es una estación. Con el cursor encima se ve cuál es; con un clic se abre.</p>
            )}
          </div>
        </MapaPeru>
      ) : mapa.isError ? (
        <EstadoDeError error={mapa.error} alReintentar={() => void mapa.refetch()} />
      ) : (
        <Estado tipo="cargando" titulo="Cargando el mapa…" className={ALTO_DEL_MAPA} />
      )}
    </Tarjeta>
  )
}

function NoMiden({ filas }: { filas: readonly FilaEstacion[] }) {
  if (filas.length === 0) return null
  return (
    <div className="border-borde mt-4 border-t pt-4">
      <Desplegable titulo={`${filas.length} estaciones no miden PM por hora y nunca van a tener estado`}>
        <p>
          Son de muestreo manual de 24 horas o no tienen PM2.5 y PM10 horarios: no entran al modelo. En el mapa van
          apagadas, con un rombo. Que no midan no quiere decir que ahí el aire esté limpio.
        </p>
        <ul aria-label="Estaciones que no miden PM por hora" className="grid gap-x-4 gap-y-1 sm:grid-cols-2">
          {filas.map((f) => (
            <li key={f.id}>
              <Link to={enlaceDe(f.id)} className="font-mono font-semibold">
                {f.id}
              </Link>{' '}
              <span className="text-xs">{lugarDe(f.estacion)}</span>
            </li>
          ))}
        </ul>
      </Desplegable>
    </div>
  )
}

function Contenido({
  estaciones,
  impacto,
  estados,
}: {
  estaciones: readonly Estacion[]
  impacto: Impacto
  estados: readonly EstadoEstacion[]
}) {
  const sinPM = useMemo(() => new Set(impacto.alcance.estaciones_sin_pm_horario), [impacto])
  const filas = useMemo(() => filasDe(estaciones, estados, sinPM), [estaciones, estados, sinPM])
  const resumen = useMemo(() => resumir(filas), [filas])
  // El lugar de cada marca sale solo del catálogo: no se vuelve a repartir con cada estado.
  const lugares = useMemo<LugarEnMapa[]>(
    () => estaciones.map((e) => ({ id: e.id, lon: e.lon, lat: e.lat, apagada: sinPM.has(e.id) })),
    [estaciones, sinPM],
  )
  const frase = frasePanel(resumen)
  const miden = filas.filter((f) => f.situacion !== 'no_mide')
  const noMiden = filas.filter((f) => f.situacion === 'no_mide')

  // Tres disposiciones, según el ancho en rem (que el modo proyector agranda):
  // en una columna; el mapa junto a las cifras y la lista debajo, a todo el
  // ancho; y, si hay sitio, el mapa a la izquierda con las cifras y la lista al lado.
  return (
    <div className="@container">
      <div className="grid items-start gap-4 @4xl:grid-cols-[minmax(0,27rem)_minmax(0,1fr)] @6xl:grid-rows-[auto_1fr]">
        <div className="space-y-4 @4xl:col-start-2 @4xl:row-start-1" data-zona="resumen">
          {resumen.ultimo === null ? (
            <SinEstados />
          ) : (
            <>
              <Cifras resumen={resumen} />
              <Frase tono={frase.tono}>{frase.texto}</Frase>
            </>
          )}
        </div>
        <div className="@4xl:col-start-1 @4xl:row-start-1 @6xl:row-span-2 @6xl:h-full" data-zona="mapa">
          <TarjetaDelMapa filas={filas} lugares={lugares} />
        </div>
        <div className="@4xl:col-span-2 @6xl:col-span-1 @6xl:col-start-2 @6xl:row-start-2" data-zona="lista">
          <Tarjeta titulo="Estaciones, de la más grave a la menos" aLaDerecha={`${miden.length} miden PM por hora`} className="p-4!">
            <Tabla
              titulo="Último estado recibido de cada estación, por gravedad"
              columnas={COLUMNAS}
              filas={miden}
              claveDe={(f) => f.id}
              vacio="Ninguna estación del catálogo mide PM por hora."
            />
            <NoMiden filas={noMiden} />
          </Tarjeta>
        </div>
      </div>
    </div>
  )
}

export default function Panel() {
  useRegistroDeSesion()
  const estaciones = useEstaciones()
  const impacto = useImpacto()
  const estados = useEstados()
  const consultas = [estaciones, impacto, estados]
  const fallida = consultas.find((c) => c.data === undefined && c.isError)

  return (
    <div className="space-y-4">
      <header className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
        <h1 className="text-texto text-2xl font-semibold">Panel</h1>
        <p className="text-suave text-sm">
          El último estado recibido de cada estación, con su fecha. Se actualiza solo cuando llega uno nuevo.
        </p>
      </header>
      {fallida ? (
        <EstadoDeError
          error={fallida.error}
          alReintentar={() => consultas.filter((c) => c.isError).forEach((c) => void c.refetch())}
        />
      ) : !estaciones.data || !impacto.data || !estados.data ? (
        <Estado tipo="cargando" titulo="Cargando las estaciones…" />
      ) : estaciones.data.length === 0 ? (
        <Estado tipo="vacio" titulo="El catálogo de estaciones está vacío">
          La API respondió sin estaciones: falta sembrar la colección de estaciones.
        </Estado>
      ) : (
        <Contenido estaciones={estaciones.data} impacto={impacto.data} estados={estados.data} />
      )}
    </div>
  )
}
