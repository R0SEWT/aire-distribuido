import { Component, type ErrorInfo, type ReactNode } from 'react'
import { esFalloDeCarga } from '../api/errores'
import { Estado } from './Estado'

interface Props {
  /**
   * Qué protege: una pantalla (el armazón sigue a la vista y se puede
   * reintentar) o la aplicación entera (la última red: recargar).
   */
  alcance: 'pantalla' | 'aplicacion'
  children: ReactNode
  /**
   * Cuando este valor cambia, el error se olvida y lo de adentro se vuelve a
   * dibujar. El armazón pasa la ruta: navegar a otra pantalla limpia el error
   * de la anterior, sin desmontar una pantalla sana.
   */
  reiniciarCon?: unknown
  /** Para las pruebas. Por omisión recarga la página. */
  recargar?: () => void
}

interface EstadoDeBarrera {
  fallo: { error: unknown } | null
}

/**
 * La barrera de errores. Si algo de adentro lanza al dibujarse, React desmonta
 * todo el árbol hasta la barrera más cercana: sin ninguna, la página queda en
 * blanco. Con ella se muestra qué pasó y qué hacer.
 *
 * Distingue dos casos:
 * - Un trozo de la app que no se pudo descargar (cada pantalla se carga al
 *   visitarla; si la app se actualizó con la pestaña abierta, ese archivo ya
 *   no existe). Reintentar no sirve: se ofrece recargar la página.
 * - Un error de la pantalla. Se ofrece reintentar, que la vuelve a dibujar.
 */
export class Barrera extends Component<Props, EstadoDeBarrera> {
  override state: EstadoDeBarrera = { fallo: null }

  static getDerivedStateFromError(error: unknown): EstadoDeBarrera {
    return { fallo: { error } }
  }

  override componentDidUpdate(previas: Props) {
    if (this.state.fallo && previas.reiniciarCon !== this.props.reiniciarCon) this.setState({ fallo: null })
  }

  override componentDidCatch(error: unknown, info: ErrorInfo) {
    console.error(`la ${this.props.alcance} falló al dibujarse`, error, info.componentStack)
  }

  private readonly recargar = () => (this.props.recargar ?? (() => window.location.reload()))()

  override render() {
    const { fallo } = this.state
    if (!fallo) return this.props.children
    const dentro = this.contenido(fallo.error)
    if (this.props.alcance === 'pantalla') return dentro
    return <div className="mx-auto flex min-h-dvh max-w-2xl flex-col justify-center px-6 py-10">{dentro}</div>
  }

  private contenido(error: unknown) {
    const recargar = { etiqueta: 'Recargar la página', alHacer: this.recargar }
    if (esFalloDeCarga(error)) {
      return (
        <Estado tipo="error" titulo="Esta pantalla no se pudo cargar" accion={recargar}>
          La aplicación se actualizó o se perdió la conexión mientras estaba abierta. Recarga la página para traer la
          versión actual; la sesión se conserva.
        </Estado>
      )
    }
    const mensaje = error instanceof Error && error.message !== '' ? error.message : null
    if (this.props.alcance === 'aplicacion') {
      return (
        <Estado tipo="error" titulo="La aplicación falló" accion={recargar}>
          Algo se rompió al dibujar la página. Recárgala; la sesión se conserva.
          {mensaje && <span className="text-tenue mt-2 block font-mono text-xs">{mensaje}</span>}
        </Estado>
      )
    }
    return (
      <Estado
        tipo="error"
        titulo="Esta pantalla falló"
        accion={{ etiqueta: 'Reintentar', alHacer: () => this.setState({ fallo: null }) }}
      >
        Algo se rompió al dibujarla. El resto de la aplicación sigue funcionando: puedes reintentar o ir a otra
        pantalla.
        {mensaje && <span className="text-tenue mt-2 block font-mono text-xs">{mensaje}</span>}
      </Estado>
    )
  }
}
