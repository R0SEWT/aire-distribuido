// Las rutas de la app. Una por pantalla (spec §5), cada una en su archivo de
// src/pantallas/ y cargada recién cuando se la visita.
//
// Sin sesión, todo lleva al ingreso (RequiereSesion). Entrenamiento y Clúster
// son de admin (RequiereRol): a un usuario se le dice que no tiene permiso, y
// la API le respondería 403 de todos modos.

import { QueryClientProvider, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { lazy, useEffect, useState } from 'react'
import { BrowserRouter, Route, Routes } from 'react-router'
import { claves, crearClienteDeConsultas } from './api/cliente'
import { olvidarEventosEnVivo } from './api/enVivo'
import { olvidarHistoria } from './api/historia'
import { TiempoRealProveedor } from './api/TiempoRealProveedor'
import { useEvento } from './api/tiempoReal'
import { olvidarVistas } from './api/vistas'
import type { OpcionesWS } from './api/ws'
import { Armazon } from './armazon/Armazon'
import { RUTAS } from './armazon/menu'
import { Barrera } from './componentes/Barrera'
import { entrenamientoALaVista } from './pantallas/entrenamiento/almacen'
import Ingreso from './pantallas/Ingreso'
import { simulador } from './pantallas/simulacion/motor'
import { avanzarEpoca } from './sesion/epoca'
import { RequiereRol, RequiereSesion } from './sesion/guardas'
import { SOLO_ADMIN } from './sesion/roles'
import { useSesion } from './sesion/useSesion'

const Panel = lazy(() => import('./pantallas/Panel'))
const Estacion = lazy(() => import('./pantallas/Estacion'))
const Simulacion = lazy(() => import('./pantallas/Simulacion'))
const Impacto = lazy(() => import('./pantallas/Impacto'))
const Entrenamiento = lazy(() => import('./pantallas/Entrenamiento'))
const Cluster = lazy(() => import('./pantallas/Cluster'))
const NoExiste = lazy(() => import('./pantallas/NoExiste'))
const Muestrario = lazy(() => import('./pantallas/Muestrario'))

export function Rutas() {
  return (
    <Routes>
      <Route path={RUTAS.ingreso} element={<Ingreso />} />
      <Route element={<RequiereSesion />}>
        <Route element={<Armazon />}>
          <Route path={RUTAS.panel} element={<Panel />} />
          <Route path={RUTAS.estacion} element={<Estacion />} />
          <Route path={`${RUTAS.estacion}/:id`} element={<Estacion />} />
          <Route path={RUTAS.simulacion} element={<Simulacion />} />
          <Route path={RUTAS.impacto} element={<Impacto />} />
          <Route
            path={RUTAS.entrenamiento}
            element={
              <RequiereRol roles={SOLO_ADMIN}>
                <Entrenamiento />
              </RequiereRol>
            }
          />
          <Route
            path={RUTAS.cluster}
            element={
              <RequiereRol roles={SOLO_ADMIN}>
                <Cluster />
              </RequiereRol>
            }
          />
          {/* Fuera del menú: los componentes base a la vista, para quien construye una pantalla. */}
          <Route path={RUTAS.muestrario} element={<Muestrario />} />
          <Route path="*" element={<NoExiste />} />
        </Route>
      </Route>
    </Routes>
  )
}

/**
 * Todo lo que una sesión deja en la pestaña, borrado en un solo lugar: lo que
 * vio un admin no lo ve quien entre después. Cada cosa que una pantalla guarde
 * fuera de sus componentes se suma aquí.
 */
function limpiarLoDeLaSesion(consultas: QueryClient) {
  // Antes que nada: la respuesta de un pedido que quedó en viaje ya no es de la sesión vigente.
  avanzarEpoca()
  // Primero lo que sigue haciendo cosas: una simulación en curso se corta (lo ya enviado queda en el sistema).
  simulador.reiniciar()
  // Lo visto: las lecturas enviadas y los estados recibidos (estos, también de sessionStorage).
  olvidarVistas()
  olvidarHistoria(consultas)
  // El entrenamiento que estaba a la vista.
  entrenamientoALaVista.reiniciar()
  // La caché en vivo: los eventos que llegaron por el WebSocket de esta sesión ya no mandan sobre la API.
  olvidarEventosEnVivo(consultas)
  // Y la caché de consultas. La salud es pública y la muestra también el ingreso: se queda.
  consultas.removeQueries({ predicate: (c) => c.queryKey[0] !== claves.salud[0] })
}

/**
 * Limpia lo de la sesión cuando termina, sea por «Salir», porque venció o por
 * un 401: en los tres casos el token deja de estar. Y también al arrancar sin
 * sesión: si venció con la pestaña recargada, lo guardado no espera al próximo.
 */
function LimpiarAlSalir() {
  const { sesion } = useSesion()
  const consultas = useQueryClient()
  const token = sesion?.token
  useEffect(() => {
    if (token === undefined) {
      limpiarLoDeLaSesion(consultas)
      return
    }
    return () => limpiarLoDeLaSesion(consultas)
  }, [token, consultas])
  return null
}

/**
 * Las rondas y el fin de un entrenamiento llegan al que está a la vista
 * (pantallas/entrenamiento/almacen.ts) aunque su pantalla no esté montada: al
 * volver a ella, está al día.
 */
function EntrenamientoEnVivo() {
  useEvento('entrenamiento.ronda', (evento) => entrenamientoALaVista.recibir({ tipo: 'ronda', evento }))
  useEvento('entrenamiento.fin', (evento) => entrenamientoALaVista.recibir({ tipo: 'fin', evento, ahora: Date.now() }))
  return null
}

interface Props {
  /** Para las pruebas: otra caché y otra forma de abrir el WebSocket. */
  consultas?: QueryClient
  tiempoReal?: Partial<OpcionesWS>
}

/** Lo que rodea a las rutas: la caché de consultas y la conexión de tiempo real. */
export function Proveedores({ consultas, tiempoReal, children }: Props & { children: React.ReactNode }) {
  const [propias] = useState(crearClienteDeConsultas)
  return (
    <QueryClientProvider client={consultas ?? propias}>
      <TiempoRealProveedor opciones={tiempoReal}>
        <LimpiarAlSalir />
        <EntrenamientoEnVivo />
        {children}
      </TiempoRealProveedor>
    </QueryClientProvider>
  )
}

export function App() {
  return (
    // La última red: si falla algo fuera de una pantalla (el armazón, el
    // ingreso, un proveedor), se dice y se ofrece recargar en vez de dejar la
    // página en blanco. Los errores de una pantalla los ataja el armazón.
    <Barrera alcance="aplicacion">
      <Proveedores>
        <BrowserRouter>
          <Rutas />
        </BrowserRouter>
      </Proveedores>
    </Barrera>
  )
}
