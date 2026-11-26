// Impacto, primera mitad: el problema, cuánto tiempo, dónde y junto a qué, y
// quién vive ahí (spec §5.7). Todo sale de impacto.json.

import { useState } from 'react'
import {
  Banda,
  BANDAS,
  Cifra,
  enumerar,
  Etiqueta,
  fechaDia,
  Frase,
  nombrePropio,
  numero,
  Pildoras,
  porciento,
  Tabla,
  type Columna,
} from '../../componentes/indice'
import { BarraBandas } from '../../graficos/BarraBandas'
import { DeDondeSale, Seccion } from './apoyo'
import { DIAS_DE_CAMPANA_CORTA } from './textos'
import type { DistritoDeImpacto, EstacionDeImpacto, Impacto, NombreDeBanda, TipoDeActividad } from '../../api/datos'

const ORDEN: readonly NombreDeBanda[] = ['buena', 'moderada', 'mala', 'umbral_de_cuidado']

export function ElProblema({ impacto }: { impacto: Impacto }) {
  const r = impacto.metadatos.reglas
  return (
    <Seccion id="el-problema" pregunta="El problema" titulo="Qué es el ECA y qué es el INCA">
      <p className="text-texto leading-relaxed">
        El <strong>ECA</strong> (Estándar de Calidad Ambiental, DS 003-2017-MINAM) aporta los valores de referencia que
        usa este análisis para la media de 24 horas: <strong>{numero(r.eca_pm2_5)} µg/m³ de PM2.5</strong> y{' '}
        <strong>{numero(r.eca_pm10)} µg/m³ de PM10</strong>, dos tamaños de partículas suspendidas en el aire.
      </p>
      <p className="text-texto leading-relaxed">
        El <strong>INCA</strong> (Índice de Calidad del Aire, RM 181-2016-MINAM) pone esa misma media en una de cuatro
        bandas, de buena a umbral de cuidado, y se informa aparte del ECA.
      </p>
      <div className="flex flex-wrap gap-2">
        {ORDEN.map((b) => (
          <Banda key={b} valor={b} />
        ))}
      </div>
      <p className="text-suave text-sm leading-relaxed">
        Las estaciones de OEFA miden junto a actividades fiscalizadas: una mina, una refinería, una zona de ladrilleras.
        Este sistema compara la media de 24 horas con el ECA cuando reúne al menos {r.minimo_lecturas} lecturas en la
        ventana. Si no alcanza ese mínimo, no hay media calculable: no equivale a estar bajo el ECA.
      </p>
    </Seccion>
  )
}

export function CuantoTiempo({ impacto }: { impacto: Impacto }) {
  const t = impacto.totales
  const r = impacto.metadatos.reglas
  return (
    <Seccion id="cuanto-tiempo" pregunta="¿Cuánto tiempo?" titulo="Las horas en que la media de 24 horas superó el ECA">
      <div className="flex flex-wrap items-end gap-x-10 gap-y-4">
        <Cifra valor={t.horas_sobre_eca} unidad="horas-estación sobre el ECA" />
        <Cifra tamano="mediana" rotulo="De las horas con media calculable" valor={t.horas_calculables} unidad={`horas-estación · ${porciento(t.horas_sobre_eca, t.horas_calculables)}`} />
      </div>
      <Frase tono="mala">
        De cada 100 horas-estación con media de 24 horas calculable, {numero((100 * t.horas_sobre_eca) / t.horas_calculables, 1)}{' '}
        superaron el ECA. Una misma hora medida en dos estaciones cuenta dos veces. Se suman las observaciones de{' '}
        {numero(impacto.alcance.estaciones_con_pm_horario)} estaciones en sus periodos medidos: no es un promedio del país
        ni la duración de un único episodio.
      </Frase>
      <div>
        <p className="rotulo mb-2">Cómo se reparten esas {numero(t.horas_calculables)} horas entre las bandas INCA</p>
        <BarraBandas horas={t.inca} de="todas las estaciones" alto={1.5} />
        <dl className="mt-3 grid grid-cols-2 gap-x-6 gap-y-3 sm:grid-cols-4">
          {ORDEN.map((b) => (
            <div key={b}>
              <dt className="text-texto flex items-center gap-2 text-sm font-semibold">
                <span aria-hidden="true" className={`size-2.5 shrink-0 rounded-full ${BANDAS[b].fondo}`} />
                {BANDAS[b].palabra}
              </dt>
              <dd className="mt-0.5">
                <span className="text-texto block font-mono text-lg leading-tight font-semibold tabular-nums">{numero(t.inca[b])}</span>
                <span className="text-suave block text-sm">horas · {porciento(t.inca[b], t.horas_calculables)}</span>
              </dd>
            </div>
          ))}
        </dl>
      </div>
      <DeDondeSale impacto={impacto} fuentes={['gold', 'silver', 'alerta_24h']}>
        <p>
          {r.lectura} De {numero(t.lecturas)} lecturas, {numero(t.horas_calculables)} tienen media de 24 horas: la
          ventana {r.ventana} necesita {r.minimo_lecturas} lecturas o más. El porcentaje divide las horas-estación sobre
          el ECA entre las que tienen media calculable; las horas sin media no entran en ese denominador.
        </p>
        <p>
          Una hora está sobre el ECA si se cumple la regla de la alerta: {r.alerta}. La banda INCA es {r.inca.regla}, con
          cortes de PM10 en {r.inca.cortes_pm10.join(', ')} y de PM2.5 en {r.inca.cortes_pm2_5.join(', ')} µg/m³. Es la
          misma regla de <code className="font-mono">{r.implementacion}</code>, la que usa la API.
        </p>
      </DeDondeSale>
    </Seccion>
  )
}

