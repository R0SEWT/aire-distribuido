// Simulación y escenario (spec §5.4 y §7.3): lecturas reales de OEFA mandadas
// una por una a POST /api/predicciones, para ver el sistema en marcha.
//
//  - El escenario (admin): 48 horas seguidas de diciembre de 2021 de las
//    estaciones que midieron a la vez.
//  - Un día de una estación: cada serie de /datos/series.json trae sus 48 horas
//    y lo que la API tiene que responder al final.
//  - Una lectura a mano, a la hora siguiente al último estado de la estación.
//
// La corrida vive en simulacion/motor.ts, fuera de React: sigue aunque se
// cambie de pantalla. Aquí se elige qué mandar, se consulta el estado de las
// estaciones antes de empezar y se muestra lo que la API respondió contra lo
// esperado del archivo.

import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { claves } from '../api/cliente'
import { useSeries, type Series } from '../api/datos'
import { useEstaciones, useEstados } from '../api/estaciones'
import { Desplegable, Estado, EstadoDeError, Frase, numero, Pildoras, Tarjeta, type Opcion, type Tono } from '../componentes/indice'
import { SOLO_ADMIN, puede } from '../sesion/roles'
import { useSesion } from '../sesion/useSesion'
import { compararTramo, resumir, type Resumen } from './simulacion/comparar'
import { Botones, ElegirRitmo, EstadoDelSistema, QueMandaElEscenario, QueMandaUnDia, TarjetaDeControles } from './simulacion/Controles'
import { enumerar } from './simulacion/fechas'
import { LecturaAMano } from './simulacion/LecturaAMano'
import { revisarSistema } from './simulacion/limpieza'
import { Marcha } from './simulacion/Marcha'
import { enMarcha, simulador as elDeLaApp, useCorrida, type Corrida, type Simulador } from './simulacion/motor'
import { horasPorSegundoDe, ritmoDe, type Ritmo } from './simulacion/ritmos'
import { estacionesConPM, tramoDelDia, tramoDelEscenario, type Tramo } from './simulacion/series'
import { TablaComparacion } from './simulacion/TablaComparacion'

type Modo = 'escenario' | 'dia' | 'mano'

interface Props {
  /** Para las pruebas: otro simulador (sin esperas) en lugar del de la app. */
  simulador?: Simulador
}

export default function Simulacion({ simulador = elDeLaApp }: Props) {
  const series = useSeries()

  return (
    <div className="space-y-5">
      {series.isPending ? (
        <>
          <Encabezado />
          <Estado tipo="cargando" titulo="Cargando las series…" />
        </>
      ) : series.isError ? (
        <>
          <Encabezado />
          <EstadoDeError error={series.error} alReintentar={() => void series.refetch()} />
        </>
      ) : (
        <Mesa series={series.data} simulador={simulador} />
      )}
    </div>
  )
}

/** El título de la pantalla y, a su derecha, qué se simula. */
function Encabezado({ children }: { children?: ReactNode }) {
  return (
    <header className="space-y-1">
      <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2">
        <h1 className="text-texto text-2xl font-semibold">Simulación</h1>
        {children}
      </div>
      <p className="text-suave">
        Lecturas reales de OEFA mandadas una por una a la API: cómo se llena la ventana de 24 horas y cuándo se enciende
        la alerta.
      </p>
    </header>
  )
}

