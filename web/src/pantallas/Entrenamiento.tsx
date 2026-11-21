import { EnConstruccion } from '../componentes/EnConstruccion'

// Entrenamiento (spec §5.5). El armazón, la ruta y la guarda ya están: esta pantalla se
// construye aquí, reemplazando el marcador.
export default function Entrenamiento() {
  return (
    <EnConstruccion pantalla="Entrenamiento" seccion="§5.5" archivo="web/src/pantallas/Entrenamiento.tsx">
      <p>
        Aquí se lanza un entrenamiento distribuido (nodos, réplicas y semilla), se reproducen sus rondas a ritmo humano
        con los eventos «entrenamiento.ronda» y «entrenamiento.fin», y se ve el historial.
      </p>
      <p>Solo para administradores.</p>
    </EnConstruccion>
  )
}
