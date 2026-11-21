import { EnConstruccion } from '../componentes/EnConstruccion'

// Clúster (spec §5.6). El armazón, la ruta y la guarda ya están: esta pantalla se
// construye aquí, reemplazando el marcador.
export default function Cluster() {
  return (
    <EnConstruccion pantalla="Clúster" seccion="§5.6" archivo="web/src/pantallas/Cluster.tsx">
      <p>
        Aquí van los nodos del clúster, el CPU y la memoria del coordinador, la latencia de la API por ruta, las
        escrituras por almacén, los eventos del clúster y la salud de cada componente.
      </p>
      <p>Solo para administradores.</p>
    </EnConstruccion>
  )
}