function Mesa({ series, simulador }: { series: Series; simulador: Simulador }) {
  const { sesion } = useSesion()
  const esAdmin = puede(sesion?.rol, SOLO_ADMIN)
  const consultas = useQueryClient()
  const corrida = useCorrida(simulador)
  const ocupado = enMarcha(corrida.fase)

  const estados = useEstados()
  // Los nombres de las estaciones acompañan al código; si no llegan, queda el código.
  const catalogo = useEstaciones()
  const descripcionDe = useCallback(
    (estacion: string) => {
      const e = catalogo.data?.find((x) => x.id === estacion)
      if (!e) return null
      // La ficha repite a veces el objeto fiscalizado con otro nombre, separado por una barra: basta el primero.
      return [e.objeto_fiscalizado?.split(' | ')[0], e.distrito ? enMinusculas(e.distrito) : null].filter(Boolean).join(', ') || null
    },
    [catalogo.data],
  )

  const tramos = useMemo(
    () => ({ escenario: tramoDelEscenario(series.escenario), dias: new Map(series.dias.map((d) => [d.id, tramoDelDia(d)])) }),
    [series],
  )

  const [modoElegido, setModo] = useState<Modo>(() => corrida.tramo?.tipo ?? (esAdmin ? 'escenario' : 'dia'))
  const [diaId, setDiaId] = useState(() => (corrida.tramo?.tipo === 'dia' ? corrida.tramo.id : (series.dias[0]?.id ?? '')))
  const [ritmo, setRitmo] = useState<Ritmo>(() => (corrida.tramo ? ritmoDe(corrida.horasPorSegundo) : 'normal'))
  const [comprobando, setComprobando] = useState(false)

  // Con una corrida sin terminar, la pantalla muestra esa corrida y no deja elegir otra cosa.
  const modo: Modo = ocupado && corrida.tramo ? corrida.tramo.tipo : modoElegido === 'escenario' && !esAdmin ? 'dia' : modoElegido
  const dia = series.dias.find((d) => d.id === (ocupado && corrida.tramo?.tipo === 'dia' ? corrida.tramo.id : diaId))
  const tramo: Tramo | undefined =
    ocupado && corrida.tramo ? corrida.tramo : modo === 'escenario' ? tramos.escenario : dia ? tramos.dias.get(dia.id) : undefined
  const deEsteTramo = tramo !== undefined && corrida.tramo?.id === tramo.id
  const visible: Corrida | null = deEsteTramo ? corrida : null

  const revision = useMemo(() => (tramo && estados.data ? revisarSistema(tramo, estados.data) : null), [tramo, estados.data])
  const comparaciones = useMemo(() => (tramo && visible ? compararTramo(tramo, visible.porEstacion) : []), [tramo, visible])

  // Al terminar o detenerse, el estado de las estaciones se vuelve a pedir: el
  // WebSocket ya lo trae, pero puede estar caído.
  const fase = corrida.fase
  useEffect(() => {
    if (fase === 'terminada' || fase === 'detenida') void consultas.invalidateQueries({ queryKey: claves.regimenes })
  }, [fase, consultas])

  async function empezar() {
    if (!tramo || ocupado || comprobando) return
    // Lo que ya estaba a la vista: si el aviso ya se mostró, empezar es «de todos modos».
    const yaAvisado = estados.isError || (revision !== null && !revision.limpio)
    setComprobando(true)
    const fresca = await estados.refetch()
    setComprobando(false)
    if (!yaAvisado && (fresca.isError || !revisarSistema(tramo, fresca.data ?? []).limpio)) return
    simulador.iniciar(tramo, horasPorSegundoDe(ritmo))
  }

  const modos: Opcion<Modo>[] = [
    ...(esAdmin ? [{ valor: 'escenario' as const, etiqueta: 'El escenario', deshabilitada: ocupado }] : []),
    { valor: 'dia', etiqueta: 'Un día de una estación', deshabilitada: ocupado },
    { valor: 'mano', etiqueta: 'Una lectura a mano', deshabilitada: ocupado },
  ]

  return (
    <>
      <Encabezado>
        <div className="flex flex-wrap items-center gap-3">
          {!esAdmin && <p className="text-tenue text-sm">Cargar el escenario es de quien administra.</p>}
          <Pildoras etiqueta="Qué se simula" opciones={modos} valor={modo} alCambiar={setModo} />
        </div>
      </Encabezado>

      {modo === 'mano' ? (
        <LecturaAMano estaciones={estacionesConPM(series)} estados={estados.data} descripcionDe={descripcionDe} ocupado={ocupado} />
      ) : !tramo ? (
        <Estado tipo="vacio" titulo="El archivo de series no trae ningún día para simular" />
      ) : (
        <>
          <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,23rem)_minmax(0,1fr)]">
            <TarjetaDeControles tipo={tramo.tipo}>
              {tramo.tipo === 'escenario' ? (
                <QueMandaElEscenario escenario={series.escenario} />
              ) : (
                <QueMandaUnDia dias={series.dias} elegido={dia} alElegir={setDiaId} bloqueado={ocupado} descripcionDe={descripcionDe} />
              )}
              {ocupado ? (
                <p className="text-suave text-sm leading-relaxed">
                  La corrida sigue aunque cambies de pantalla: puedes ir al panel a ver cómo se llena y volver.
                </p>
              ) : (
                <div>
                  <p className="rotulo mb-1.5">Antes de simular</p>
                  <EstadoDelSistema
                    tramo={tramo}
                    consulta={{ cargando: estados.isPending, error: estados.isError ? estados.error : null, alReintentar: () => void estados.refetch() }}
                    revision={revision}
                    otrosDias={tramo.tipo === 'dia' && estados.data ? { dias: series.dias, estados: estados.data, alElegir: setDiaId } : undefined}
                  />
                </div>
              )}
              <Botones
                fase={corrida.fase}
                deEsteTramo={deEsteTramo}
                tipo={tramo.tipo}
                revision={revision}
                sinComprobar={estados.isError}
                comprobando={comprobando}
                puedeEmpezar={tramo.lecturas.length > 0}
                alEmpezar={() => void empezar()}
                alPausar={() => simulador.pausar()}
                alContinuar={() => simulador.continuar()}
                alDetener={() => simulador.detener()}
              />
              <div className="border-borde space-y-4 border-t pt-4">
                <ElegirRitmo
                  valor={ocupado ? ritmoDe(corrida.horasPorSegundo) : ritmo}
                  alCambiar={(r) => {
                    setRitmo(r)
                    simulador.cambiarRitmo(horasPorSegundoDe(r))
                  }}
                  horas={series.escenario.horas}
                />
                <p className="text-suave text-sm leading-relaxed">
                  <strong className="text-texto">Simular escribe estado compartido:</strong> el último estado de cada
                  estación cambia para todos los que tengan la app abierta.
                </p>
              </div>
            </TarjetaDeControles>

            <Marcha key={tramo.id} tramo={tramo} corrida={visible} reglas={series.metadatos.reglas} />
          </div>

          {visible && (
            <Tarjeta rotulo="Evaluación" titulo="Lo que respondió la API contra lo esperado del archivo">
              <div className="space-y-4">
                <Veredicto resumen={resumir(comparaciones)} corrida={visible} />
                <TablaComparacion comparaciones={comparaciones} />
                <Desplegable titulo="¿De dónde sale lo esperado?">
                  <p>{series.metadatos.como_se_usa.esperado}</p>
                  <p>{series.metadatos.como_se_usa.regimen}</p>
                  <p>
                    Regla de la alerta: {series.metadatos.reglas.alerta}. Banda INCA: {series.metadatos.reglas.inca.regla}.
                    El archivo lo escribe <code className="font-mono">{series.metadatos.generador}</code> ({series.metadatos.generado}).
                  </p>
                </Desplegable>
              </div>
            </Tarjeta>
          )}
        </>
      )}
    </>
  )
}

