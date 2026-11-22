// La columna de la izquierda de Simulación: qué se manda, a qué ritmo, si el
// sistema está limpio para ese tramo y los botones de la corrida.

import type { ReactNode } from 'react'
import type { DiaDeSeries, EscenarioDeSeries } from '../../api/datos'
import type { EstadoEstacion } from '../../api/enVivo'
import { Estado, EstadoDeError, Etiqueta, fechaHora, numero, Pildoras, Tarjeta, type Opcion } from '../../componentes/indice'
import { diasEnLlano, enumerar, fechaDia } from './fechas'
import { diaDisponible, type Choque, type Revision, type TipoDeChoque } from './limpieza'
import type { Fase } from './motor'
import { horasPorSegundoDe, RITMOS, type Ritmo } from './ritmos'
import { nombreDelCaso, type Tramo } from './series'

/** Lo que el escenario manda, con las cifras del archivo. */
export function QueMandaElEscenario({ escenario }: { escenario: EscenarioDeSeries }) {
  return (
    <div className="space-y-3">
      <dl className="grid grid-cols-3 gap-3">
        <Dato valor={numero(escenario.horas)} que="horas seguidas" />
        <Dato valor={numero(escenario.estaciones.length)} que="estaciones" />
        <Dato valor={numero(escenario.lecturas.length)} que="lecturas" />
      </dl>
      <p className="text-suave text-sm leading-relaxed">
        Las {numero(escenario.horas)} horas reales del <strong className="text-texto">{diasEnLlano(escenario.dias)}</strong>
        : el tramo de esos días en que más estaciones midieron a la vez. Van en orden, por hora y dentro de la hora por
        estación.
      </p>
      {escenario.estaciones_sin_lecturas.length > 0 && (
        <p className="text-tenue text-xs leading-relaxed">
          Sin lecturas en ese tramo: {enumerar(escenario.estaciones_sin_lecturas)}.
        </p>
      )}
    </div>
  )
}

function Dato({ valor, que }: { valor: string; que: string }) {
  return (
    <div>
      <dd className="text-texto font-mono text-2xl leading-none font-semibold tabular-nums">{valor}</dd>
      <dt className="text-suave mt-1 text-xs">{que}</dt>
    </div>
  )
}

interface PropsDia {
  dias: readonly DiaDeSeries[]
  elegido: DiaDeSeries | undefined
  alElegir: (id: string) => void
  bloqueado: boolean
  descripcionDe: (estacion: string) => string | null
}

/** El día que se simula: una de las series de series.json, con su caso. */
export function QueMandaUnDia({ dias, elegido, alElegir, bloqueado, descripcionDe }: PropsDia) {
  return (
    <div className="space-y-3">
      <div>
        <label htmlFor="dia-de-simulacion" className="rotulo mb-1 block">
          Estación y día
        </label>
        <select
          id="dia-de-simulacion"
          className="campo"
          value={elegido?.id ?? ''}
          disabled={bloqueado}
          onChange={(e) => alElegir(e.target.value)}
        >
          {dias.map((d) => (
            <option key={d.id} value={d.id}>
              {d.estacion} · {fechaDia(d.dia)} · {nombreDelCaso(d.caso)}
            </option>
          ))}
        </select>
      </div>
      {elegido && (
        <>
          <p className="text-texto text-sm leading-relaxed">{elegido.descripcion}</p>
          <p className="text-suave text-sm leading-relaxed">
            {descripcionDe(elegido.estacion) && <>{descripcionDe(elegido.estacion)}. </>}
            Se mandan {numero(elegido.lecturas.length)} lecturas: el día anterior, que llena la ventana, y el{' '}
            {fechaDia(elegido.dia)}
            {elegido.huecos > 0 && ` (${elegido.huecos === 1 ? 'falta 1 hora' : `faltan ${elegido.huecos} horas`} en la fuente)`}.
          </p>
        </>
      )}
    </div>
  )
}

const QUE_PASA: Record<TipoDeChoque, (n: number) => string> = {
  posterior: (n) =>
    `${n === 1 ? 'tiene' : 'tienen'} un estado posterior a todo el tramo: ninguna de sus lecturas lo va a cambiar`,
  misma: (n) =>
    `ya ${n === 1 ? 'tiene' : 'tienen'} el estado de la última hora del tramo (ya se simuló): solo su última lectura lo reemplaza`,
  dentro: (n) => `${n === 1 ? 'tiene' : 'tienen'} un estado de una hora del tramo: hasta esa hora no cambia`,
}

