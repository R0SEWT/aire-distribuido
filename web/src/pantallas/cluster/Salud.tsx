import type { ReactNode } from 'react'
import type { components } from '../../api/contrato'

type Salud = components['schemas']['Salud']

function Componente({ nombre, bien, children }: { nombre: string; bien: boolean; children: ReactNode }) {
  return (
    <div className="bg-relieve min-w-0 rounded-lg px-3 py-2.5" data-componente={nombre} data-bien={bien}>
      <dt className="rotulo">{nombre}</dt>
      <dd className="text-texto mt-1 flex min-w-0 items-center gap-2 text-sm font-semibold">
        <span aria-hidden="true" className={`size-2.5 shrink-0 rounded-full ${bien ? 'bg-ok' : 'bg-falla'}`} />
        <span className="min-w-0 break-words">{children}</span>
      </dd>
    </div>
  )
}

interface Props {
  /** La respuesta de /api/salud; undefined si todavía no llegó o si la API no contestó. */
  salud: Salud | undefined
  /** Todavía no hubo ninguna respuesta ni ningún error. */
  consultando: boolean
}

/** La salud de cada componente (spec §5.6): API, MongoDB, Redis y modelo activo. */
export function Salud({ salud, consultando }: Props) {
  if (consultando && !salud) return <p className="text-suave text-sm">Consultando /api/salud…</p>
  // Sin respuesta de /api/salud, lo único que se sabe es que la API no contestó.
  return (
    <dl className="grid grid-cols-2 gap-3">
      <Componente nombre="API" bien={salud?.api === 'ok'}>
        {salud ? 'Responde' : 'No responde'}
      </Componente>
      <Componente nombre="MongoDB" bien={salud?.mongo === 'ok'}>
        {salud ? (salud.mongo === 'ok' ? 'Responde' : 'No responde') : 'Sin dato'}
      </Componente>
      <Componente nombre="Redis" bien={salud?.redis === 'ok'}>
        {salud ? (salud.redis === 'ok' ? 'Responde' : 'No responde') : 'Sin dato'}
      </Componente>
      <Componente nombre="Modelo activo" bien={Boolean(salud?.modelo_activo)}>
        {salud ? (salud.modelo_activo ? <span className="font-mono text-xs">{salud.modelo_activo}</span> : 'No hay') : 'Sin dato'}
      </Componente>
    </dl>
  )
}
