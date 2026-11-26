import { BANDAS, estadoDeBanda, type BandaINCA, type EstadoBanda } from './bandas'

interface Props {
  /** La banda que manda la API (null: sin datos suficientes) o 'no_mide'. */
  valor: BandaINCA | EstadoBanda | undefined
  /** Antepone «INCA» a la palabra, para cuando la etiqueta va sola. */
  conPrefijo?: boolean
}

/** La banda INCA de una estación: un punto del color de la banda y, siempre, su palabra. */
export function Banda({ valor, conPrefijo = false }: Props) {
  const estado = estadoDeBanda(valor)
  const info = BANDAS[estado]
  const esBanda = estado !== 'sin_datos' && estado !== 'no_mide'
  return (
    <span
      className="border-borde bg-relieve text-texto inline-flex items-center gap-2 rounded-full border px-2.5 py-0.5 text-sm font-semibold whitespace-nowrap"
      data-banda={estado}
    >
      <span aria-hidden="true" className={`size-2.5 shrink-0 rounded-full ${info.fondo}`} />
      {conPrefijo && esBanda ? `INCA ${info.palabra.toLowerCase()}` : info.palabra}
    </span>
  )
}
