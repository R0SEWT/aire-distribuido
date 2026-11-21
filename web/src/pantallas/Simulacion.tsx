import { EnConstruccion } from '../componentes/EnConstruccion'

// Simulación (spec §5.4). El armazón, la ruta y la guarda ya están: esta pantalla se
// construye aquí, reemplazando el marcador.
export default function Simulacion() {
  return (
    <EnConstruccion pantalla="Simulación" seccion="§5.4" archivo="web/src/pantallas/Simulacion.tsx">
      <p>
        Aquí se carga el escenario (48 horas reales de diciembre de 2021) y se simula un día de una estación, lectura por
        lectura, para ver cómo se llena la ventana de 24 horas y cuándo se enciende la alerta.
      </p>
      <p>La simulación escribe el último estado de la estación, que es compartido: la pantalla lo dice antes de empezar.</p>
    </EnConstruccion>
  )
}
