// El único lugar que traduce lo que responde la API a lo que la UI muestra
// (spec §5.8 y §9.5). La API responde los errores como {"error": "…"}; aquí se
// decide qué clase de error es y con qué palabras se le cuenta a la persona.
//
// Las pantallas no miran códigos HTTP: reciben un ErrorApi (lo lanza
// api/cliente.ts) y se lo pasan a <EstadoDeError>, o le preguntan a
// presentarError() qué estado de pantalla le corresponde.

export type TipoError =
  /** 401 al ingresar: usuario o clave incorrectos. */
  | 'credenciales'
  /** 401 con una sesión iniciada: el token venció o dejó de valer. La sesión se cierra. */
  | 'sesion_vencida'
  /** 403: el rol no alcanza. */
  | 'sin_permiso'
  /** 404. */
  | 'no_encontrado'
  /** 400 y 422: la API rechazó el pedido y dice por qué. */
  | 'pedido_invalido'
  /** 409: choca con algo en curso (ya hay un entrenamiento). */
  | 'conflicto'
  /** 502, 503 y 504: la API, o un almacén del que depende, no está disponible. */
  | 'no_disponible'
  /** No llegó ninguna respuesta: la API está apagada o no hay red. */
  | 'sin_respuesta'
  | 'inesperado'

export class ErrorApi extends Error {
  readonly tipo: TipoError
  /** El código HTTP, o null si no hubo respuesta. */
  readonly estado: number | null
  /** Las palabras de la API ({"error": "…"}), si las mandó. */
  readonly detalle: string | null

  constructor(tipo: TipoError, estado: number | null, detalle: string | null) {
    super(detalle ?? TITULOS[tipo])
    this.name = 'ErrorApi'
    this.tipo = tipo
    this.estado = estado
    this.detalle = detalle
  }
}

const TITULOS: Record<TipoError, string> = {
  credenciales: 'Usuario o clave incorrectos',
  sesion_vencida: 'La sesión venció',
  sin_permiso: 'No tienes permiso para ver esto',
  no_encontrado: 'No se encontró lo que se pidió',
  pedido_invalido: 'La API no aceptó el pedido',
  conflicto: 'No se puede hacer ahora',
  no_disponible: 'El servicio no está disponible',
  sin_respuesta: 'La API no responde',
  inesperado: 'Algo falló al hablar con la API',
}

/** El texto de {"error": "…"}, si el cuerpo tiene esa forma. */
export function textoDeError(cuerpo: unknown): string | null {
  if (typeof cuerpo === 'object' && cuerpo !== null && 'error' in cuerpo) {
    const e = (cuerpo as { error: unknown }).error
    if (typeof e === 'string' && e.trim() !== '') return e.trim()
  }
  return null
}

function tipoDeEstado(estado: number, conSesion: boolean): TipoError {
  switch (estado) {
    case 400:
    case 422:
      return 'pedido_invalido'
    case 401:
      return conSesion ? 'sesion_vencida' : 'credenciales'
    case 403:
      return 'sin_permiso'
    case 404:
      return 'no_encontrado'
    case 409:
      return 'conflicto'
    case 502: // el proxy (Caddy o Vite) no llegó a la API
    case 503:
    case 504:
      return 'no_disponible'
    default:
      return 'inesperado'
  }
}

/**
 * Traduce una respuesta de error. conSesion dice si el pedido llevaba un token:
 * un 401 significa cosas distintas al ingresar y con la sesión iniciada.
 */
export function errorDeRespuesta(estado: number, cuerpo: unknown, conSesion: boolean): ErrorApi {
  return new ErrorApi(tipoDeEstado(estado, conSesion), estado, textoDeError(cuerpo))
}

/** Traduce lo que lanza fetch cuando no hay respuesta, o cualquier otra excepción. */
export function errorDeExcepcion(e: unknown): ErrorApi {
  if (e instanceof ErrorApi) return e
  // fetch lanza TypeError cuando no pudo conectar (API apagada, sin red, CORS).
  if (e instanceof TypeError) return new ErrorApi('sin_respuesta', null, null)
  return new ErrorApi('inesperado', null, e instanceof Error && e.message !== '' ? e.message : null)
}

/** Si la excepción es un pedido cancelado a propósito (TanStack Query al desmontar). */
export function esCancelacion(e: unknown): boolean {
  return e instanceof DOMException && e.name === 'AbortError'
}

/** Los estados de pantalla de §5.8 que nacen de un error. */
export type EstadoDePantalla = 'error' | 'sin-permiso' | 'sesion-vencida'

export interface ErrorPresentado {
  estado: EstadoDePantalla
  titulo: string
  /** Las palabras de la API, o una explicación propia si no mandó ninguna. */
  detalle: string | null
  /** Si tiene sentido ofrecer «Reintentar»: lo mismo, pedido otra vez, podría salir bien. */
  reintentable: boolean
}

const EXPLICACIONES: Partial<Record<TipoError, string>> = {
  sesion_vencida: 'Vuelve a ingresar para seguir.',
  sin_respuesta: 'Puede estar apagada o reiniciándose. Vuelve a intentar en unos segundos.',
  no_disponible: 'La API está en marcha, pero algo de lo que depende no responde.',
}

/** Cómo se muestra un error, sea o no de la API: título, detalle y si se ofrece reintentar. */
export function presentarError(e: unknown): ErrorPresentado {
  const error = errorDeExcepcion(e)
  const estado: EstadoDePantalla =
    error.tipo === 'sin_permiso' ? 'sin-permiso' : error.tipo === 'sesion_vencida' ? 'sesion-vencida' : 'error'
  return {
    estado,
    titulo: TITULOS[error.tipo],
    detalle: error.detalle ?? EXPLICACIONES[error.tipo] ?? null,
    reintentable: ['no_disponible', 'sin_respuesta', 'inesperado', 'conflicto'].includes(error.tipo),
  }
}

/** Si vale la pena que TanStack Query repita el pedido solo. */
export function convieneReintentar(e: unknown): boolean {
  const tipo = errorDeExcepcion(e).tipo
  return tipo === 'no_disponible' || tipo === 'sin_respuesta'
}
