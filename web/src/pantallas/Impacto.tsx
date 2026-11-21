import { EnConstruccion } from '../componentes/EnConstruccion'

// Impacto (spec §5.7). El armazón, la ruta y la guarda ya están: esta pantalla se
// construye aquí, reemplazando el marcador.
export default function Impacto() {
  return (
    <EnConstruccion pantalla="Impacto" seccion="§5.7" archivo="web/src/pantallas/Impacto.tsx">
      <p>
        Aquí va la lectura de impacto social con los datos reales: cuántas horas superaron el ECA, dónde y junto a qué
        actividad, quién vive en cada distrito y lo que este dato no puede decir.
      </p>
    </EnConstruccion>
  )
}