function midio(desde: string, hasta: string) {
  return (
    <span className="whitespace-nowrap">
      {fechaDia(desde)} – {fechaDia(hasta)}
    </span>
  )
}

function SobreElEca({ horas, calculables }: { horas: number; calculables: number }) {
  return (
    <span className="inline-flex flex-col items-end leading-tight whitespace-nowrap">
      <span className="text-texto font-semibold">{numero(horas)}</span>
      <span className="text-tenue font-sans text-xs">
        de {numero(calculables)} · {porciento(horas, calculables)}
      </span>
    </span>
  )
}

function Reparto({ de, fila }: { de: string; fila: { inca: Record<NombreDeBanda, number>; banda_mas_frecuente: NombreDeBanda } }) {
  return (
    <div className="flex w-40 flex-col gap-1.5">
      <BarraBandas horas={fila.inca} de={de} />
      <span className="text-suave text-xs">
        más frecuente: <span className="text-texto font-semibold">{BANDAS[fila.banda_mas_frecuente].palabra.toLowerCase()}</span>
      </span>
    </div>
  )
}

const COLUMNAS_DE_ESTACION: Columna<EstacionDeImpacto>[] = [
  {
    clave: 'estacion',
    titulo: 'Estación, tipo de actividad y lo que vigila',
    celda: (e) => (
      <div className="w-72">
        <p className="flex flex-wrap items-baseline gap-x-2">
          <span className="text-texto font-mono font-semibold">{e.estacion}</span>
          <span className={e.tipo_actividad === 'no lo dice el nombre' ? 'text-tenue italic' : 'text-texto font-semibold'}>
            {e.tipo_actividad === 'no lo dice el nombre' ? 'actividad: no lo dice el nombre' : e.tipo_actividad}
          </span>
        </p>
        <p className="text-suave mt-0.5 text-xs leading-snug">{e.objeto_fiscalizado.split(' | ')[0]}</p>
      </div>
    ),
  },
  {
    clave: 'midio',
    titulo: 'Cuándo midió',
    celda: (e) => (
      <div className="leading-tight">
        <p>{midio(e.primera_fecha, e.ultima_fecha)}</p>
        <p className="text-tenue mt-0.5 flex flex-wrap items-center gap-1.5 text-xs">
          {numero(e.dias_con_datos)} días con datos
          {e.dias_con_datos < DIAS_DE_CAMPANA_CORTA && <Etiqueta tono="aviso">campaña corta</Etiqueta>}
        </p>
      </div>
    ),
  },
  {
    clave: 'sobre_eca',
    titulo: 'Horas sobre el ECA',
    numerica: true,
    celda: (e) => <SobreElEca horas={e.horas_sobre_eca} calculables={e.horas_calculables} />,
  },
  { clave: 'inca', titulo: 'Bandas INCA de sus horas', celda: (e) => <Reparto de={e.estacion} fila={e} /> },
  {
    clave: 'distrito',
    titulo: 'Distrito (contexto)',
    celda: (e) =>
      e.distrito && e.poblacion_distrito ? (
        <div className="leading-tight">
          <p className="text-suave">
            {nombrePropio(e.distrito)}, {nombrePropio(e.departamento)}
          </p>
          <p className="text-tenue mt-0.5 text-xs whitespace-nowrap">
            {numero(e.poblacion_distrito.poblacion)} hab. (censo de {e.poblacion_distrito.anio})
          </p>
        </div>
      ) : (
        <span className="text-tenue italic">sin distrito en la fuente</span>
      ),
  },
]

