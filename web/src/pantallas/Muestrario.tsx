// El muestrario de los componentes base (spec §8), en /muestrario. No está en
// el menú: es para quien construye una pantalla, que ve aquí cómo queda cada
// componente en modo normal y en modo proyector. Los números son los de los
// ejemplos del contrato, no datos de ninguna estación.

import { useState } from 'react'
import { errorDeRespuesta } from '../api/errores'
import {
  Banda,
  Cifra,
  Desplegable,
  Estado,
  EstadoDeError,
  Etiqueta,
  Frase,
  numero,
  Pildoras,
  Tabla,
  Tarjeta,
  type BandaINCA,
  type Columna,
  type EstadoBanda,
} from '../componentes/indice'

interface Fila {
  estacion: string
  distrito: string
  pm2_5: number | null
  pm10: number | null
  inca: BandaINCA | EstadoBanda
  alerta: boolean
}

const FILAS: Fila[] = [
  { estacion: 'CA-AYAS-01', distrito: 'San Marcos', pm2_5: 58.3, pm10: 121.4, inca: 'mala', alerta: true },
  { estacion: 'CA-EJ-02', distrito: 'Ejemplo', pm2_5: 18.2, pm10: 96.0, inca: 'moderada', alerta: false },
  { estacion: 'CA-EJ-03', distrito: 'Ejemplo', pm2_5: 9.0, pm10: 17.6, inca: 'buena', alerta: false },
  { estacion: 'CA-MAC-01', distrito: 'Ejemplo', pm2_5: null, pm10: null, inca: null, alerta: false },
  { estacion: 'CA-EJ-05', distrito: 'Ejemplo', pm2_5: null, pm10: null, inca: 'no_mide', alerta: false },
]

const COLUMNAS: Columna<Fila>[] = [
  { clave: 'estacion', titulo: 'Estación', celda: (f) => <span className="font-mono">{f.estacion}</span> },
  { clave: 'distrito', titulo: 'Distrito', celda: (f) => f.distrito },
  { clave: 'pm2_5', titulo: 'PM2.5 · 24 h', numerica: true, celda: (f) => numero(f.pm2_5, 1) },
  { clave: 'pm10', titulo: 'PM10 · 24 h', numerica: true, celda: (f) => numero(f.pm10, 1) },
  { clave: 'inca', titulo: 'Banda INCA', celda: (f) => <Banda valor={f.inca} /> },
  { clave: 'alerta', titulo: 'Alerta', celda: (f) => (f.alerta ? <Etiqueta tono="alerta">En alerta</Etiqueta> : '—') },
]

export default function Muestrario() {
  const [contaminante, setContaminante] = useState<'pm2_5' | 'pm10'>('pm10')
  return (
    <div className="space-y-5">
      <header>
        <h1 className="text-texto text-2xl font-semibold">Muestrario de componentes</h1>
        <p className="text-suave mt-1">
          Los componentes base de <code className="font-mono">src/componentes/</code>, con los números de los ejemplos
          del contrato. No son datos de ninguna estación.
        </p>
      </header>

      <div className="grid gap-5 lg:grid-cols-[minmax(0,26rem)_minmax(0,1fr)]">
        <Tarjeta acento="mala" rotulo="CA-AYAS-01 · media de 24 horas" titulo="Tarjeta con borde de acento">
          <div className="space-y-4">
            <Pildoras
              etiqueta="Contaminante"
              valor={contaminante}
              alCambiar={setContaminante}
              opciones={[
                { valor: 'pm2_5', etiqueta: 'PM2.5' },
                { valor: 'pm10', etiqueta: 'PM10' },
              ]}
            />
            <Cifra valor={contaminante === 'pm10' ? 121.4 : 58.3} decimales={1} unidad="µg/m³" />
            <div className="flex flex-wrap items-center gap-2">
              <Banda valor="mala" conPrefijo />
              <Etiqueta tono="alerta">En alerta</Etiqueta>
              <Etiqueta>24 lecturas</Etiqueta>
            </div>
            <div className="border-borde grid grid-cols-3 gap-3 border-t pt-4">
              <Cifra tamano="chica" rotulo="PM2.5" valor={58.3} decimales={1} unidad="µg/m³" />
              <Cifra tamano="chica" rotulo="PM10" valor={121.4} decimales={1} unidad="µg/m³" />
              <Cifra tamano="chica" rotulo="Régimen" valor={null} sinDato="sin dato" />
            </div>
            <Frase tono="mala">
              {contaminante === 'pm10'
                ? 'La media de las últimas 24 horas de PM10 es 121 µg/m³: supera el ECA (100).'
                : 'La media de las últimas 24 horas de PM2.5 es 58 µg/m³: supera el ECA (50).'}
            </Frase>
            <Desplegable titulo="¿De dónde sale este número?">
              <p>
                Es la media de las lecturas horarias de la estación en las últimas 24 horas. Hay alerta cuando supera
                el ECA (DS 003-2017-MINAM): 50 µg/m³ de PM2.5 o 100 µg/m³ de PM10.
              </p>
            </Desplegable>
          </div>
        </Tarjeta>

        <div className="space-y-5">
          <Tarjeta titulo="Cifras" aLaDerecha="Cifra · mediana">
            <div className="grid gap-4 sm:grid-cols-3">
              <Cifra tamano="mediana" rotulo="Estaciones en alerta" valor={2} unidad="de 17" />
              <Cifra tamano="mediana" rotulo="Horas en el gold" valor={65873} unidad="horas" />
              <Cifra tamano="mediana" rotulo="Media de 24 h" valor={null} unidad="µg/m³" sinDato="sin datos suficientes" />
            </div>
          </Tarjeta>

          <Tarjeta titulo="Bandas INCA y los dos estados sin banda">
            <div className="flex flex-wrap gap-2">
              <Banda valor="buena" />
              <Banda valor="moderada" />
              <Banda valor="mala" />
              <Banda valor="umbral_de_cuidado" />
              <Banda valor={null} />
              <Banda valor="no_mide" />
            </div>
            <div className="mt-4 grid gap-3 sm:grid-cols-2">
              <Frase tono="buena">Frase en tono «buena»: lo que el número significa, comparado con qué.</Frase>
              <Frase tono="moderada">Frase en tono «moderada».</Frase>
              <Frase tono="alerta">Frase en tono «alerta».</Frase>
              <Frase>Frase en tono «acento», el de por omisión.</Frase>
            </div>
          </Tarjeta>
        </div>
      </div>

      <Tarjeta titulo="Tabla">
        <Tabla titulo="Estaciones de ejemplo" columnas={COLUMNAS} filas={FILAS} claveDe={(f) => f.estacion} />
      </Tarjeta>

      <div className="grid gap-5 md:grid-cols-2 xl:grid-cols-4">
        <Estado tipo="cargando" />
        <Estado tipo="vacio" titulo="Ninguna estación tiene estado" accion={{ etiqueta: 'Cargar el escenario', alHacer: () => {} }}>
          El sistema recién se levantó y todavía no recibió lecturas.
        </Estado>
        <EstadoDeError error={errorDeRespuesta(503, { error: 'MongoDB no disponible' }, true)} alReintentar={() => {}} />
        <EstadoDeError error={errorDeRespuesta(403, { error: 'se requiere el rol admin' }, true)} />
      </div>
    </div>
  )
}
