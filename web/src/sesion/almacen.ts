// La sesión: el token, el rol y cuándo vence (spec §10).
//
// Vive en sessionStorage: sobrevive a recargar la página (un F5 en plena
// sustentación no saca de la sesión) y se borra al cerrar la pestaña. No va en
// localStorage. El token dura lo que diga la API y no se renueva: al vencer, o
// cuando la API responde 401, la sesión se cierra y la UI vuelve al ingreso.
//
// Es un almacén fuera de React (suscribir + estadoSesion) para que el cliente
// de la API y el WebSocket lo lean sin pasar por un componente.

import type { components } from '../api/contrato'

export type Rol = components['schemas']['Token']['rol']

export interface Sesion {
  usuario: string
  token: string
  rol: Rol
  /** Cuándo vence el token, en milisegundos desde la época. */
  expira: number
}

/** Por qué no hay sesión: nunca la hubo (null), el usuario salió, o venció o la API la rechazó. */
export type MotivoCierre = 'salir' | 'vencida'

export interface EstadoSesion {
  sesion: Sesion | null
  motivo: MotivoCierre | null
}

const CLAVE = 'aire.sesion'
// setTimeout guarda la espera en 32 bits con signo: más que esto se dispara al instante.
const ESPERA_MAXIMA = 2 ** 31 - 1

let estado: EstadoSesion = { sesion: leer(), motivo: null }
let reloj: ReturnType<typeof setTimeout> | undefined
const oyentes = new Set<() => void>()

programarVencimiento()

function esSesion(x: unknown): x is Sesion {
  if (typeof x !== 'object' || x === null) return false
  const s = x as Record<string, unknown>
  return (
    typeof s.usuario === 'string' &&
    typeof s.token === 'string' &&
    s.token !== '' &&
    (s.rol === 'usuario' || s.rol === 'admin') &&
    typeof s.expira === 'number' &&
    Number.isFinite(s.expira)
  )
}

function leer(): Sesion | null {
  try {
    const crudo = sessionStorage.getItem(CLAVE)
    if (crudo === null) return null
    const s: unknown = JSON.parse(crudo)
    if (esSesion(s) && s.expira > Date.now()) return s
  } catch {
    // Un valor ilegible o un sessionStorage bloqueado: como si no hubiera sesión.
  }
  borrar()
  return null
}

function borrar() {
  try {
    sessionStorage.removeItem(CLAVE)
  } catch {
    // Nada que hacer: sin sessionStorage tampoco había nada guardado.
  }
}

function programarVencimiento() {
  clearTimeout(reloj)
  reloj = undefined
  const s = estado.sesion
  if (!s) return
  const falta = s.expira - Date.now()
  if (falta <= 0) {
    cerrarSesion('vencida')
    return
  }
  // Si la espera no entra en un setTimeout, se vuelve a programar al despertar.
  reloj = setTimeout(programarVencimiento, Math.min(falta, ESPERA_MAXIMA))
}

function cambiar(nuevo: EstadoSesion) {
  estado = nuevo
  programarVencimiento()
  for (const f of [...oyentes]) f()
}

export function estadoSesion(): EstadoSesion {
  return estado
}

export function sesionActual(): Sesion | null {
  return estado.sesion
}

export function suscribir(oyente: () => void): () => void {
  oyentes.add(oyente)
  return () => oyentes.delete(oyente)
}

export function abrirSesion(sesion: Sesion) {
  try {
    sessionStorage.setItem(CLAVE, JSON.stringify(sesion))
  } catch {
    // Sin sessionStorage la sesión dura lo que dure la página abierta.
  }
  cambiar({ sesion, motivo: null })
}

export function cerrarSesion(motivo: MotivoCierre) {
  borrar()
  // Sin sesión no hay nada que cerrar: el motivo de la última vez se queda.
  if (estado.sesion === null) return
  cambiar({ sesion: null, motivo })
}

/** Solo para las pruebas: vuelve a leer sessionStorage como al cargar la página. */
export function recargarSesion() {
  estado = { sesion: leer(), motivo: null }
  programarVencimiento()
  for (const f of [...oyentes]) f()
}
