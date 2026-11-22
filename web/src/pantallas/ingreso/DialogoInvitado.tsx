// El diálogo del ingreso de invitado. La pantalla de ingreso no tiene registro
// (las cuentas las configura quien levanta el sistema): quien aprieta «No pienso
// crearme una cuenta aquí» se encuentra con este diálogo, que igual lo deja
// pasar con la cuenta de invitado del servidor (POST /api/auth/invitado). Las
// credenciales de esa cuenta no pasan por aquí: la API responde el token.
//
// Es un <dialog> abierto con showModal(): el navegador lo pone encima de todo,
// deja inerte lo de atrás y lo cierra con Escape. Aquí se suma lo que no trae:
// el foco inicial en «Entrar igual», cerrar con un clic fuera y que Tab dé la
// vuelta dentro del diálogo (el navegador, después del último control, saca el
// foco a su propia barra). El foco de vuelta al botón que lo abrió lo pone la
// pantalla.

import { useEffect, useId, useRef, useState, type KeyboardEvent, type MouseEvent } from 'react'
import { ErrorApi, errorDeExcepcion, esCancelacion, presentarError } from '../../api/errores'
import { Aviso } from '../../componentes/Aviso'
import { useSesion } from '../../sesion/useSesion'
import gato from './gato.webp'

/** Lo que recibe el foco con Tab dentro del diálogo. */
const ENFOCABLES = 'button:not(:disabled), a[href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled)'

/** El tamaño de gato.webp: declarado en la imagen para que el diálogo no salte al cargarla. */
const ANCHO_GATO = 635
const ALTO_GATO = 356

export function DialogoInvitado({ alCerrar }: { alCerrar: () => void }) {
  const { entrarComoInvitado } = useSesion()
  const dialogo = useRef<HTMLDialogElement>(null)
  const botonEntrar = useRef<HTMLButtonElement>(null)
  const botonCerrar = useRef<HTMLButtonElement>(null)
  const pedido = useRef<AbortController | null>(null)
  const apretoFuera = useRef(false)
  const [entrando, setEntrando] = useState(false)
  const [error, setError] = useState<ErrorApi | null>(null)
  const idTitulo = useId()
  // La API no tiene cuenta de invitado: pedirlo otra vez no cambia nada.
  const sinInvitado = error?.tipo === 'no_encontrado'

  useEffect(() => {
    const d = dialogo.current
    // Sin close() al desmontar: en desarrollo (StrictMode) el efecto corre dos
    // veces, y cerrar entre una y otra avisaría un cierre que nadie pidió.
    if (d && !d.open) d.showModal()
    botonEntrar.current?.focus()
  }, [])

  // Tras un error el foco va a lo que sigue: reintentar o, si no hay invitado
  // («Entrar igual» ya no está), cerrar.
  useEffect(() => {
    if (!error) return
    const destino = error.tipo === 'no_encontrado' ? botonCerrar : botonEntrar
    destino.current?.focus()
  }, [error])

  function cerrar() {
    dialogo.current?.close()
  }

  /** El diálogo se cerró, por el botón, por Escape o por un clic fuera. */
  function alCerrarse() {
    // Si había un pedido en viaje, su respuesta ya no abre la sesión.
    pedido.current?.abort()
    alCerrar()
  }

  async function entrar() {
    if (entrando) return
    const control = new AbortController()
    pedido.current = control
    setEntrando(true)
    setError(null)
    try {
      await entrarComoInvitado(control.signal)
      // Hay sesión: la pantalla de ingreso lleva al panel y este diálogo se va con ella.
    } catch (fallo) {
      if (control.signal.aborted || esCancelacion(fallo)) return
      setError(errorDeExcepcion(fallo))
      setEntrando(false)
    }
  }

  /** Tab después del último control vuelve al primero, y Mayús+Tab antes del primero, al último. */
  function darLaVuelta(e: KeyboardEvent<HTMLDialogElement>) {
    if (e.key !== 'Tab') return
    const controles = [...e.currentTarget.querySelectorAll<HTMLElement>(ENFOCABLES)]
    const primero = controles[0]
    const ultimo = controles[controles.length - 1]
    if (!primero || !ultimo) return
    const borde = e.shiftKey ? primero : ultimo
    if (document.activeElement !== borde) return
    e.preventDefault()
    ;(e.shiftKey ? ultimo : primero).focus()
  }

  // Un clic «fuera» cae en el propio <dialog> (su telón es parte de él); uno
  // dentro, en el contenido. Tiene que empezar y terminar fuera: soltar fuera
  // una selección de texto que empezó dentro no lo cierra.
  const esFuera = (e: MouseEvent<HTMLDialogElement>) => e.target === e.currentTarget

  return (
    <dialog
      ref={dialogo}
      aria-modal="true"
      aria-labelledby={idTitulo}
      className="dialogo-vidrio m-auto w-[calc(100%-2rem)] max-w-md p-0"
      onClose={alCerrarse}
      onKeyDown={darLaVuelta}
      onMouseDown={(e) => {
        apretoFuera.current = esFuera(e)
      }}
      onClick={(e) => {
        if (esFuera(e) && apretoFuera.current) cerrar()
      }}
    >
      <div className="space-y-5 p-6">
        <h2 id={idTitulo} className="dialogo-vidrio-titulo">
          Perdón, era parte de la nota
        </h2>
        <img
          src={gato}
          width={ANCHO_GATO}
          height={ALTO_GATO}
          alt="Un gato blanco con los ojos llenos de lágrimas, en una foto borrosa."
          className="dialogo-vidrio-foto"
        />

        {error && <ErrorDeInvitado error={error} />}

        <div className="flex flex-wrap items-center gap-2">
          {!sinInvitado && (
            // aria-disabled y no disabled mientras pide: un botón deshabilitado suelta el foco,
            // y dentro de un diálogo modal no hay a dónde vaya. entrar() ya ignora el segundo clic.
            <button
              type="button"
              className="dialogo-vidrio-boton dialogo-vidrio-boton-principal aria-disabled:cursor-not-allowed aria-disabled:opacity-50"
              ref={botonEntrar}
              onClick={() => void entrar()}
              aria-disabled={entrando}
            >
              {entrando ? 'Entrando…' : 'Entrar igual'}
            </button>
          )}
          <button type="button" className="dialogo-vidrio-boton" ref={botonCerrar} onClick={cerrar}>
            Cerrar
          </button>
        </div>
      </div>
    </dialog>
  )
}

function ErrorDeInvitado({ error }: { error: ErrorApi }) {
  // 404: quien levantó la API no le puso cuenta de invitado (API_INVITADO).
  if (error.tipo === 'no_encontrado') {
    return (
      <Aviso tono="alerta" titulo="Este sistema no tiene cuenta de invitado.">
        Pídele una a quien lo levantó.
      </Aviso>
    )
  }
  const p = presentarError(error)
  return (
    <Aviso tono="alerta" titulo={`${p.titulo}.`}>
      {p.detalle}
    </Aviso>
  )
}
