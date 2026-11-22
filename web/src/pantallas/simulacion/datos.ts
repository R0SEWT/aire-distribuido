// Los archivos estáticos de /datos/ (web/public/datos/README.md): series.json e
// impacto.json. No pasan por la API ni por su cliente tipado: los sirve el mismo
// origen, y uno que no existe responde 404 (no la página).
//
// Lo usan Simulación e Impacto. Las claves de caché van aquí porque
// api/cliente.ts todavía no tiene una para los datos estáticos.

import { ErrorApi, errorDeFetch, errorDeRespuesta, esCancelacion } from '../../api/errores'

export const clavesDatos = {
  series: ['datos', 'series'] as const,
  impacto: ['datos', 'impacto'] as const,
}

/** Los datos estáticos no cambian mientras la página está abierta: se piden una vez. */
export const DATOS_ESTATICOS = { staleTime: Infinity, gcTime: Infinity } as const

/**
 * Pide un archivo de /datos/ y lo devuelve ya leído. Si no está o no es JSON,
 * lanza un ErrorApi, como pedir(): la pantalla se lo pasa a <EstadoDeError>.
 */
export async function pedirDatos<T>(archivo: string, signal?: AbortSignal): Promise<T> {
  const ruta = `/datos/${archivo}`
  let respuesta: Response
  try {
    respuesta = await globalThis.fetch(new Request(new URL(ruta, globalThis.location.origin), { signal }))
  } catch (e) {
    if (esCancelacion(e)) throw e
    throw errorDeFetch(e)
  }
  if (!respuesta.ok) {
    throw errorDeRespuesta(respuesta.status, { error: `no se pudo leer ${ruta} (respondió ${respuesta.status})` }, true)
  }
  try {
    return (await respuesta.json()) as T
  } catch {
    throw new ErrorApi('inesperado', respuesta.status, `${ruta} no es un JSON válido`)
  }
}
