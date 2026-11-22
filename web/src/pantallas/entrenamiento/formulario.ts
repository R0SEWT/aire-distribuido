// Las reglas del formulario de un entrenamiento (spec §5.5), sin React.
//
// Aquí se valida lo que se puede saber sin preguntar: que los tres campos sean
// enteros y estén en rango. Lo demás lo decide la API y se muestra con sus
// palabras: si hay nodos libres (422), si ya hay un entrenamiento (409), si las
// réplicas entran en la memoria de los nodos (400) y si la API puede entrenar (503).

import type { Pedido } from './seguimiento'

export interface Campos {
  nodos: string
  replicas: string
  semilla: string
}

export type Errores = Partial<Record<keyof Campos, string>>

/** Las réplicas del preajuste «medición larga»: dura lo suficiente para verla avanzar en vivo. */
export const REPLICAS_MEDICION = 300
export const SEMILLA_POR_OMISION = 42

const ENTERO = /^[+-]?\d+$/

function entero(texto: string): number | null {
  const limpio = texto.trim()
  if (!ENTERO.test(limpio)) return null
  return Number(limpio)
}

function deUnoOMas(texto: string, que: string): { valor: number } | { error: string } {
  if (texto.trim() === '') return { error: `Falta ${que}.` }
  const valor = entero(texto)
  if (valor === null) return { error: 'Tiene que ser un número entero, sin decimales.' }
  if (valor < 1) return { error: 'Tiene que ser 1 o más.' }
  if (!Number.isSafeInteger(valor)) return { error: 'Es un número demasiado grande.' }
  return { valor }
}

/** Valida los tres campos. Devuelve el pedido listo para la API, o el error de cada campo que lo tenga. */
export function validar(campos: Campos): { pedido: Pedido; errores: null } | { pedido: null; errores: Errores } {
  const errores: Errores = {}

  const nodos = deUnoOMas(campos.nodos, 'el número de nodos')
  if ('error' in nodos) errores.nodos = nodos.error

  const replicas = deUnoOMas(campos.replicas, 'el número de réplicas')
  if ('error' in replicas) errores.replicas = replicas.error

  let semilla = 0
  if (campos.semilla.trim() === '') errores.semilla = 'Falta la semilla.'
  else {
    const valor = entero(campos.semilla)
    if (valor === null) errores.semilla = 'Tiene que ser un número entero; puede ser negativo.'
    // La API acepta un int64, pero el navegador solo representa sin error hasta 2^53.
    else if (!Number.isSafeInteger(valor)) errores.semilla = 'Es demasiado grande: usa hasta 15 cifras.'
    else semilla = valor
  }

  if ('error' in nodos || 'error' in replicas || errores.semilla) return { pedido: null, errores }
  return { pedido: { nodos: nodos.valor, replicas: replicas.valor, semilla }, errores: null }
}

/** «1 nodo», «3 nodos». */
export function cuantosNodos(n: number): string {
  return n === 1 ? '1 nodo' : `${n} nodos`
}
