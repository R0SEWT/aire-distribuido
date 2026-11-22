// La columna de la derecha de Simulación: por dónde va la corrida y el gráfico
// que se va llenando con lo que responde la API. Sin corrida, muestra lo que se
// va a mandar.

import { useMemo, useState } from 'react'
import type { Esperado, ReglasDeSeries } from '../../api/datos'
import { presentarError } from '../../api/errores'
import {
  Aviso,
  Banda,
  BANDAS,
  Cifra,
  diaDeInstante,
  enumerar,
  Etiqueta,
  fechaHora,
  Frase,
  hora,
  numero,
  Pildoras,
  Tarjeta,
  type Tono,
} from '../../componentes/indice'
import { CurvaSimulacion, type Contaminante } from '../../graficos/CurvaSimulacion'
import { RejillaEscenario, type FilaDeRejilla } from '../../graficos/RejillaEscenario'
import { horasEntre } from './fechas'
import type { Corrida, Fase, Seguimiento } from './motor'
import type { Tramo } from './series'

const FASES: Record<Fase, { palabra: string; tono: Tono }> = {
  lista: { palabra: 'Sin empezar', tono: 'neutro' },
  en_curso: { palabra: 'En curso', tono: 'acento' },
  en_pausa: { palabra: 'En pausa', tono: 'aviso' },
  con_error: { palabra: 'Detenida por un error', tono: 'alerta' },
  detenida: { palabra: 'Detenida', tono: 'aviso' },
  terminada: { palabra: 'Terminó', tono: 'buena' },
}

interface Props {
  tramo: Tramo
  /** La corrida de este tramo, si la hay: en curso o ya terminada. */
  corrida: Corrida | null
  reglas: ReglasDeSeries
}

/** El contaminante que más se acerca a su ECA al final del tramo: el que conviene mirar primero. */
function contaminanteQueManda(esperado: Esperado | undefined, reglas: ReglasDeSeries): Contaminante {
  const a = (esperado?.media_24h.pm2_5 ?? 0) / reglas.eca_pm2_5
  const b = (esperado?.media_24h.pm10 ?? 0) / reglas.eca_pm10
  return a > b ? 'pm2_5' : 'pm10'
}

function fraseDelEscenario(tramo: Tramo, corrida: Corrida, reglas: ReglasDeSeries): { tono: Tono; texto: string } {
  const seguimientos = Object.values(corrida.porEstacion)
  const conMedia = seguimientos.filter((s) => s.ultima?.inca).length
  const enAlerta = seguimientos.filter((s) => s.ultima?.alerta).map((s) => s.estacion)
  const n = tramo.estaciones.length
  const alertas =
    enAlerta.length === 0
      ? 'ninguna está en alerta'
      : `${enAlerta.length === 1 ? 'está en alerta' : 'están en alerta'} ${enumerar(enAlerta)}`
  if (conMedia === 0) {
    return {
      tono: 'acento',
      texto: `Todavía ninguna estación tiene media de 24 horas: la API la da recién cuando la ventana junta ${reglas.minimo_lecturas} lecturas.`,
    }
  }
  return {
    tono: enAlerta.length > 0 ? 'alerta' : 'buena',
    texto: `${conMedia} de ${n} estaciones tienen media de 24 horas y ${alertas}: su media supera el ECA.`,
  }
}

