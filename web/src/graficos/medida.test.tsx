// El ayudante que mide la caja de un gráfico: sin ResizeObserver vale lo dado
// por omisión; con él, el ancho y el alto de la caja y el rem de la página, al
// día cuando cambia el tamaño o el modo proyector.

import { act, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useMedida } from './medida'

function Caja({ porOmision }: { porOmision: { ancho: number; alto?: number } }) {
  const [caja, medida] = useMedida<HTMLDivElement>(porOmision)
  return (
    <div ref={caja} data-testid="caja">
      {medida.ancho}×{medida.alto} · rem {medida.rem} · escala {medida.escala}
    </div>
  )
}

/** Un ResizeObserver que se dispara a mano, y una caja y un rem del tamaño que se diga. */
function conMedidas(inicial: { ancho: number; alto: number; rem: number }) {
  const actual = { ...inicial }
  const avisos = new Set<() => void>()
  vi.stubGlobal(
    'ResizeObserver',
    class {
      private readonly aviso: () => void
      constructor(aviso: () => void) {
        this.aviso = aviso
      }
      observe() {
        avisos.add(this.aviso)
      }
      disconnect() {
        avisos.delete(this.aviso)
      }
    },
  )
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(
    () => ({ width: actual.ancho, height: actual.alto }) as DOMRect,
  )
  const estilos = globalThis.getComputedStyle.bind(globalThis)
  vi.spyOn(globalThis, 'getComputedStyle').mockImplementation((elemento, pseudo) =>
    elemento === document.documentElement ? ({ fontSize: `${actual.rem}px` } as CSSStyleDeclaration) : estilos(elemento, pseudo),
  )
  return {
    actual,
    avisos,
    cambiarTamano: () => act(() => avisos.forEach((aviso) => aviso())),
  }
}

describe('useMedida', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('sin con qué medir (jsdom) vale lo dado por omisión, con el rem de 16', () => {
    render(<Caja porOmision={{ ancho: 640 }} />)
    expect(screen.getByTestId('caja')).toHaveTextContent('640×0 · rem 16 · escala 1')
  })

  it('mide la caja al montarse: píxeles enteros que no la desbordan', () => {
    conMedidas({ ancho: 812.7, alto: 300.4, rem: 16 })
    render(<Caja porOmision={{ ancho: 640, alto: 200 }} />)
    expect(screen.getByTestId('caja')).toHaveTextContent('812×300 · rem 16 · escala 1')
  })

  it('vuelve a medir cuando la caja cambia de tamaño', () => {
    const medidas = conMedidas({ ancho: 800, alto: 300, rem: 16 })
    render(<Caja porOmision={{ ancho: 640 }} />)
    medidas.actual.ancho = 500
    medidas.cambiarTamano()
    expect(screen.getByTestId('caja')).toHaveTextContent('500×300')
  })

  it('una caja sin ancho (oculta) conserva lo último medido', () => {
    const medidas = conMedidas({ ancho: 800, alto: 300, rem: 16 })
    render(<Caja porOmision={{ ancho: 640 }} />)
    medidas.actual.ancho = 0
    medidas.actual.alto = 0
    medidas.cambiarTamano()
    expect(screen.getByTestId('caja')).toHaveTextContent('800×300')
  })

  it('el modo proyector cambia el rem aunque la caja conserve su ancho', async () => {
    const medidas = conMedidas({ ancho: 800, alto: 300, rem: 16 })
    render(<Caja porOmision={{ ancho: 640 }} />)
    medidas.actual.rem = 19
    act(() => {
      document.documentElement.dataset.modo = 'proyector'
    })
    await waitFor(() => expect(screen.getByTestId('caja')).toHaveTextContent('800×300 · rem 19 · escala 1.1875'))
  })

  it('al desmontarse deja de mirar', () => {
    const medidas = conMedidas({ ancho: 800, alto: 300, rem: 16 })
    const { unmount } = render(<Caja porOmision={{ ancho: 640 }} />)
    expect(medidas.avisos.size).toBe(1)
    unmount()
    expect(medidas.avisos.size).toBe(0)
  })
})
