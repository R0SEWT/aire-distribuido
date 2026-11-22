// La pantalla de ingreso (spec §5.1): usuario y clave contra POST /api/auth/token.

import { useRef, useState, type FormEvent } from 'react'
import { Navigate, useLocation } from 'react-router'
import { ErrorApi, errorDeExcepcion, presentarError } from '../api/errores'
import { BotonProyector } from '../armazon/BotonProyector'
import { EstadoApi } from '../armazon/Indicadores'
import { Marca } from '../armazon/Marca'
import { puedeEntrar, RUTAS } from '../armazon/menu'
import { Pie } from '../armazon/Pie'
import { useTitulo } from '../armazon/titulo'
import { Aviso } from '../componentes/Aviso'
import type { DesdeDonde } from '../sesion/guardas'
import { useSesion } from '../sesion/useSesion'

export default function Ingreso() {
  const { sesion, motivo, ingresar } = useSesion()
  const ubicacion = useLocation()
  const [usuario, setUsuario] = useState('')
  const [clave, setClave] = useState('')
  const [enviando, setEnviando] = useState(false)
  const [error, setError] = useState<ErrorApi | null>(null)
  const campoClave = useRef<HTMLInputElement>(null)
  useTitulo('Ingreso')

  if (sesion) {
    // Ya hay sesión: a donde iba cuando la guarda lo mandó aquí, si su rol
    // puede verlo; si no, al panel.
    const desde = (ubicacion.state as DesdeDonde | null)?.desde
    const vuelve = desde !== undefined && desde !== RUTAS.ingreso && puedeEntrar(desde, sesion.rol)
    return <Navigate to={vuelve ? desde : RUTAS.panel} replace />
  }

  async function enviar(e?: FormEvent) {
    e?.preventDefault()
    if (enviando) return
    setEnviando(true)
    setError(null)
    try {
      await ingresar(usuario.trim(), clave)
    } catch (fallo) {
      const traducido = errorDeExcepcion(fallo)
      setError(traducido)
      if (traducido.tipo === 'credenciales') {
        setClave('')
        campoClave.current?.focus()
      }
    } finally {
      setEnviando(false)
    }
  }

  return (
    <div className="flex min-h-dvh flex-col">
      <header className="border-borde border-b">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center gap-x-4 gap-y-2 px-6 py-3">
          <Marca />
          <EstadoApi />
          <div className="ml-auto">
            <BotonProyector />
          </div>
        </div>
      </header>

      <main className="mx-auto flex w-full max-w-md flex-1 flex-col justify-center px-6 py-10">
        <h1 className="text-texto text-2xl font-semibold">¿Cómo está el aire cerca de cada estación?</h1>
        <p className="text-suave mt-2 leading-relaxed">
          El último estado recibido de las estaciones de OEFA, las alertas por la media de 24 horas y el entrenamiento
          del modelo en el clúster.
        </p>

        {motivo === 'vencida' && !error && (
          <Aviso titulo="La sesión venció." className="mt-5">
            Vuelve a ingresar para seguir.
          </Aviso>
        )}

        <form
          onSubmit={enviar}
          className="border-borde bg-superficie border-l-acento mt-5 space-y-4 rounded-xl border border-l-4 p-5"
          aria-label="Ingreso"
        >
          <div>
            <label htmlFor="usuario" className="rotulo mb-1.5 block">
              Usuario
            </label>
            <input
              id="usuario"
              name="usuario"
              className="campo"
              autoComplete="username"
              autoCapitalize="none"
              spellCheck={false}
              autoFocus
              required
              value={usuario}
              onChange={(e) => setUsuario(e.target.value)}
            />
          </div>
          <div>
            <label htmlFor="clave" className="rotulo mb-1.5 block">
              Clave
            </label>
            <input
              id="clave"
              name="clave"
              type="password"
              className="campo"
              autoComplete="current-password"
              required
              ref={campoClave}
              value={clave}
              onChange={(e) => setClave(e.target.value)}
            />
          </div>

          {error && <ErrorDeIngreso error={error} alReintentar={() => void enviar()} enviando={enviando} />}

          <button type="submit" className="boton w-full" disabled={enviando}>
            {enviando ? 'Ingresando…' : 'Ingresar'}
          </button>
        </form>

        <p className="text-tenue mt-4 text-sm">
          Las cuentas las configura quien levanta el sistema. La sesión dura lo que dure el token y se cierra al cerrar
          la pestaña.
        </p>
      </main>

      <Pie />
    </div>
  )
}

function ErrorDeIngreso({
  error,
  alReintentar,
  enviando,
}: {
  error: ErrorApi
  alReintentar: () => void
  enviando: boolean
}) {
  // Usuario o clave incorrectos: se dice eso, sin más (ni cuál de los dos).
  if (error.tipo === 'credenciales') {
    return <Aviso tono="alerta" titulo="Usuario o clave incorrectos." />
  }
  const p = presentarError(error)
  return (
    <Aviso
      tono="alerta"
      titulo={`${p.titulo}.`}
      accion={p.reintentable ? { etiqueta: 'Reintentar', alHacer: alReintentar, deshabilitada: enviando } : undefined}
    >
      {p.detalle}
    </Aviso>
  )
}
