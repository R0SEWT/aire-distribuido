import { NavLink } from 'react-router'
import { useSesion } from '../sesion/useSesion'
import { BotonProyector } from './BotonProyector'
import { EstadoApi, EstadoTiempoReal, SimulacionEnCurso } from './Indicadores'
import { Marca } from './Marca'
import { menuDe, RUTAS } from './menu'

const NOMBRE_DE_ROL = { usuario: 'usuario', admin: 'administrador' } as const

/**
 * La barra superior (spec §5): el nombre del sistema, el estado de la API y
 * del tiempo real, la simulación en curso si hay una, el menú en píldoras según
 * el rol y, a la derecha, el modo proyector y salir.
 */
export function Barra() {
  const { sesion, salir } = useSesion()
  if (!sesion) return null
  return (
    <header className="border-borde border-b">
      <div className="mx-auto flex max-w-7xl flex-wrap items-center gap-x-4 gap-y-2 px-6 py-3">
        <NavLink to={RUTAS.panel} className="inline-flex items-center no-underline hover:no-underline" aria-label="Aire distribuido: ir al panel">
          <Marca />
        </NavLink>
        <EstadoApi />
        <EstadoTiempoReal />
        <div className="ml-auto flex flex-wrap items-center gap-2">
          <span className="text-suave text-sm" data-sesion-rol={sesion.rol}>
            <span className="text-texto font-semibold">{sesion.usuario}</span> · {NOMBRE_DE_ROL[sesion.rol]}
          </span>
          <BotonProyector />
          <button type="button" className="pildora" onClick={salir}>
            Salir
          </button>
        </div>
      </div>
      <div className="mx-auto flex max-w-7xl flex-wrap items-center gap-2 px-6 pb-3">
        <nav aria-label="Pantallas" className="flex flex-wrap gap-2">
          {menuDe(sesion.rol).map((entrada) => (
            <NavLink key={entrada.ruta} to={entrada.ruta} end={entrada.ruta === RUTAS.panel} className="pildora">
              {entrada.etiqueta}
            </NavLink>
          ))}
        </nav>
        {/* En el renglón del menú, que tiene sitio: aparece y se va sin mover nada de lugar. */}
        <div className="ml-auto">
          <SimulacionEnCurso />
        </div>
      </div>
    </header>
  )
}
