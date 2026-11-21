import { Suspense } from 'react'
import { Outlet } from 'react-router'
import { Estado } from '../componentes/Estado'
import { AvisoVencimiento } from './AvisoVencimiento'
import { Barra } from './Barra'
import { Pie } from './Pie'

/**
 * El armazón de las pantallas con sesión: la barra arriba, la pantalla en el
 * medio y el pie abajo. Cada pantalla se carga cuando se la visita (App.tsx).
 */
export function Armazon() {
  return (
    <div className="flex min-h-dvh flex-col">
      <a href="#contenido" className="pildora sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2">
        Saltar al contenido
      </a>
      <Barra />
      <AvisoVencimiento />
      <main id="contenido" className="mx-auto w-full max-w-7xl flex-1 px-6 py-6">
        <Suspense fallback={<Estado tipo="cargando" titulo="Cargando la pantalla…" />}>
          <Outlet />
        </Suspense>
      </main>
      <Pie />
    </div>
  )
}
