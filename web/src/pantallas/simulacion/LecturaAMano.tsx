// Una lectura a mano (spec §5.4): PM2.5 y PM10 para una de las 17 estaciones con
// PM por hora, a la hora siguiente a su último estado. No se elige una fecha
// libre: el estado de una estación no retrocede y una fecha lejana lo dejaría
// clavado.

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useId, useState, type FormEvent } from 'react'
import { api, claves, pedir } from '../../api/cliente'
import type { EstadoEstacion } from '../../api/enVivo'
import { presentarError } from '../../api/errores'
import { anotarVista } from '../../api/vistas'
import { Banda, Cifra, Estado, Etiqueta, fechaHora, Frase, numero, Tarjeta } from '../../componentes/indice'
import { horaSiguiente } from './fechas'
import type { Lectura, Prediccion } from './series'

interface Props {
  /** Las estaciones con PM por hora. */
  estaciones: readonly string[]
  /** El último estado de las estaciones que tienen uno; undefined mientras se consulta. */
  estados: readonly EstadoEstacion[] | undefined
  /** Cómo se llama cada estación, si se sabe. */
  descripcionDe: (estacion: string) => string | null
  /** Hay una simulación sin terminar: no se mezclan lecturas a mano con las de un tramo. */
  ocupado: boolean
}

function numeroDe(texto: string): number | null {
  if (texto.trim() === '') return null
  const n = Number(texto.replace(',', '.'))
  return Number.isFinite(n) ? n : null
}

function enLlano(r: Prediccion): string {
  const regimen = `La API asignó la lectura al régimen ${r.regimen}${r.excede_eca ? ', un grupo de lecturas cuya media excede el ECA' : ''}.`
  if (r.media_24h.pm2_5 === null || r.media_24h.pm10 === null) {
    return `${regimen} La ventana de 24 horas tiene ${r.media_24h.lecturas} lecturas: todavía no hay media, ni alerta, ni banda INCA.`
  }
  const medias = `La media de 24 horas es ${numero(r.media_24h.pm2_5, 1)} µg/m³ de PM2.5 y ${numero(r.media_24h.pm10, 1)} µg/m³ de PM10`
  return `${regimen} ${medias}: ${r.alerta ? 'supera el ECA y hay alerta' : 'no supera el ECA'}.`
}