const COLUMNAS_DE_TIPO: Columna<TipoDeActividad>[] = [
  { clave: 'tipo', titulo: 'Tipo de actividad', celda: (t) => <span className={t.tipo_actividad === 'no lo dice el nombre' ? 'text-tenue italic' : 'text-texto font-semibold'}>{t.tipo_actividad}</span> },
  {
    clave: 'estaciones',
    titulo: 'Estaciones',
    celda: (t) => <span className="text-suave font-mono text-xs">{t.estaciones.join(', ')}</span>,
  },
  {
    clave: 'midio',
    titulo: 'Cuándo midieron',
    celda: (t) => (
      <div className="leading-tight">
        <p>{midio(t.primera_fecha, t.ultima_fecha)}</p>
        <p className="text-tenue mt-0.5 text-xs">{numero(t.dias_estacion_con_datos)} días-estación con datos</p>
      </div>
    ),
  },
  {
    clave: 'sobre_eca',
    titulo: 'Horas sobre el ECA',
    numerica: true,
    celda: (t) => <SobreElEca horas={t.horas_sobre_eca} calculables={t.horas_calculables} />,
  },
  { clave: 'inca', titulo: 'Bandas INCA de sus horas', celda: (t) => <Reparto de={t.tipo_actividad} fila={t} /> },
]

type Vista = 'estacion' | 'tipo'

const proporcionSobreEca = (x: { horas_sobre_eca: number; horas_calculables: number }) =>
  x.horas_calculables > 0 ? x.horas_sobre_eca / x.horas_calculables : 0

export function DondeYJuntoAQue({ impacto }: { impacto: Impacto }) {
  const [vista, setVista] = useState<Vista>('estacion')
  // De la mayor parte de sus horas sobre el ECA a la menor: así una campaña corta no queda detrás de una larga.
  const estaciones = [...impacto.estaciones].sort((a, b) => proporcionSobreEca(b) - proporcionSobreEca(a) || a.estacion.localeCompare(b.estacion))
  const tipos = [...impacto.tipos_actividad].sort((a, b) => proporcionSobreEca(b) - proporcionSobreEca(a) || a.tipo_actividad.localeCompare(b.tipo_actividad))
  const cortas = impacto.estaciones.filter((e) => e.dias_con_datos < DIAS_DE_CAMPANA_CORTA).length
  const sinHoras = impacto.estaciones.filter((e) => e.horas_sobre_eca === 0).length
  return (
    <Seccion id="donde" pregunta="¿Dónde y junto a qué?" titulo="Cada estación vigila una actividad: esa es la unidad honesta">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Pildoras
          etiqueta="Cómo se agrupa"
          valor={vista}
          alCambiar={setVista}
          opciones={[
            { valor: 'estacion', etiqueta: 'Por estación' },
            { valor: 'tipo', etiqueta: 'Por tipo de actividad' },
          ]}
        />
        <p className="text-tenue text-sm">Ordenado por la parte de sus horas que superó el ECA.</p>
      </div>
      {vista === 'estacion' ? (
        <Tabla titulo="Lo medido por cada estación con PM por hora" columnas={COLUMNAS_DE_ESTACION} filas={estaciones} claveDe={(e) => e.estacion} />
      ) : (
        <Tabla titulo="Lo medido por tipo de actividad" columnas={COLUMNAS_DE_TIPO} filas={tipos} claveDe={(t) => t.tipo_actividad} />
      )}
      <Frase tono="aviso">
        Las horas sobre el ECA de una estación describen las semanas en que midió, no un año: {numero(cortas)} de las{' '}
        {numero(impacto.estaciones.length)} son campañas cortas (menos de {DIAS_DE_CAMPANA_CORTA} días con datos). En{' '}
        {numero(sinHoras)} no se registraron superaciones entre las medias de 24 horas calculables de ese periodo.
      </Frase>
      <DeDondeSale impacto={impacto} fuentes={['gold', 'ficha', 'poblacion']}>
        <p>
          Las horas son las de cada estación en el gold. El objeto fiscalizado y el tipo de actividad salen de la ficha de
          estaciones; «no lo dice el nombre» es eso: el nombre del objeto fiscalizado no dice qué actividad es, y no se
          inventa.
        </p>
        {vista === 'tipo' && <p>{impacto.metadatos.notas.find((n) => n.includes('dias_estacion_con_datos'))}</p>}
      </DeDondeSale>
    </Seccion>
  )
}