function fraseDelDia(s: Seguimiento | undefined, reglas: ReglasDeSeries): { tono: Tono; texto: string } {
  const r = s?.ultima
  if (!s || !r) return { tono: 'acento', texto: 'Todavía no se mandó ninguna lectura.' }
  if (r.media_24h.pm2_5 === null || r.media_24h.pm10 === null) {
    const faltan = reglas.minimo_lecturas - r.media_24h.lecturas
    return {
      tono: 'acento',
      texto: `La ventana de 24 horas tiene ${r.media_24h.lecturas} ${r.media_24h.lecturas === 1 ? 'lectura' : 'lecturas'}: ${
        faltan === 1 ? 'falta 1' : `faltan ${faltan}`
      } para que la API calcule la media. Sin media no hay alerta ni banda INCA.`,
    }
  }
  const medias = `La media de las últimas 24 horas es ${numero(r.media_24h.pm2_5, 1)} µg/m³ de PM2.5 y ${numero(r.media_24h.pm10, 1)} µg/m³ de PM10`
  if (!r.alerta) {
    return { tono: 'buena', texto: `${medias}: no supera el ECA (${reglas.eca_pm2_5} y ${reglas.eca_pm10}).` }
  }
  const cual = [
    r.media_24h.pm2_5 > reglas.eca_pm2_5 ? `el de PM2.5 (${reglas.eca_pm2_5})` : null,
    r.media_24h.pm10 > reglas.eca_pm10 ? `el de PM10 (${reglas.eca_pm10})` : null,
  ].filter((x): x is string => x !== null)
  const desde = s.primeraAlerta ? ` Hay alerta desde el ${fechaHora(s.primeraAlerta)}.` : ''
  return { tono: 'alerta', texto: `${medias}: supera el ECA, ${enumerar(cual)}.${desde}` }
}

/** La ventana de 24 horas, lectura por lectura: se ve cómo se llena y dónde aparece la media. */
function Ventana({ lecturas, reglas }: { lecturas: number; reglas: ReglasDeSeries }) {
  const casillas = Array.from({ length: reglas.ventana_horas }, (_, i) => i)
  return (
    <div>
      <p className="rotulo mb-1.5">
        Ventana de 24 h · {lecturas} de {reglas.ventana_horas} lecturas
      </p>
      <div
        role="img"
        aria-label={`La ventana de 24 horas tiene ${lecturas} de ${reglas.ventana_horas} lecturas; la media aparece con ${reglas.minimo_lecturas}.`}
        className="flex items-end gap-1"
      >
        {casillas.map((i) => (
          <span
            key={i}
            className={`h-4 flex-1 rounded-sm border ${i < lecturas ? 'border-acento bg-acento' : 'border-borde bg-transparent'} ${
              i === reglas.minimo_lecturas - 1 ? 'h-6' : ''
            }`}
          />
        ))}
      </div>
      <p className="text-tenue mt-1 text-xs">Casilla alta: la lectura {reglas.minimo_lecturas}, donde aparece la media.</p>
    </div>
  )
}

