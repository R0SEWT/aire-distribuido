import { useEffect, useState } from 'react'
import { hora } from '../componentes/formato'
import { useSesion } from '../sesion/useSesion'

/** Con cuánta anticipación se avisa que la sesión vence. */
export const AVISO_MS = 5 * 60_000

function useAhora(cadaMs: number): number {
  const [ahora, setAhora] = useState(() => Date.now())
  useEffect(() => {
    const reloj = setInterval(() => setAhora(Date.now()), cadaMs)
    return () => clearInterval(reloj)
  }, [cadaMs])
  return ahora
}

/**
 * Avisa antes de que venza la sesión (spec §10). El token no se renueva: lo
 * que se puede hacer es volver a ingresar cuando convenga, en vez de que la
 * sesión se corte en medio de algo.
 */
export function AvisoVencimiento() {
  const { sesion, salir } = useSesion()
  const ahora = useAhora(10_000)
  if (!sesion) return null
  const falta = sesion.expira - ahora
  if (falta > AVISO_MS || falta <= 0) return null
  const minutos = Math.ceil(falta / 60_000)
  return (
    <div role="alert" className="border-aviso bg-relieve border-b">
      <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-3 px-6 py-2.5 text-sm">
        <p className="text-texto">
          <strong className="text-aviso font-semibold">La sesión está por vencer.</strong> Se cierra a las{' '}
          <span className="font-mono">{hora(sesion.expira)}</span> (en {minutos === 1 ? 'menos de 1 minuto' : `${minutos} minutos`})
          y no se renueva sola.
        </p>
        <button type="button" className="pildora" onClick={salir}>
          Volver a ingresar ahora
        </button>
      </div>
    </div>
  )
}
