// Impacto, segunda mitad: si la alerta sirve, con cuánta anticipación, y lo que
// este dato no puede decir (spec §5.7, §7.2 y §7.4). Todo sale de impacto.json.

import { Cifra, Desplegable, enumerar, Frase, nombrePropio, numero, proporcion, Tabla, type Columna } from '../../componentes/indice'
import { AnticipacionEpisodios, type MedidaDeAnticipacion } from '../../graficos/AnticipacionEpisodios'
import { SenalesAlerta } from '../../graficos/SenalesAlerta'
import { DeDondeSale, Seccion } from './apoyo'
import { DIAS_DE_CAMPANA_CORTA, enHoras, enLetras, horas } from './textos'
import type { Cuartiles, Impacto } from '../../api/datos'

export function SirveLaAlerta({ impacto }: { impacto: Impacto }) {
  const s = impacto.senales
  const r = impacto.metadatos.reglas
  const senales = [
    {
      nombre: `Régimen alto (régimen ${s.regimen_alto.regimen})`,
      precision: s.regimen_alto.precision,
      cobertura: s.regimen_alto.cobertura,
      detallePrecision: `${numero(s.regimen_alto.aciertos)} de ${numero(s.regimen_alto.horas)} horas`,
      detalleCobertura: `${numero(s.regimen_alto.aciertos)} de ${numero(s.horas_sobre_eca)} horas`,
    },
    {
      nombre: 'Lectura horaria sobre el umbral',
      precision: s.lectura_horaria.precision,
      cobertura: s.lectura_horaria.cobertura,
      detallePrecision: `${numero(s.lectura_horaria.aciertos)} de ${numero(s.lectura_horaria.horas)} horas`,
      detalleCobertura: `${numero(s.lectura_horaria.aciertos)} de ${numero(s.horas_sobre_eca)} horas`,
    },
  ]
  return (
    <Seccion id="sirve-la-alerta" pregunta="¿Sirve la alerta?" titulo="La alerta del sistema es la norma misma; las señales hora a hora aciertan menos">
      <p className="text-texto leading-relaxed">
        La alerta del sistema sale de la media de 24 horas: es la norma misma, no una predicción, y por eso no falla ni se
        adelanta. Hay dos señales que se encienden hora a hora: que la estación entre al{' '}
        <strong>régimen alto</strong> del modelo, y que la <strong>lectura de la hora</strong> pase de {numero(r.eca_pm2_5)}{' '}
        µg/m³ de PM2.5 o de {numero(r.eca_pm10)} de PM10. Esto es lo que acertarían si se usaran como alerta.
      </p>
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,24rem)] lg:items-start">
        <SenalesAlerta senales={senales} />
        <div className="grid grid-cols-2 gap-x-6 gap-y-4">
          <Cifra tamano="mediana" rotulo="Régimen alto · precisión" valor={s.regimen_alto.precision * 100} decimales={1} unidad="%" />
          <Cifra tamano="mediana" rotulo="Régimen alto · cobertura" valor={s.regimen_alto.cobertura * 100} decimales={1} unidad="%" />
          <Cifra tamano="mediana" rotulo="Lectura horaria · precisión" valor={s.lectura_horaria.precision * 100} decimales={1} unidad="%" />
          <Cifra tamano="mediana" rotulo="Lectura horaria · cobertura" valor={s.lectura_horaria.cobertura * 100} decimales={1} unidad="%" />
        </div>
      </div>
      <Frase tono="moderada">
        El régimen alto acierta el {proporcion(s.regimen_alto.precision, 0)} de las veces que suena (precisión) y detecta el{' '}
        {proporcion(s.regimen_alto.cobertura, 0)} de las horas sobre el ECA (cobertura). La lectura horaria detecta más (
        {proporcion(s.lectura_horaria.cobertura, 0)}) y acierta menos ({proporcion(s.lectura_horaria.precision, 0)}): por
        eso el régimen se muestra como dato y la alerta es la de la media de 24 horas.
      </Frase>
      <DeDondeSale impacto={impacto} fuentes={['gold', 'asignaciones', 'regimenes']}>
        <p>{s.nota}</p>
        <p>
          Régimen alto: encendido en {numero(s.regimen_alto.horas)} horas, {numero(s.regimen_alto.aciertos)} de ellas con
          la media sobre el ECA. Lectura horaria: {numero(s.lectura_horaria.horas)} horas y{' '}
          {numero(s.lectura_horaria.aciertos)} aciertos. Horas sobre el ECA: {numero(s.horas_sobre_eca)} de{' '}
          {numero(s.horas_calculables)} con media calculable.
        </p>
      </DeDondeSale>
    </Seccion>
  )
}