const COLUMNAS_DE_DISTRITO: Columna<DistritoDeImpacto>[] = [
  {
    clave: 'distrito',
    titulo: 'Distrito',
    celda: (d) =>
      d.distrito ? (
        <div className="leading-tight">
          <p className="text-texto font-semibold">{nombrePropio(d.distrito)}</p>
          <p className="text-tenue mt-0.5 text-xs">
            {nombrePropio(d.provincia)}, {nombrePropio(d.departamento)}
          </p>
        </div>
      ) : (
        <span className="text-tenue italic">Sin distrito en la fuente</span>
      ),
  },
  {
    clave: 'poblacion',
    titulo: 'Población censada',
    numerica: true,
    celda: (d) =>
      d.poblacion !== null ? (
        <span className="inline-flex flex-col items-end leading-tight whitespace-nowrap">
          <span className="text-texto font-semibold">{numero(d.poblacion)}</span>
          <span className="text-tenue font-sans text-xs">
            censo de {d.anio}
            {d.fuente && (
              <>
                {' · '}
                <a href={d.fuente} target="_blank" rel="noopener noreferrer">
                  INEI
                </a>
              </>
            )}
          </span>
        </span>
      ) : (
        '—'
      ),
  },
  { clave: 'estaciones', titulo: 'Estaciones con PM por hora', celda: (d) => <span className="text-suave font-mono text-xs">{d.estaciones.join(', ')}</span> },
  { clave: 'junto_a', titulo: 'Junto a qué miden', celda: (d) => <span className="text-suave">{enumerar(d.tipos_actividad)}</span> },
]

export function QuienViveAhi({ impacto }: { impacto: Impacto }) {
  const distritos = [...impacto.distritos].sort((a, b) => (a.distrito ?? '￿').localeCompare(b.distrito ?? '￿'))
  return (
    <Seccion id="quien-vive-ahi" pregunta="¿Quién vive ahí?" titulo="Dónde podría servir esta información">
      <p className="text-texto leading-relaxed">
        El posible beneficio es facilitar la consulta de mediciones y avisos de las estaciones en los{' '}
        {numero(impacto.alcance.distritos_con_pm_horario)} distritos con PM por hora. La población censada ubica el contexto
        de esas estaciones; no mide cuántas personas usan el sistema, recibieron avisos o estuvieron expuestas. Estos
        datos tampoco permiten afirmar que antes no tuvieran otra fuente de avisos.
      </p>
      <Tabla titulo="La población de cada distrito con una estación de PM por hora, fila por fila" columnas={COLUMNAS_DE_DISTRITO} filas={distritos} claveDe={(d) => d.ubigeo ?? 'sin-distrito'} />
      <Frase tono="acento">
        La población va fila por fila, como contexto de su distrito, y no se suma: las estaciones miden junto a una
        actividad fiscalizada, y vivir en el distrito no es haber respirado ese aire.
      </Frase>
      <DeDondeSale impacto={impacto} fuentes={['poblacion', 'ficha']}>
        <p>
          Es la población censada del distrito de cada estación; el enlace de cada fila abre la página del INEI de donde
          sale y el año que corresponde. No es una estimación de la población actual ni del alcance de la estación. El
          cruce es por ubigeo, no por nombre: Lurigancho no es San Juan de Lurigancho.
        </p>
        <p>{impacto.metadatos.notas.find((n) => n.includes('No se suma'))}</p>
      </DeDondeSale>
    </Seccion>
  )
}