function enMinusculas(texto: string): string {
  return texto.toLowerCase().replace(/(^|\s)\p{L}/gu, (l) => l.toUpperCase())
}

/** En una frase: cuántas estaciones quedaron como dice el archivo. */
function Veredicto({ resumen, corrida }: { resumen: Resumen; corrida: Corrida }) {
  const n = resumen.estaciones
  const de = (k: number) => `${k} de ${n} ${n === 1 ? 'estación' : 'estaciones'}`
  if (resumen.completas === 0) {
    return <Frase tono="neutro">Todavía ninguna estación recibió todas sus lecturas: la comparación se hace con la última de cada una.</Frase>
  }
  const alertas =
    resumen.enAlerta.length === 0
      ? 'Ninguna terminó en alerta.'
      : `${resumen.enAlerta.length === 1 ? 'Terminó en alerta' : 'Terminaron en alerta'} ${enumerar(resumen.enAlerta)}.`
  const tono: Tono = resumen.difieren > 0 ? 'alerta' : resumen.soloRegimen > 0 ? 'moderada' : 'buena'
  const partes = [
    resumen.coinciden === 0
      ? n === 1
        ? 'La estación no quedó como dice el archivo.'
        : `Ninguna de las ${n} estaciones quedó como dice el archivo.`
      : `${de(resumen.coinciden)} ${resumen.coinciden === 1 ? 'quedó' : 'quedaron'} como dice el archivo: la misma media de 24 horas, la misma alerta, la misma banda INCA y el mismo régimen.`,
    resumen.soloRegimen > 0
      ? `En ${resumen.soloRegimen} solo difiere el régimen: el esperado vale para el modelo sembrado y el activo es ${corrida.modeloId ?? 'otro'}.`
      : null,
    resumen.difieren > 0 ? `${resumen.difieren === 1 ? 'Difiere 1' : `Difieren ${resumen.difieren}`}: mira la tabla.` : null,
    resumen.completas < n ? `Faltan lecturas en ${n - resumen.completas}.` : null,
    resumen.completas === n ? alertas : null,
  ].filter((p): p is string => p !== null)
  return (
    <div className="space-y-2">
      <Frase tono={tono}>{partes.join(' ')}</Frase>
      {corrida.fase === 'terminada' && (
        <p className="text-tenue text-sm">
          {numero(corrida.enviadas)} lecturas en {numero(corrida.duracionMs / 1000, 1)} segundos, con el modelo{' '}
          <span className="font-mono">{corrida.modeloId}</span>.
        </p>
      )}
    </div>
  )
}