function GrupoDeChoques({ tipo, choques }: { tipo: TipoDeChoque; choques: readonly Choque[] }) {
  if (choques.length === 0) return null
  const fechas = new Set(choques.map((c) => c.guardada))
  return (
    <li>
      <strong className="text-texto">
        {choques.length} {choques.length === 1 ? 'estación' : 'estaciones'}
      </strong>{' '}
      {QUE_PASA[tipo](choques.length)}.{' '}
      <span className="text-tenue font-mono text-xs">
        {choques.map((c) => c.estacion).join(', ')}
        {fechas.size === 1 && ` · estado del ${fechaHora(choques[0]!.guardada)}`}
      </span>
    </li>
  )
}

interface PropsSistema {
  tramo: Tramo
  consulta: { cargando: boolean; error: unknown; alReintentar: () => void }
  revision: Revision | null
  /** Para un día: los otros días que todavía moverían el estado de su estación. */
  otrosDias?: { dias: readonly DiaDeSeries[]; estados: readonly EstadoEstacion[]; alElegir: (id: string) => void }
}

/** El estado de las estaciones del tramo antes de simular (spec §5.4): si no está limpio, se dice. */
export function EstadoDelSistema({ tramo, consulta, revision, otrosDias }: PropsSistema) {
  if (consulta.error) {
    return (
      <div className="space-y-2">
        <EstadoDeError error={consulta.error} alReintentar={consulta.alReintentar} />
        <p className="text-suave text-sm">
          No se pudo consultar el estado de las estaciones: no se sabe si el sistema está limpio para este tramo.
        </p>
      </div>
    )
  }
  if (consulta.cargando || !revision) {
    return <Estado tipo="cargando" titulo="Consultando el estado de las estaciones…" className="py-5" />
  }
  const n = tramo.estaciones.length
  if (revision.limpio) {
    return (
      <div className="space-y-2" data-sistema="limpio">
        <Etiqueta tono="buena">Sistema limpio para este tramo</Etiqueta>
        <p className="text-suave text-sm leading-relaxed">
          {n === 1
            ? `${tramo.estaciones[0]} no tiene un estado de esas horas ni posterior`
            : `Ninguna de las ${n} estaciones tiene un estado de esas horas ni posterior`}
          : cada lectura va a mover el panel.
        </p>
      </div>
    )
  }
  const de = (tipo: TipoDeChoque) => revision.choques.filter((c) => c.tipo === tipo)
  if (soloRepite(revision)) {
    return (
      <div className="space-y-2" data-sistema="ya-cargado">
        <Etiqueta tono="acento">Este tramo ya está cargado</Etiqueta>
        <p className="text-suave text-sm leading-relaxed">
          {n === 1 ? `${tramo.estaciones[0]} ya tiene` : `Las ${revision.choques.length} estaciones ya tienen`} el estado de la
          última hora del tramo ({fechaHora(revision.choques[0]!.guardada)}). El estado no retrocede: si se vuelve a mandar,
          el panel no se mueve hasta la última lectura.
        </p>
      </div>
    )
  }
  const libres = otrosDias?.dias.filter((d) => diaDisponible(d, otrosDias.estados)) ?? []
  return (
    <div className="space-y-2" role="alert" data-sistema="no-limpio">
      <Etiqueta tono="moderada">El sistema no está limpio para este tramo</Etiqueta>
      <p className="text-suave text-sm leading-relaxed">
        <strong className="text-texto">El último estado de una estación no retrocede:</strong> una lectura más antigua que
        la guardada se predice y se responde, pero no cambia el estado ni mueve el panel.
      </p>
      <ul className="text-suave list-disc space-y-1 pl-5 text-sm leading-relaxed">
        <GrupoDeChoques tipo="posterior" choques={de('posterior')} />
        <GrupoDeChoques tipo="misma" choques={de('misma')} />
        <GrupoDeChoques tipo="dentro" choques={de('dentro')} />
      </ul>
      <p className="text-suave text-sm leading-relaxed">
        Para partir de cero hay que levantar el sistema limpio (<code className="font-mono">docker compose down -v</code>).
      </p>
      {otrosDias && (
        <div className="space-y-2">
          <p className="text-suave text-sm">
            {libres.length > 0 ? 'Días que todavía moverían el estado de su estación:' : 'Ningún otro día movería el estado de su estación.'}
          </p>
          <div className="flex flex-wrap gap-2">
            {libres.map((d) => (
              <button key={d.id} type="button" className="pildora" onClick={() => otrosDias.alElegir(d.id)}>
                {d.estacion} · {fechaDia(d.dia)}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

/** Si lo único que pasa es que el tramo ya se simuló entero: todas sus estaciones están en su última hora. */
function soloRepite(revision: Revision): boolean {
  return revision.choques.length > 0 && revision.choques.every((c) => c.tipo === 'misma')
}

interface PropsBotones {
  fase: Fase
  /** La corrida a la vista es de este tramo: si no, los botones son los de empezar. */
  deEsteTramo: boolean
  tipo: Tramo['tipo']
  /** null: todavía no se sabe. */
  revision: Revision | null
  /** No se pudo consultar el estado de las estaciones. */
  sinComprobar: boolean
  comprobando: boolean
  puedeEmpezar: boolean
  alEmpezar: () => void
  alPausar: () => void
  alContinuar: () => void
  alDetener: () => void
}

export function Botones({ fase, deEsteTramo, tipo, revision, sinComprobar, comprobando, puedeEmpezar, alEmpezar, alPausar, alContinuar, alDetener }: PropsBotones) {
  const detener = (
    <button type="button" className="pildora" onClick={alDetener}>
      Detener
    </button>
  )
  if (fase === 'en_curso') {
    return (
      <Fila>
        <button type="button" className="boton" onClick={alPausar}>
          Pausar
        </button>
        {detener}
      </Fila>
    )
  }
  if (fase === 'en_pausa' || fase === 'con_error') {
    return (
      <Fila>
        <button type="button" className="boton" onClick={alContinuar}>
          {fase === 'con_error' ? 'Reintentar esta lectura' : 'Continuar'}
        </button>
        {detener}
      </Fila>
    )
  }
  const repetir = (deEsteTramo && (fase === 'terminada' || fase === 'detenida')) || (revision !== null && soloRepite(revision))
  const que = repetir ? 'Volver a simular' : tipo === 'escenario' ? 'Cargar el escenario' : 'Simular este día'
  // «De todos modos»: hay un aviso a la vista y se empieza igual.
  const conAviso = revision !== null && !revision.limpio && !soloRepite(revision)
  return (
    <Fila>
      <button type="button" className="boton" disabled={!puedeEmpezar || comprobando} onClick={alEmpezar}>
        {comprobando ? 'Consultando el estado…' : conAviso ? `${que} de todos modos` : sinComprobar ? `${que} sin comprobar` : que}
      </button>
    </Fila>
  )
}

function Fila({ children }: { children: ReactNode }) {
  return <div className="flex flex-wrap items-center gap-2">{children}</div>
}

interface PropsRitmo {
  valor: Ritmo
  alCambiar: (ritmo: Ritmo) => void
  horas: number
}

export function ElegirRitmo({ valor, alCambiar, horas }: PropsRitmo) {
  const opciones: Opcion<Ritmo>[] = RITMOS.map((r) => ({ valor: r.valor, etiqueta: r.etiqueta }))
  const segundos = horas / horasPorSegundoDe(valor)
  return (
    <div>
      <p className="rotulo mb-1.5">Ritmo</p>
      <Pildoras etiqueta="Ritmo de la simulación" opciones={opciones} valor={valor} alCambiar={alCambiar} />
      <p className="text-tenue mt-1.5 text-xs">
        {numero(horasPorSegundoDe(valor))} {horasPorSegundoDe(valor) === 1 ? 'hora simulada' : 'horas simuladas'} por segundo: las{' '}
        {numero(horas)} horas tardan unos {numero(segundos)} segundos.
      </p>
    </div>
  )
}

/** La tarjeta que agrupa los controles, con su rótulo según lo que se simula. */
export function TarjetaDeControles({ tipo, children }: { tipo: Tramo['tipo']; children: ReactNode }) {
  return (
    <Tarjeta
      rotulo="Qué se manda a la API"
      titulo={tipo === 'escenario' ? 'El escenario' : 'Un día de una estación'}
    >
      <div className="space-y-5">{children}</div>
    </Tarjeta>
  )
}
