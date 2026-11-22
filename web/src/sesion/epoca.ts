// La «época» de la sesión: una marca que avanza cada vez que una sesión termina.
//
// Lo que vive fuera de React (el entrenamiento a la vista, las lecturas
// enviadas…) se vacía al terminar la sesión, pero un pedido que estaba en viaje
// puede responder después: su callback escribiría en un almacén ya vacío y lo
// heredaría quien entre a continuación en la misma pestaña. Quien vaya a
// escribir en uno de esos almacenes con la respuesta de un pedido toma la marca
// antes de pedir y la comprueba al responder:
//
//   const sigue = marcarSesion()
//   const respuesta = await pedir(…)
//   if (!sigue()) return        // la sesión que lo pidió ya terminó
//   almacen.anotar(respuesta)
//
// La avanza limpiarLoDeLaSesion() (App.tsx), que es donde se vacía todo.

let epoca = 0

/** La sesión terminó: lo que se pidió antes ya no es de la sesión vigente. */
export function avanzarEpoca() {
  epoca += 1
}

/** Marca la sesión vigente. Lo que devuelve dice, cuando se le pregunte, si sigue siendo la misma. */
export function marcarSesion(): () => boolean {
  const alMarcar = epoca
  return () => alMarcar === epoca
}