function Rango({ q }: { q: Cuartiles | null }) {
  if (!q) return <span className="text-tenue">—</span>
  return (
    <span className="inline-flex flex-col items-end leading-tight whitespace-nowrap">
      <span className="text-texto font-semibold">{horas(q.mediana)} h</span>
      <span className="text-tenue font-sans text-xs">
        {horas(q.minimo)} a {horas(q.maximo)}
      </span>
    </span>
  )
}

type FilaPorEstacion = Impacto['anticipacion']['por_estacion'][number]

const COLUMNAS_POR_ESTACION: Columna<FilaPorEstacion>[] = [
  { clave: 'estacion', titulo: 'Estación', celda: (f) => <span className="font-mono font-semibold">{f.estacion}</span> },
  { clave: 'episodios', titulo: 'Episodios', numerica: true, celda: (f) => numero(f.episodios) },
  { clave: 'lh_antes', titulo: 'Lectura horaria · desde la primera vez', numerica: true, celda: (f) => <Rango q={f.lectura_horaria.horas_antes} /> },
  { clave: 'lh_seguidas', titulo: 'Lectura horaria · seguidas', numerica: true, celda: (f) => <Rango q={f.lectura_horaria.horas_seguidas} /> },
  { clave: 'reg_antes', titulo: 'Régimen · desde la primera vez', numerica: true, celda: (f) => <Rango q={f.regimen.horas_antes} /> },
  { clave: 'reg_seguidas', titulo: 'Régimen · seguidas', numerica: true, celda: (f) => <Rango q={f.regimen.horas_seguidas} /> },
]

/** El texto del archivo nombra sus campos (horas_antes, horas_seguidas): se escriben como código. */
function conCodigo(texto: string) {
  return texto.split(/\b([a-z]+_[a-z_]+)\b/).map((trozo, i) =>
    i % 2 === 1 ? (
      <code key={i} className="font-mono">
        {trozo}
      </code>
    ) : (
      trozo
    ),
  )
}

