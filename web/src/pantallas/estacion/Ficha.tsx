import type { ReactNode } from 'react'
import { numero, Tarjeta } from '../../componentes/indice'
import type { Impacto } from '../../api/datos'
import type { Estacion } from '../../api/estaciones'
import { nombrePropio } from '../panel/situacion'
import { fechaDia } from './norma'

function Dato({ nombre, children }: { nombre: string; children: ReactNode }) {
  return (
    <div>
      <dt className="rotulo">{nombre}</dt>
      <dd className="text-texto mt-0.5 text-sm">{children}</dd>
    </div>
  )
}

const SIN_DATO = <span className="text-suave">la fuente no lo dice</span>

/**
 * La ficha de la estación (spec §5.3): qué vigila, dónde está, cuándo midió y
 * cuánta gente vive en su distrito. La población es contexto, no gente
 * expuesta (spec §7.2), y la ficha lo dice.
 */
export function Ficha({ estacion, impacto, midePM }: { estacion: Estacion; impacto: Impacto; midePM: boolean }) {
  const medicion = impacto.estaciones.find((e) => e.estacion === estacion.id)
  const poblacion = impacto.poblacion_distritos.find((p) => p.estaciones.includes(estacion.id))
  // La ficha pone «no lo dice el nombre» cuando el tipo de actividad no se pudo deducir.
  const actividad = estacion.tipo_actividad && estacion.tipo_actividad !== 'no lo dice el nombre' ? estacion.tipo_actividad : null

  return (
    <Tarjeta rotulo="Ficha · objeto fiscalizado" titulo={estacion.objeto_fiscalizado ?? 'La fuente no dice qué vigila'}>
      <dl className="grid grid-cols-2 gap-x-6 gap-y-3">
        <Dato nombre="Tipo de actividad">{actividad ?? SIN_DATO}</Dato>
        <Dato nombre="Estación">
          <span className="font-mono">{estacion.id}</span>
        </Dato>
        <Dato nombre="Distrito">{nombrePropio(estacion.distrito) ?? SIN_DATO}</Dato>
        <Dato nombre="Provincia">{nombrePropio(estacion.provincia) ?? SIN_DATO}</Dato>
        <Dato nombre="Departamento">{nombrePropio(estacion.departamento) ?? SIN_DATO}</Dato>
        <Dato nombre="Coordenadas">
          <span className="font-mono tabular-nums">
            {numero(estacion.lat, 4)}, {numero(estacion.lon, 4)}
          </span>
        </Dato>
        <div className="col-span-2">
          <Dato nombre="Midió PM por hora">
            {medicion ? (
              <>
                Del {fechaDia(medicion.primera_fecha)} al {fechaDia(medicion.ultima_fecha)}:{' '}
                <span className="font-mono tabular-nums">{numero(medicion.dias_con_datos)}</span> días con datos.
              </>
            ) : midePM ? (
              SIN_DATO
            ) : (
              'No mide PM2.5 y PM10 por hora: no hay fechas que dar.'
            )}
          </Dato>
        </div>
        <div className="col-span-2">
          <Dato nombre="Población del distrito">
            {poblacion ? (
              <>
                <span className="font-mono tabular-nums">{numero(poblacion.poblacion)}</span> habitantes en{' '}
                {nombrePropio(poblacion.distrito)} (
                <a href={poblacion.fuente} target="_blank" rel="noreferrer">
                  INEI, censo de {poblacion.anio}
                </a>
                ).
              </>
            ) : (
              SIN_DATO
            )}
          </Dato>
          {poblacion && (
            <p className="text-suave mt-1 text-xs">
              Es contexto, no gente expuesta: la estación vigila una actividad y mide cerca de ella. Vivir en el distrito
              no es haber respirado ese aire.
            </p>
          )}
        </div>
      </dl>
    </Tarjeta>
  )
}
