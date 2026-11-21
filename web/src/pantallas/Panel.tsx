import { EnConstruccion } from '../componentes/EnConstruccion'

// Panel (spec §5.2). El armazón, la ruta y la guarda ya están: esta pantalla se
// construye aquí, reemplazando el marcador.
export default function Panel() {
  return (
    <EnConstruccion pantalla="Panel" seccion="§5.2" archivo="web/src/pantallas/Panel.tsx">
      <p>
        Aquí va el mapa del Perú con las 33 estaciones, las tres cifras de arriba (estaciones en alerta, estaciones con
        estado y la fecha del último estado recibido) y la lista ordenada por gravedad.
      </p>
      <p>Se actualiza sola con el evento «estacion.regimen» del WebSocket.</p>
    </EnConstruccion>
  )
}