export function ConCuantaAnticipacion({ impacto }: { impacto: Impacto }) {
  const a = impacto.anticipacion
  const t = a.total
  const tope = Number(a.definicion.tope_horas) || 23
  const lh = t.lectura_horaria
  const reg = t.regimen
  const conEpisodios = a.por_estacion.filter((f) => f.episodios > 0)
  const sinEpisodios = a.por_estacion.length - conEpisodios.length

  const medidas: MedidaDeAnticipacion[] = [
    {
      titulo: 'Desde la primera vez que la señal se enciende, en las 24 horas previas',
      marcaElTope: true,
      filas: [
        { senal: 'Lectura horaria', horas: a.episodios.map((e) => e.horas_antes_lectura_horaria), cuartiles: lh.horas_antes },
        { senal: 'Régimen alto', horas: a.episodios.flatMap((e) => (e.horas_antes_regimen === null ? [] : [e.horas_antes_regimen])), cuartiles: reg.horas_antes },
      ],
    },
    {
      titulo: 'Horas seguidas encendida, sin interrupción, justo antes del cruce',
      marcaElTope: false,
      filas: [
        { senal: 'Lectura horaria', horas: a.episodios.map((e) => e.horas_seguidas_lectura_horaria), cuartiles: lh.horas_seguidas },
        { senal: 'Régimen alto', horas: a.episodios.map((e) => e.horas_seguidas_regimen), cuartiles: reg.horas_seguidas },
      ],
    },
  ]

  return (
    <Seccion id="anticipacion" pregunta="¿Con cuánta anticipación?" titulo="Depende de cómo se cuente: las dos medidas van juntas">
      <p className="text-texto leading-relaxed">
        Un <strong>episodio</strong> es una vez que la media de 24 horas de una estación cruza el ECA. Hubo{' '}
        <strong>{numero(t.episodios)}</strong> en todo el gold. Para cada uno se mira, en las 24 horas anteriores, cuánto
        antes ya estaba encendida cada señal, de dos maneras.
      </p>
      <div className="grid gap-4 md:grid-cols-2">
        <div className="border-borde bg-relieve rounded-lg border p-4">
          <p className="rotulo">Desde la primera vez que se enciende</p>
          <div className="mt-2 grid grid-cols-2 gap-4">
            <Cifra tamano="mediana" rotulo="Lectura horaria" valor={lh.horas_antes?.mediana} decimales={Number.isInteger(lh.horas_antes?.mediana) ? 0 : 1} unidad="h · mediana" />
            <Cifra tamano="mediana" rotulo="Régimen alto" valor={reg.horas_antes?.mediana} decimales={Number.isInteger(reg.horas_antes?.mediana) ? 0 : 1} unidad="h · mediana" />
          </div>
          <p className="text-suave mt-2 text-sm leading-relaxed">
            Cuenta desde el primer encendido, aunque después la señal se haya apagado. No puede pasar de {tope} horas.
          </p>
        </div>
        <div className="border-borde bg-relieve rounded-lg border p-4">
          <p className="rotulo">Encendida sin interrupción antes del cruce</p>
          <div className="mt-2 grid grid-cols-2 gap-4">
            <Cifra tamano="mediana" rotulo="Lectura horaria" valor={lh.horas_seguidas?.mediana} decimales={Number.isInteger(lh.horas_seguidas?.mediana) ? 0 : 1} unidad="h · mediana" />
            <Cifra tamano="mediana" rotulo="Régimen alto" valor={reg.horas_seguidas?.mediana} decimales={Number.isInteger(reg.horas_seguidas?.mediana) ? 0 : 1} unidad="h · mediana" />
          </div>
          <p className="text-suave mt-2 text-sm leading-relaxed">
            Es la cota baja: cuántas horas seguidas llevaba encendida cuando la media cruzó. Una hora apagada corta la
            cuenta.
          </p>
        </div>
      </div>
      {lh.horas_antes && lh.horas_seguidas && reg.horas_antes && reg.horas_seguidas && (
        <Frase tono="moderada">
          La señal se prende y se apaga. En la mitad de los {numero(t.episodios)} episodios, la lectura horaria había pasado
          el umbral por primera vez {enHoras(lh.horas_antes.mediana)} antes del cruce o más; pero en la mitad llevaba
          encendida sin interrupción {enHoras(lh.horas_seguidas.mediana)} o menos. Con el régimen alto,{' '}
          {enHoras(reg.horas_antes.mediana)} y {enHoras(reg.horas_seguidas.mediana)}. No es un aviso sostenido de{' '}
          {horas(lh.horas_antes.mediana)} horas.
        </Frase>
      )}
      <AnticipacionEpisodios medidas={medidas} tope={tope} />
      <ul className="text-suave flex flex-wrap gap-x-5 gap-y-1 text-xs">
        <li className="flex items-center gap-1.5">
          <span aria-hidden="true" className="bg-acento-claro inline-block size-2 rounded-full" /> Un episodio
        </li>
        <li className="flex items-center gap-1.5">
          <span aria-hidden="true" className="border-acento-claro inline-block size-2 rounded-full border-[1.5px]" /> En el
          tope de {tope} h: la señal ya estaba encendida al empezar la ventana ({numero(lh.en_el_tope)} episodios con la
          lectura horaria y {numero(reg.en_el_tope)} con el régimen)
        </li>
        <li className="flex items-center gap-1.5">
          <span aria-hidden="true" className="bg-texto inline-block h-3 w-0.5" /> Mediana
        </li>
        <li className="flex items-center gap-1.5">
          <span aria-hidden="true" className="bg-acento/20 inline-block h-3 w-4 rounded-sm" /> Entre el primer y el tercer
          cuartil
        </li>
      </ul>

      <div className="border-inca-moderada bg-relieve rounded-lg border-l-4 px-4 py-3">
        <p className="text-texto font-semibold">Lo que esta medida no es</p>
        <p className="text-texto mt-1 text-sm leading-relaxed">{conCodigo(a.lo_que_no_es)}</p>
      </div>

      <Desplegable titulo={`Por estación: las ${numero(conEpisodios.length)} que tuvieron episodios`}>
        <Tabla titulo="Anticipación por estación: mediana y rango, en horas" columnas={COLUMNAS_POR_ESTACION} filas={conEpisodios} claveDe={(f) => f.estacion} />
        <p>
          Cada celda es la mediana de la estación y, debajo, su mínimo y su máximo. Con uno o dos episodios, la mediana
          dice poco. En las otras {numero(sinEpisodios)} estaciones la media de 24 horas nunca cruzó el ECA.
        </p>
      </Desplegable>
      <DeDondeSale impacto={impacto} fuentes={['gold', 'asignaciones', 'regimenes']}>
        <p>{a.definicion.episodio}</p>
        <p>{a.definicion.ventana_de_busqueda}</p>
        <p>
          <strong className="text-texto">Desde la primera vez.</strong> Lectura horaria: {a.definicion.horas_antes_lectura_horaria}{' '}
          Régimen: {a.definicion.horas_antes_regimen}
        </p>
        <p>
          <strong className="text-texto">Horas seguidas.</strong> {a.definicion.horas_seguidas}
        </p>
        <p>
          {numero(t.con_media_previa)} de los {numero(t.episodios)} episodios tienen media calculable en la hora anterior
          (el cruce se ve); en los otros {numero(t.sin_media_previa)} la media aparece ya sobre el ECA. Contando solo los
          primeros, las medianas son {horas(a.solo_con_media_previa.lectura_horaria.horas_antes?.mediana)} y{' '}
          {horas(a.solo_con_media_previa.regimen.horas_antes?.mediana)} horas desde la primera vez, y{' '}
          {horas(a.solo_con_media_previa.lectura_horaria.horas_seguidas?.mediana)} y{' '}
          {horas(a.solo_con_media_previa.regimen.horas_seguidas?.mediana)} horas seguidas. {a.definicion.cuartiles}
        </p>
      </DeDondeSale>
    </Seccion>
  )
}