export function LecturaAMano({ estaciones, estados, descripcionDe, ocupado }: Props) {
  const consultas = useQueryClient()
  const id = useId()
  const [estacion, setEstacion] = useState(estaciones[0] ?? '')
  const [pm25, setPm25] = useState('')
  const [pm10, setPm10] = useState('')

  const estado = estados?.find((e) => e.estacion === estacion)
  const fecha = estado ? horaSiguiente(estado.fecha_hora) : null
  const a = numeroDe(pm25)
  const b = numeroDe(pm10)

  const envio = useMutation({
    mutationFn: (lectura: Lectura) => pedir(api.POST('/api/predicciones', { body: lectura })),
    onSuccess: (r, lectura) => {
      anotarVista(lectura.estacion, {
        fecha_hora: lectura.fecha_hora,
        pm2_5: lectura.pm2_5,
        pm10: lectura.pm10,
        media_pm2_5: r.media_24h.pm2_5,
        media_pm10: r.media_24h.pm10,
        lecturas: r.media_24h.lecturas,
        alerta: r.alerta,
        inca: r.inca,
        regimen: r.regimen,
        excede_eca: r.excede_eca,
      })
      // El evento del WebSocket ya actualiza la caché; por si está caído, se vuelve a pedir.
      void consultas.invalidateQueries({ queryKey: claves.regimenes })
    },
  })

  function enviar(e: FormEvent) {
    e.preventDefault()
    if (fecha === null || a === null || b === null || ocupado) return
    envio.mutate({ estacion, fecha_hora: fecha, pm2_5: a, pm10: b })
  }

  const error = envio.isError ? presentarError(envio.error) : null
  const enviada = envio.isSuccess ? envio.variables : null
  return (
    <div className="grid gap-5 lg:grid-cols-[minmax(0,23rem)_minmax(0,1fr)]">
      <Tarjeta rotulo="Lectura a mano" titulo="Una lectura, a la hora siguiente">
        <form className="space-y-4" onSubmit={enviar} aria-label="Lectura a mano">
          <div>
            <label htmlFor={`${id}-estacion`} className="rotulo mb-1 block">
              Estación
            </label>
            <select
              id={`${id}-estacion`}
              className="campo"
              value={estacion}
              onChange={(e) => {
                setEstacion(e.target.value)
                envio.reset()
              }}
            >
              {estaciones.map((e) => (
                <option key={e} value={e}>
                  {e}
                  {descripcionDe(e) ? ` · ${descripcionDe(e)}` : ''}
                </option>
              ))}
            </select>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label htmlFor={`${id}-pm25`} className="rotulo mb-1 block">
                PM2.5 (µg/m³)
              </label>
              <input
                id={`${id}-pm25`}
                className="campo"
                inputMode="decimal"
                placeholder="70.2"
                value={pm25}
                onChange={(e) => setPm25(e.target.value)}
              />
            </div>
            <div>
              <label htmlFor={`${id}-pm10`} className="rotulo mb-1 block">
                PM10 (µg/m³)
              </label>
              <input
                id={`${id}-pm10`}
                className="campo"
                inputMode="decimal"
                placeholder="190.5"
                value={pm10}
                onChange={(e) => setPm10(e.target.value)}
              />
            </div>
          </div>
          {estados === undefined ? (
            <p className="text-suave text-sm">Consultando el último estado de la estación…</p>
          ) : estado && fecha ? (
            <p className="text-suave text-sm">
              Su último estado es del <span className="text-texto font-mono">{fechaHora(estado.fecha_hora)}</span>. La
              lectura se manda con la hora siguiente: <span className="text-texto font-mono">{fechaHora(fecha)}</span>.
            </p>
          ) : (
            <p className="text-aviso text-sm font-semibold">
              {estacion} todavía no tiene estado: no hay una «hora siguiente». Primero simula un día de esa estación o
              carga el escenario.
            </p>
          )}
          {ocupado && <p className="text-aviso text-sm font-semibold">Hay una simulación sin terminar: termínala o detenla antes.</p>}
          <button type="submit" className="boton" disabled={fecha === null || a === null || b === null || ocupado || envio.isPending}>
            {envio.isPending ? 'Enviando…' : 'Enviar la lectura'}
          </button>
          <p className="text-tenue text-xs">
            Escribe el último estado de la estación, que es compartido. Una lectura con valores inventados cambia la media
            de las 24 horas siguientes.
          </p>
        </form>
      </Tarjeta>

      {error ? (
        <Estado tipo="error" titulo={error.titulo}>
          {error.detalle}
        </Estado>
      ) : envio.isSuccess && enviada ? (
        <Tarjeta
          acento={envio.data.alerta ? 'alerta' : 'acento'}
          rotulo={`${enviada.estacion} · lo que respondió la API`}
          titulo={fechaHora(enviada.fecha_hora)}
          aLaDerecha={<span className="font-mono">modelo {envio.data.modelo_id}</span>}
        >
          <div className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-4">
              <Cifra tamano="mediana" rotulo="Régimen" valor={envio.data.regimen} unidad={envio.data.excede_eca ? 'el grupo excede el ECA' : 'el grupo no excede el ECA'} />
              <Cifra tamano="mediana" rotulo="PM2.5 · media de 24 h" valor={envio.data.media_24h.pm2_5} decimales={1} unidad="µg/m³" sinDato="sin media" />
              <Cifra tamano="mediana" rotulo="PM10 · media de 24 h" valor={envio.data.media_24h.pm10} decimales={1} unidad="µg/m³" sinDato="sin media" />
              <Cifra tamano="mediana" rotulo="En la ventana" valor={envio.data.media_24h.lecturas} unidad="lecturas" />
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Banda valor={envio.data.inca} conPrefijo />
              {envio.data.alerta ? <Etiqueta tono="alerta">En alerta</Etiqueta> : <Etiqueta>Sin alerta</Etiqueta>}
            </div>
            <Frase tono={envio.data.alerta ? 'alerta' : 'acento'}>{enLlano(envio.data)}</Frase>
          </div>
        </Tarjeta>
      ) : (
        <Estado tipo="vacio" titulo="Todavía no se envió ninguna lectura a mano">
          Elige una estación y escribe las dos concentraciones. La respuesta de la API aparece aquí: el régimen, la media
          de 24 horas, la alerta y la banda INCA.
        </Estado>
      )}
    </div>
  )
}