export function Marcha({ tramo, corrida, reglas }: Props) {
  const [elegidaAMano, setElegida] = useState<string | null>(null)
  const [contaminanteAMano, setContaminante] = useState<Contaminante | null>(null)

  const horas = useMemo(() => horasEntre(tramo.desde, tramo.hasta), [tramo])
  const porEstacion = corrida?.porEstacion
  const programadas = useMemo(() => {
    const m = new Map<string, Set<number>>()
    for (const l of tramo.lecturas) {
      const s = m.get(l.estacion) ?? new Set<number>()
      s.add(Date.parse(l.fecha_hora))
      m.set(l.estacion, s)
    }
    return m
  }, [tramo])
  const filas: FilaDeRejilla[] = useMemo(
    () =>
      tramo.estaciones.map((estacion) => ({
        estacion,
        programadas: programadas.get(estacion) ?? new Set<number>(),
        puntos: porEstacion?.[estacion]?.puntos ?? [],
      })),
    [tramo, programadas, porEstacion],
  )

  const fase = corrida?.fase ?? 'lista'
  const enviadas = corrida?.enviadas ?? 0
  const total = tramo.lecturas.length
  const esEscenario = tramo.tipo === 'escenario'

  // La estación de la curva: la que se eligió o, si no, la primera en alerta.
  const elegida =
    elegidaAMano !== null && tramo.estaciones.includes(elegidaAMano)
      ? elegidaAMano
      : (tramo.estaciones.find((e) => porEstacion?.[e]?.ultima?.alerta) ??
        tramo.estaciones.find((e) => tramo.esperado[e]?.alerta) ??
        tramo.estaciones[0] ??
        '')
  const seguimiento = porEstacion?.[elegida]
  const contaminante = contaminanteAMano ?? contaminanteQueManda(tramo.esperado[elegida], reglas)
  const lecturasDeElegida = useMemo(() => tramo.lecturas.filter((l) => l.estacion === elegida), [tramo, elegida])

  const frase = !corrida
    ? null
    : esEscenario
      ? fraseDelEscenario(tramo, corrida, reglas)
      : fraseDelDia(corrida.porEstacion[tramo.estaciones[0] ?? ''], reglas)
  const error = corrida?.fase === 'con_error' ? presentarError(corrida.error) : null
  const seguimientos = Object.values(porEstacion ?? {})
  const enAlerta = seguimientos.filter((s) => s.ultima?.alerta).length
  const unica = esEscenario ? undefined : porEstacion?.[tramo.estaciones[0] ?? '']

  const pildorasDeContaminante = (
    <Pildoras
      etiqueta="Contaminante"
      valor={contaminante}
      alCambiar={setContaminante}
      opciones={[
        { valor: 'pm2_5', etiqueta: 'PM2.5' },
        { valor: 'pm10', etiqueta: 'PM10' },
      ]}
    />
  )
  const curva = (
    <CurvaSimulacion
      alto={esEscenario ? 15.5 : 13.5}
      estacion={elegida}
      horas={horas}
      programadas={lecturasDeElegida}
      puntos={seguimiento?.puntos ?? []}
      contaminante={contaminante}
      eca={contaminante === 'pm2_5' ? reglas.eca_pm2_5 : reglas.eca_pm10}
      cortes={contaminante === 'pm2_5' ? reglas.inca.cortes_pm2_5 : reglas.inca.cortes_pm10}
      primeraMedia={seguimiento?.primeraMedia ?? null}
      primeraAlerta={seguimiento?.primeraAlerta ?? null}
    />
  )

  return (
    <div className="space-y-5">
      <Tarjeta
        acento={FASES[fase].tono === 'neutro' ? undefined : FASES[fase].tono}
        rotulo={`Progreso · ${tramo.tipo === 'escenario' ? 'el escenario' : tramo.estaciones[0]}`}
        aLaDerecha={<Etiqueta tono={FASES[fase].tono}>{FASES[fase].palabra}</Etiqueta>}
      >
        <div className="space-y-3.5">
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-[1fr_1.35fr_1fr_1fr]">
            <Cifra tamano="mediana" rotulo="Lecturas enviadas" valor={enviadas} unidad={`de ${numero(total)}`} />
            <div>
              <p className="rotulo mb-1">Hora simulada</p>
              {corrida?.horaActual ? (
                <p className="flex items-baseline gap-x-2">
                  <span className="text-texto font-mono text-3xl leading-none font-semibold tabular-nums">{hora(corrida.horaActual)}</span>
                  <span className="text-suave text-sm font-medium whitespace-nowrap">{diaDeInstante(corrida.horaActual)}</span>
                </p>
              ) : (
                <p className="flex items-baseline gap-2">
                  <span aria-hidden="true" className="text-texto font-mono text-3xl leading-none font-semibold">
                    —
                  </span>
                  <span className="text-suave text-base font-medium">sin empezar</span>
                </p>
              )}
            </div>
            {esEscenario ? (
              <>
                <Cifra
                  tamano="mediana"
                  rotulo="Con media de 24 h"
                  valor={seguimientos.filter((s) => s.ultima?.inca).length}
                  unidad={`de ${tramo.estaciones.length}`}
                />
                <Cifra tamano="mediana" rotulo="En alerta" valor={enAlerta} unidad={enAlerta === 1 ? 'estación' : 'estaciones'} />
              </>
            ) : (
              <>
                <Cifra tamano="mediana" rotulo="PM2.5 · media 24 h" valor={unica?.ultima?.media_24h.pm2_5} decimales={1} unidad="µg/m³" sinDato="sin media" />
                <Cifra tamano="mediana" rotulo="PM10 · media 24 h" valor={unica?.ultima?.media_24h.pm10} decimales={1} unidad="µg/m³" sinDato="sin media" />
              </>
            )}
          </div>
          <progress
            className="[&::-webkit-progress-bar]:bg-relieve [&::-webkit-progress-value]:bg-acento [&::-moz-progress-bar]:bg-acento bg-relieve block h-2 w-full appearance-none overflow-hidden rounded-full border-0"
            aria-label="Lecturas enviadas"
            value={enviadas}
            max={Math.max(1, total)}
          />
          {error && corrida?.lecturaConError && (
            <Aviso tono="alerta" titulo={`${error.titulo}${error.detalle ? `: ${error.detalle}` : ''}`}>
              <p>
                No pasó la lectura de <span className="font-mono">{corrida.lecturaConError.estacion}</span> del{' '}
                {fechaHora(corrida.lecturaConError.fecha_hora)}. La corrida espera ahí: no la salta, porque el orden
                importa. Las {numero(corrida.enviadas)} anteriores ya están en el sistema.
              </p>
            </Aviso>
          )}

          {esEscenario ? (
            <div>
              <RejillaEscenario
                horas={horas}
                filas={filas}
                minimo={reglas.minimo_lecturas}
                cursor={corrida?.horaActual && fase !== 'terminada' ? Date.parse(corrida.horaActual) : null}
                elegida={elegida}
                alElegir={setElegida}
              />
              <ul className="text-suave mt-2 flex flex-wrap items-center gap-x-4 gap-y-1.5 text-xs">
                <li className="flex items-center gap-1.5">
                  <span aria-hidden="true" className="border-borde-control inline-block size-3 rounded-sm border" /> Por enviar
                </li>
                <li className="flex items-center gap-1.5">
                  <span aria-hidden="true" className="bg-sin-datos inline-block size-1.5 rounded-full" /> Enviada, sin
                  media (menos de {reglas.minimo_lecturas} en la ventana)
                </li>
                {(['buena', 'moderada', 'mala', 'umbral_de_cuidado'] as const).map((b) => (
                  <li key={b} className="flex items-center gap-1.5">
                    <span aria-hidden="true" className={`inline-block size-3 rounded-sm ${BANDAS[b].fondo}`} /> {BANDAS[b].palabra}
                  </li>
                ))}
                <li className="flex items-center gap-1.5">
                  <span aria-hidden="true" className="border-texto bg-inca-mala inline-block size-3 rounded-sm border-2" /> Borde
                  blanco: en alerta
                </li>
                <li className="flex items-center gap-1.5">
                  <span aria-hidden="true" className="bg-borde-control inline-block h-0.5 w-2" /> Sin lectura en la fuente
                </li>
              </ul>
            </div>
          ) : (
            <>
              <div className="grid gap-x-6 gap-y-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
                <Ventana lecturas={unica?.ultima?.media_24h.lecturas ?? 0} reglas={reglas} />
                <div className="flex flex-wrap items-center gap-2">
                  <Banda valor={unica?.ultima ? unica.ultima.inca : undefined} conPrefijo />
                  {unica?.ultima?.alerta ? <Etiqueta tono="alerta">En alerta</Etiqueta> : <Etiqueta>Sin alerta</Etiqueta>}
                  {pildorasDeContaminante}
                </div>
              </div>
              {curva}
            </>
          )}

          {frase && <Frase tono={frase.tono}>{frase.texto}</Frase>}
          {corrida?.fase === 'detenida' && (
            <p className="text-suave text-sm">
              Se detuvo con {numero(corrida.enviadas)} de {numero(total)} lecturas enviadas. Lo enviado ya está en el
              sistema: el último estado de esas estaciones quedó en la hora simulada.
            </p>
          )}
        </div>
      </Tarjeta>

      {esEscenario && (
        <Tarjeta
          rotulo={`${elegida} · lecturas horarias y media de 24 horas`}
          titulo="La curva de una estación"
          aLaDerecha={
            <div className="flex flex-wrap items-center justify-end gap-2">
              <label className="flex items-center gap-2">
                <span className="rotulo">Estación</span>
                <select className="campo w-auto py-1" value={elegida} onChange={(e) => setElegida(e.target.value)}>
                  {tramo.estaciones.map((e) => (
                    <option key={e} value={e}>
                      {e}
                    </option>
                  ))}
                </select>
              </label>
              {pildorasDeContaminante}
            </div>
          }
        >
          {curva}
        </Tarjeta>
      )}
    </div>
  )
}