export function LoQueNoPuedeDecir({ impacto }: { impacto: Impacto }) {
  const { alcance } = impacto
  // El distrito más poblado es el que decidiría una suma: se nombra con sus estaciones, sin sumar nada.
  const mayor = [...impacto.distritos].filter((d) => d.poblacion !== null).sort((a, b) => (b.poblacion ?? 0) - (a.poblacion ?? 0))[0]
  const deMayor = impacto.estaciones.filter((e) => mayor && e.ubigeo === mayor.ubigeo)
  const porTipo = new Map<string, string[]>()
  for (const e of deMayor) porTipo.set(e.tipo_actividad, [...(porTipo.get(e.tipo_actividad) ?? []), e.estacion])
  const cortas = impacto.estaciones.filter((e) => e.dias_con_datos < DIAS_DE_CAMPANA_CORTA)
  const nota = (trozo: string) => impacto.metadatos.notas.find((n) => n.includes(trozo))

  return (
    <section aria-labelledby="lo-que-no-dice" className="border-borde bg-superficie border-l-inca-moderada rounded-xl border border-l-4 p-5">
      <header className="mb-4">
        <p className="rotulo">A la vista, no en letra chica</p>
        <h2 id="lo-que-no-dice" className="text-texto text-lg font-semibold">
          Lo que este dato no puede decir
        </h2>
      </header>
      <ul className="text-texto grid gap-x-8 gap-y-4 leading-relaxed md:grid-cols-2">
        <li>
          <strong>No dice quién respiró ese aire.</strong> Las estaciones de OEFA vigilan actividades fiscalizadas y miden
          cerca de ellas. Que alguien viva en un distrito con una estación en alerta no dice que haya respirado ese aire.
        </li>
        <li>
          <strong>No hay un total de personas.</strong> La población no se suma entre distritos.
          {mayor && deMayor.length > 0 && (
            <>
              {' '}
              Uno solo decidiría la cifra: {nombrePropio(mayor.distrito)}, por {enLetras(deMayor.length)}{' '}
              {deMayor.length === 1 ? 'estación' : 'estaciones'} ({[...porTipo].map(([tipo, codigos]) => `${enumerar(codigos)}: ${tipo}`).join('; ')}
              ). Un número grande junto a «horas sobre el ECA» se lee como «personas afectadas», se escriba como se
              escriba.
            </>
          )}
        </li>
        <li>
          <strong>No describe un año entero.</strong> {numero(cortas.length)} de las {numero(impacto.estaciones.length)}{' '}
          estaciones son campañas de pocas semanas: sus horas sobre el ECA describen esas semanas.
        </li>
        <li>
          <strong>No dice nada de donde no se mide.</strong> Solo entran las {numero(alcance.estaciones_con_pm_horario)}{' '}
          estaciones con PM por hora, de {numero(alcance.estaciones_en_la_ficha)}. Un distrito sin estación, o con una que
          no mide PM por hora, no es un distrito con aire limpio.
        </li>
        <li>
          <strong>No cuenta expuestos.</strong> A la pregunta de a cuántas personas podría beneficiar se responde con
          quiénes tendrían un aviso que hoy no tienen, no con cuántos estuvieron expuestos.
        </li>
        <li>
          <strong>No es un pronóstico.</strong> El sistema clasifica y avisa sobre lo ya medido; la anticipación de arriba
          es una descripción de lo que pasó, no una promesa de cuánto antes avisaría.
        </li>
      </ul>
      <div className="mt-4">
        <Desplegable titulo={`Las ${numero(alcance.estaciones_sin_pm_horario.length)} estaciones que no entran en ninguna cifra`}>
          <p>{nota('Solo entran')}</p>
          <p className="font-mono">{alcance.estaciones_sin_pm_horario.join(', ')}</p>
          {alcance.estaciones_sin_distrito.length > 0 && (
            <p>
              {enumerar(alcance.estaciones_sin_distrito)} mide PM por hora, pero la fuente no trae su distrito: está en las
              horas y no tiene población de contexto.
            </p>
          )}
        </Desplegable>
      </div>
    </section>
  )
}
