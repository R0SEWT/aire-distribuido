import { ENLACES } from './enlaces'

/** El pie de toda la app (spec §5.7): de dónde salen los datos y dónde seguir leyendo. */
export function Pie() {
  return (
    <footer className="border-borde text-suave border-t text-sm">
      <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-x-6 gap-y-2 px-6 py-4">
        <p>
          Datos: monitoreo de calidad del aire de{' '}
          <abbr title="Organismo de Evaluación y Fiscalización Ambiental" className="no-underline">
            OEFA
          </abbr>{' '}
          (2018 a 2022), con licencia{' '}
          <a href={ENLACES.licencia} target="_blank" rel="noopener noreferrer">
            ODC-By
          </a>
          . No son lecturas en vivo.
        </p>
        <nav aria-label="Más información" className="flex flex-wrap gap-x-5 gap-y-1">
          <a href={ENLACES.docs} target="_blank" rel="noopener noreferrer">
            API (Swagger)
          </a>
          <a href={ENLACES.repositorio} target="_blank" rel="noopener noreferrer">
            Repositorio
          </a>
          <a href={ENLACES.informe} target="_blank" rel="noopener noreferrer">
            Informe
          </a>
        </nav>
      </div>
    </footer>
  )
}
