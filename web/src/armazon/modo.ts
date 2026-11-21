// El modo proyector (spec §8): letra más grande y más contraste para la
// sustentación. Es un atributo en <html> que estilos/proyector.css lee; la
// preferencia se recuerda en localStorage (no es un secreto: el token no va ahí).

import { useSyncExternalStore } from 'react'

const CLAVE = 'aire.modo'
const oyentes = new Set<() => void>()

function leer(): boolean {
  try {
    return localStorage.getItem(CLAVE) === 'proyector'
  } catch {
    return false
  }
}

function aplicar(proyector: boolean) {
  if (proyector) document.documentElement.dataset.modo = 'proyector'
  else delete document.documentElement.dataset.modo
}

function enProyector(): boolean {
  return document.documentElement.dataset.modo === 'proyector'
}

/** Pone en <html> el modo guardado. Se llama una vez, antes de dibujar. */
export function iniciarModo() {
  aplicar(leer())
}

export function ponerModoProyector(proyector: boolean) {
  aplicar(proyector)
  try {
    if (proyector) localStorage.setItem(CLAVE, 'proyector')
    else localStorage.removeItem(CLAVE)
  } catch {
    // Sin localStorage el modo dura hasta recargar la página.
  }
  for (const oyente of [...oyentes]) oyente()
}

function suscribir(oyente: () => void) {
  oyentes.add(oyente)
  return () => oyentes.delete(oyente)
}

/** Si el modo proyector está puesto, y cómo cambiarlo. */
export function useModoProyector(): [boolean, (proyector: boolean) => void] {
  return [useSyncExternalStore(suscribir, enProyector), ponerModoProyector]
}
