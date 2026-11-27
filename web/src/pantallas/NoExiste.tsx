import { Link } from 'react-router'
import { RUTAS } from '../armazon/menu'
import { Estado } from '../componentes/Estado'

/** Una dirección que no es de ninguna pantalla. */
export default function NoExiste() {
  return (
    <Estado tipo="vacio" titulo="Esta dirección no existe">
      No hay ninguna pantalla aquí. <Link to={RUTAS.panel}>Volver al panel</Link>.
    </Estado>
  )
}
