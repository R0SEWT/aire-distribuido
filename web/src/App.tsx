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
import { TiempoRealProveedor } from './api/TiempoRealProveedor'
import type { OpcionesWS } from './api/ws'
import { Armazon } from './armazon/Armazon'
import { RUTAS } from './armazon/menu'
import Ingreso from './pantallas/Ingreso'
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
 * Al terminar una sesión, lo que quedó en la caché se borra: lo que vio un
 * admin no lo ve quien entre después en la misma pestaña.
 */
function LimpiarAlSalir() {
  const { sesion } = useSesion()
  const consultas = useQueryClient()
  const token = sesion?.token
  useEffect(() => {
    return () => {
      // La salud es pública y la muestra también el ingreso: se queda.
      consultas.removeQueries({ predicate: (c) => c.queryKey[0] !== claves.salud[0] })
    }
  }, [token, consultas])
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
        {children}
      </TiempoRealProveedor>
    </QueryClientProvider>
  )
}

export function App() {
  return (
    <Proveedores>
      <BrowserRouter>
        <Rutas />
      </BrowserRouter>
    </Proveedores>
  )
}
