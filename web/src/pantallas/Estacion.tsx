import { useParams } from 'react-router'
import { EnConstruccion } from '../componentes/EnConstruccion'

// Estación (spec §5.3). El armazón y las rutas ya están (/estacion y
// /estacion/:id): esta pantalla se construye aquí, reemplazando el marcador.
export default function Estacion() {
  const { id } = useParams()
  return (
    <EnConstruccion pantalla="Estación" seccion="§5.3" archivo="web/src/pantallas/Estacion.tsx">
      <p>
        Aquí va el detalle de una estación: la media de 24 horas de PM2.5 y de PM10 en grande, si hay alerta, la banda
        INCA, la curva con la línea del ECA y la ficha con su distrito y su población.
      </p>
      <p>
        {id ? (
          <>
            Estación pedida: <code className="font-mono">{id}</code>.
          </>
        ) : (
          'Se llega desde el panel, eligiendo una estación.'
        )}
      </p>
    </EnConstruccion>
  )
}
