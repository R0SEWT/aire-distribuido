import { Suspense, useEffect, useLayoutEffect, useRef } from 'react'
import { Outlet, useLocation } from 'react-router'
import { useRegistroDeSesion } from '../api/historia'
import { Barrera } from '../componentes/Barrera'
import { Estado } from '../componentes/Estado'
import { AvisoVencimiento } from './AvisoVencimiento'
import { Barra } from './Barra'
import { tituloDe } from './menu'
import { Pie } from './Pie'

/**
 * El armazón de las pantallas con sesión: la barra arriba, la pantalla en el
 * medio y el pie abajo. Cada pantalla se carga cuando se la visita (App.tsx).
 *
 * Si una pantalla falla al dibujarse, o su archivo no se puede descargar, la
 * barrera lo muestra en su lugar: la barra y el menú siguen ahí.
 */
export function Armazon() {
  const { pathname } = useLocation()
  const principal = useRef<HTMLElement>(null)
  const primeraVez = useRef(true)
  // Lo que llega de cada estación queda anotado para su curva, se mire la pantalla que se mire.
  useRegistroDeSesion()

  // El título de la ruta. En un efecto de layout, que corre antes que los
  // efectos comunes: una pantalla puede poner uno más preciso con useTitulo.
  useLayoutEffect(() => {
    document.title = tituloDe(pathname)
  }, [pathname])

  // Al navegar, el foco va al contenido: quien usa teclado o lector de
  // pantalla no se queda en el menú. Al cargar la página, no.
  useEffect(() => {
    if (primeraVez.current) {
      primeraVez.current = false
      return
    }
    principal.current?.focus()
  }, [pathname])

  return (
    <div className="flex min-h-dvh flex-col">
      <a href="#contenido" className="pildora sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2">
        Saltar al contenido
      </a>
      <Barra />
      <AvisoVencimiento />
      <main id="contenido" ref={principal} tabIndex={-1} className="mx-auto w-full max-w-7xl flex-1 px-6 py-6 focus:outline-none">
        <Barrera alcance="pantalla" reiniciarCon={pathname}>
          <Suspense fallback={<Estado tipo="cargando" titulo="Cargando la pantalla…" />}>
            <Outlet />
          </Suspense>
        </Barrera>
      </main>
      <Pie />
    </div>
  )
}
