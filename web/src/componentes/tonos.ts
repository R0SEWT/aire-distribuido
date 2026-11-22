// Los tonos con que una tarjeta, una frase o una etiqueta toman color. Las
// clases van escritas enteras: Tailwind solo genera las que encuentra en el código.

/**
 * `aviso` es para lo que conviene saber antes de seguir (una pausa, un sistema
 * que no está limpio); `moderada` es la banda INCA. Comparten color, no sentido.
 */
export type Tono = 'neutro' | 'acento' | 'buena' | 'moderada' | 'mala' | 'alerta' | 'aviso'

export const BORDE_IZQUIERDO: Record<Tono, string> = {
  neutro: 'border-l-borde',
  acento: 'border-l-acento',
  buena: 'border-l-inca-buena',
  moderada: 'border-l-inca-moderada',
  mala: 'border-l-inca-mala',
  alerta: 'border-l-inca-cuidado',
  aviso: 'border-l-aviso',
}

export const TEXTO: Record<Tono, string> = {
  neutro: 'text-suave',
  acento: 'text-acento-claro',
  buena: 'text-inca-buena',
  moderada: 'text-inca-moderada',
  mala: 'text-inca-mala',
  // El rojo de la banda no da contraste de texto sobre una tarjeta: el texto usa uno más claro.
  alerta: 'text-falla',
  aviso: 'text-aviso',
}

export const BORDE: Record<Tono, string> = {
  neutro: 'border-borde',
  acento: 'border-acento',
  buena: 'border-inca-buena',
  moderada: 'border-inca-moderada',
  mala: 'border-inca-mala',
  alerta: 'border-inca-cuidado',
  aviso: 'border-aviso',
}
