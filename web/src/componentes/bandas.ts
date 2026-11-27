// Las bandas del INCA y los dos estados sin banda, con su palabra y su color
// (spec §5.2 y §8). El color nunca va solo: quien dibuje una banda, en una
// etiqueta o en el mapa, muestra también la palabra.

import type { components } from '../api/contrato'

/** La banda como la manda la API: null con menos de 18 lecturas en la ventana. */
export type BandaINCA = components['schemas']['BandaINCA']

/**
 * Lo que se puede decir de una estación: su banda, que todavía no tiene datos
 * suficientes, o que no mide PM por hora (16 de las 33 nunca tendrán estado).
 */
export type EstadoBanda = NonNullable<BandaINCA> | 'sin_datos' | 'no_mide'

export interface InfoBanda {
  palabra: string
  /** La variable CSS del color, para D3: .style('fill', info.color). */
  color: string
  /** La clase de Tailwind del fondo. */
  fondo: string
  /** De menos a más grave, para ordenar listas. Los estados sin banda van al final. */
  gravedad: number
}

export const BANDAS: Record<EstadoBanda, InfoBanda> = {
  buena: { palabra: 'Buena', color: 'var(--color-inca-buena)', fondo: 'bg-inca-buena', gravedad: 1 },
  moderada: { palabra: 'Moderada', color: 'var(--color-inca-moderada)', fondo: 'bg-inca-moderada', gravedad: 2 },
  mala: { palabra: 'Mala', color: 'var(--color-inca-mala)', fondo: 'bg-inca-mala', gravedad: 3 },
  umbral_de_cuidado: {
    palabra: 'Umbral de cuidado',
    color: 'var(--color-inca-cuidado)',
    fondo: 'bg-inca-cuidado',
    gravedad: 4,
  },
  sin_datos: { palabra: 'Sin datos suficientes', color: 'var(--color-sin-datos)', fondo: 'bg-sin-datos', gravedad: 0 },
  no_mide: { palabra: 'No mide PM por hora', color: 'var(--color-no-mide)', fondo: 'bg-no-mide trama', gravedad: -1 },
}

/** La banda de la API como estado: null es «sin datos suficientes». */
export function estadoDeBanda(banda: BandaINCA | EstadoBanda | undefined): EstadoBanda {
  return banda ?? 'sin_datos'
}
