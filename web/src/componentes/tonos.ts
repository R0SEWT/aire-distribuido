// Los tonos con que una tarjeta, una frase o una etiqueta toman color. Las
// clases van escritas enteras: Tailwind solo genera las que encuentra en el código.

export type Tono = 'neutro' | 'acento' | 'buena' | 'moderada' | 'mala' | 'alerta'

export const BORDE_IZQUIERDO: Record<Tono, string> = {
  neutro: 'border-l-borde',
  acento: 'border-l-acento',
  buena: 'border-l-inca-buena',
  moderada: 'border-l-inca-moderada',
  mala: 'border-l-inca-mala',
  alerta: 'border-l-inca-cuidado',
}

export const TEXTO: Record<Tono, string> = {
  neutro: 'text-suave',
  acento: 'text-acento-claro',
  buena: 'text-inca-buena',
  moderada: 'text-inca-moderada',
  mala: 'text-inca-mala',
  alerta: 'text-inca-cuidado',
}

export const BORDE: Record<Tono, string> = {
  neutro: 'border-borde',
  acento: 'border-acento',
  buena: 'border-inca-buena',
  moderada: 'border-inca-moderada',
  mala: 'border-inca-mala',
  alerta: 'border-inca-cuidado',
}
