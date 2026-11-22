// La pantalla de Impacto: que cada cifra salga de impacto.json, que la
// población no se sume nunca, y que la anticipación muestre sus dos medidas.

import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import crudoDeImpacto from '../../public/datos/impacto.json?raw'
import { crearClienteDeConsultas } from '../api/cliente'
import { Proveedores } from '../App'
import { numero } from '../componentes/formato'
import { fabricaDeEnchufes, respuesta } from '../pruebas/dobles'
import { abrirSesion, cerrarSesion, recargarSesion } from '../sesion/almacen'
import Impacto from './Impacto'
import type { Impacto as DatosDeImpacto } from './impacto/tipos'

const REAL = JSON.parse(crudoDeImpacto) as DatosDeImpacto
const copia = () => JSON.parse(crudoDeImpacto) as DatosDeImpacto

function montar(datos: unknown = REAL) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (pedido: Request) => {
      const ruta = new URL(pedido.url).pathname
      if (ruta === '/datos/impacto.json') return datos === null ? respuesta(404) : respuesta(200, datos)
      return respuesta(404, { error: `no hay doble para ${ruta}` })
    }),
  )
  abrirSesion({ usuario: 'demo', token: 'token-de-usuario', rol: 'usuario', expira: Date.now() + 3_600_000 })
  const enchufes = fabricaDeEnchufes()
  return render(
    <Proveedores consultas={crearClienteDeConsultas()} tiempoReal={{ crearEnchufe: enchufes.crearEnchufe }}>
      <Impacto />
    </Proveedores>,
  )
}

const seccion = (nombre: RegExp) => screen.findByRole('region', { name: nombre })
const texto = () => document.body.textContent ?? ''

/** Todas las maneras en que un número podría aparecer escrito. */
function escrituras(n: number): string[] {
  return [String(n), numero(n), numero(n).replaceAll(',', '.'), numero(n).replaceAll(',', ' '), numero(n).replaceAll(',', String.fromCharCode(160))]
}

describe('Impacto', () => {
  beforeEach(() => recargarSesion())
  afterEach(() => cerrarSesion('salir'))

  it('si no se puede leer el archivo, lo dice', async () => {
    montar(null)

    expect(screen.getByRole('heading', { name: 'Impacto' })).toBeInTheDocument()
    expect(within(await screen.findByRole('alert')).getByText(/no se pudo leer \/datos\/impacto\.json/)).toBeInTheDocument()
  })

  it('tiene las siete partes de la página, de arriba abajo', async () => {
    montar()
    await seccion(/ECA y qué es el INCA/)

    expect(screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent)).toEqual([
      'Qué es el ECA y qué es el INCA',
      'Las horas en que la media de 24 horas superó el ECA',
      'Cada estación vigila una actividad: esa es la unidad honesta',
      'Quiénes tendrían un aviso que hoy no tienen',
      'La alerta del sistema es la norma misma; las señales hora a hora aciertan menos',
      'Depende de cómo se cuente: las dos medidas van juntas',
      'Lo que este dato no puede decir',
    ])
  })

  describe('la población', () => {
    it('va fila por fila, con su distrito y el enlace a su fuente', async () => {
      montar()
      const tabla = within(await seccion(/Quiénes tendrían un aviso/)).getByRole('table')

      const conDistrito = REAL.distritos.filter((d) => d.distrito !== null)
      expect(conDistrito).toHaveLength(REAL.alcance.distritos_con_pm_horario)
      for (const d of conDistrito) {
        const fila = within(tabla).getByText(numero(d.poblacion)).closest('tr')!
        expect(within(fila).getByRole('link', { name: 'INEI' })).toHaveAttribute('href', d.fuente)
        expect(fila).toHaveTextContent(`censo de ${d.anio}`)
        for (const estacion of d.estaciones) expect(fila).toHaveTextContent(estacion)
      }
      // La estación sin distrito en la fuente tiene su fila, sin población.
      expect(within(tabla).getByText('Sin distrito en la fuente').closest('tr')).toHaveTextContent('CA-HUAR-01')
    })

    it('no se suma: ninguna suma de poblaciones aparece en la página', async () => {
      montar()
      await seccion(/Quiénes tendrían un aviso/)
      await userEvent.click(screen.getByRole('button', { name: 'Por tipo de actividad' }))

      const sumar = (valores: (number | null | undefined)[]) => valores.reduce<number>((s, v) => s + (v ?? 0), 0)
      const sumas = new Set([
        // Los distritos con estación horaria, los de las filas de estación (repetidos) y los 16 de la ficha.
        sumar(REAL.distritos.map((d) => d.poblacion)),
        sumar(REAL.estaciones.map((e) => e.poblacion_distrito?.poblacion)),
        sumar((JSON.parse(crudoDeImpacto) as { poblacion_distritos: { poblacion: number }[] }).poblacion_distritos.map((d) => d.poblacion)),
        // Por tipo de actividad y por departamento.
        ...REAL.tipos_actividad.map((t) => sumar(t.distritos.map((d) => d.poblacion))),
        ...[...new Set(REAL.distritos.map((d) => d.departamento))].map((dep) => sumar(REAL.distritos.filter((d) => d.departamento === dep).map((d) => d.poblacion))),
      ])
      const sueltas = new Set(REAL.distritos.map((d) => d.poblacion))
      const pagina = texto()
      for (const suma of sumas) {
        // Un grupo de un solo distrito «suma» lo mismo que su fila: eso no es una suma.
        if (suma === 0 || sueltas.has(suma)) continue
        for (const escrita of escrituras(suma)) expect(pagina).not.toContain(escrita)
      }
      expect(sumas.has(632_533)).toBe(true)
      expect(pagina).not.toMatch(/632[,.\s]?533/)
    })

    it('la tabla de distritos no tiene fila de total ni pie', async () => {
      montar()
      const tabla = within(await seccion(/Quiénes tendrían un aviso/)).getByRole('table')

      expect(within(tabla).getAllByRole('row')).toHaveLength(REAL.distritos.length + 1)
      expect(tabla.querySelector('tfoot')).toBeNull()
      expect(within(tabla).queryByText(/total|suma/i)).not.toBeInTheDocument()
      expect(screen.getByText(/La población va fila por fila, como contexto de su distrito, y no se suma/)).toBeInTheDocument()
    })

    it('en la fila de cada estación es contexto: su distrito y los habitantes del censo', async () => {
      montar()
      const tabla = within(await seccion(/Cada estación vigila una actividad/)).getByRole('table')

      expect(within(tabla).getByRole('columnheader', { name: 'Distrito (contexto)' })).toBeInTheDocument()
      const fila = within(tabla).getByText('CA-RZC-02').closest('tr')!
      expect(fila).toHaveTextContent('Lurigancho, Lima')
      expect(fila).toHaveTextContent('240,814 hab. (censo de 2017)')
      expect(within(tabla).getByText('CA-HUAR-01').closest('tr')).toHaveTextContent('sin distrito en la fuente')
    })
  })

  it('en Lurigancho hay tres estaciones horarias, y la página lo dice así', async () => {
    montar()
    const distritos = within(await seccion(/Quiénes tendrían un aviso/)).getByRole('table')

    const fila = within(distritos).getByText('Lurigancho').closest('tr')!
    expect(fila).toHaveTextContent('CA-HU-04, CA-HU-09, CA-RZC-02')
    expect(fila).toHaveTextContent('ladrilleras y metalurgia (refinación o fundición)')
    const limites = await seccion(/Lo que este dato no puede decir/)
    expect(limites).toHaveTextContent('Lurigancho, por tres estaciones (CA-HU-04 y CA-HU-09: ladrilleras; CA-RZC-02: metalurgia (refinación o fundición))')
    expect(limites).not.toHaveTextContent(/dos estaciones/)
  })

  describe('por estación', () => {
    it('cada una con lo que vigila, su tipo de actividad, cuándo midió y lo medido', async () => {
      montar()
      const tabla = within(await seccion(/Cada estación vigila una actividad/)).getByRole('table')

      expect(within(tabla).getAllByRole('row')).toHaveLength(REAL.estaciones.length + 1)
      const fila = within(tabla).getByText('CA-HU-04').closest('tr')!
      expect(fila).toHaveTextContent('Zona ladrillera de centro poblado menor Santa María de Huachipa y Nievería')
      expect(fila).toHaveTextContent('CA-HU-04ladrilleras')
      expect(within(tabla).getByText('CA-SOC-01').closest('tr')).toHaveTextContent('actividad: no lo dice el nombre')
      expect(fila).toHaveTextContent('10 dic. 2021 – 20 dic. 2021')
      expect(fila).toHaveTextContent('11 días con datos')
      expect(fila).toHaveTextContent('campaña corta')
      expect(fila).toHaveTextContent('237de 237 · 100.0 %')
      expect(fila).toHaveTextContent('más frecuente: mala')
      // Una campaña larga no lleva la marca.
      expect(within(tabla).getByText('CA-CP-01').closest('tr')).not.toHaveTextContent('campaña corta')
    })

    it('va ordenada por la parte de sus horas que superó el ECA', async () => {
      montar()
      const tabla = within(await seccion(/Cada estación vigila una actividad/)).getByRole('table')

      const codigos = within(tabla).getAllByRole('row').slice(1).map((f) => f.querySelector('.font-mono')?.textContent)
      expect(codigos.slice(0, 5)).toEqual(['CA-HU-04', 'CA-RZC-02', 'CA-HU-09', 'CA-ESP-01', 'CA-CP-01'])
    })

    it('también se puede ver por tipo de actividad', async () => {
      montar()
      const parte = await seccion(/Cada estación vigila una actividad/)

      await userEvent.click(within(parte).getByRole('button', { name: 'Por tipo de actividad' }))

      const tabla = within(parte).getByRole('table')
      expect(within(tabla).getAllByRole('row')).toHaveLength(REAL.tipos_actividad.length + 1)
      const ladrilleras = within(tabla).getByText('ladrilleras').closest('tr')!
      expect(ladrilleras).toHaveTextContent('CA-HU-04, CA-HU-09')
      expect(ladrilleras).toHaveTextContent('370de 651')
    })
  })

  it('¿sirve la alerta?: la precisión y la cobertura de cada señal, con las horas de las que salen', async () => {
    montar()
    const parte = await seccion(/La alerta del sistema es la norma misma/)

    const cifra = (rotulo: string) => within(parte).getByText(rotulo).parentElement!
    expect(cifra('Régimen alto · precisión')).toHaveTextContent('35.0%')
    expect(cifra('Régimen alto · cobertura')).toHaveTextContent('44.3%')
    expect(cifra('Lectura horaria · precisión')).toHaveTextContent('21.6%')
    expect(cifra('Lectura horaria · cobertura')).toHaveTextContent('54.8%')
    expect(parte).toHaveTextContent('El régimen alto acierta el 35 % de las veces que suena (precisión) y detecta el 44 % de las horas sobre el ECA (cobertura)')
    expect(parte).toHaveTextContent('es la norma misma, no una predicción')
    expect(parte).toHaveTextContent('encendido en 1,497 horas, 524 de ellas con la media sobre el ECA')
    expect(within(parte).getByRole('img', { name: /Precisión y cobertura de cada señal/ })).toHaveAccessibleName(
      /Régimen alto \(régimen 1\): precisión 35\.0 % y cobertura 44\.3 %; Lectura horaria sobre el umbral: precisión 21\.6 % y cobertura 54\.8 %/,
    )
  })

  describe('la anticipación', () => {
    it('muestra las dos medidas juntas: desde la primera vez y encendida sin interrupción', async () => {
      montar()
      const parte = await seccion(/las dos medidas van juntas/)

      const medida = (rotulo: string) => within(parte).getByText(rotulo).parentElement!
      const primera = medida('Desde la primera vez que se enciende')
      expect(within(primera).getByText('Lectura horaria').parentElement).toHaveTextContent('20h · mediana')
      expect(within(primera).getByText('Régimen alto').parentElement).toHaveTextContent('18h · mediana')
      const seguidas = medida('Encendida sin interrupción antes del cruce')
      expect(within(seguidas).getByText('Lectura horaria').parentElement).toHaveTextContent('1h · mediana')
      expect(within(seguidas).getByText('Régimen alto').parentElement).toHaveTextContent('0.5h · mediana')
    })

    it('no dice «avisa con 20 horas» a secas: lleva el matiz y la nota del archivo', async () => {
      montar()
      const parte = await seccion(/las dos medidas van juntas/)

      expect(parte).toHaveTextContent('La señal se prende y se apaga')
      expect(parte).toHaveTextContent('20 horas antes del cruce o más; pero en la mitad llevaba encendida sin interrupción 1 hora o menos')
      expect(parte).toHaveTextContent('No es un aviso sostenido de 20 horas')
      expect(parte).toHaveTextContent('Lo que esta medida no es')
      expect(parte).toHaveTextContent(REAL.anticipacion.lo_que_no_es)
      expect(parte).toHaveTextContent('No puede pasar de 23 horas')
      expect(texto()).not.toMatch(/avisa con (\d+|veinte) horas/i)
    })

    it('dibuja los 30 episodios en cada medida y marca los que están en el tope', async () => {
      montar()
      const parte = await seccion(/las dos medidas van juntas/)

      const grafico = within(parte).getByRole('img', { name: /Anticipación de cada señal, episodio por episodio/ })
      expect(grafico.querySelectorAll('g.fila')).toHaveLength(4)
      expect(grafico.querySelectorAll('circle')).toHaveLength(4 * REAL.anticipacion.total.episodios)
      expect(grafico.querySelectorAll('circle.fill-superficie')).toHaveLength(
        REAL.anticipacion.total.lectura_horaria.en_el_tope + REAL.anticipacion.total.regimen.en_el_tope,
      )
      expect(parte).toHaveTextContent('6 episodios con la lectura horaria y 3 con el régimen')
    })

    it('por estación solo lista las que tuvieron episodios', async () => {
      montar()
      const parte = await seccion(/las dos medidas van juntas/)

      await userEvent.click(within(parte).getByText(/Por estación: las 8 que tuvieron episodios/))

      const tabla = within(parte).getByRole('table')
      expect(within(tabla).getAllByRole('row')).toHaveLength(9)
      expect(within(tabla).getByText('CA-CP-01').closest('tr')).toHaveTextContent('1421.5 h4 a 23')
    })
  })

  describe('cada cifra sale del archivo', () => {
    it('con otro impacto.json, la página muestra las cifras de ese archivo y ninguna de las de antes', async () => {
      const otro = copia()
      otro.totales.horas_sobre_eca = 4321
      otro.totales.horas_calculables = 50_000
      otro.totales.inca = { buena: 40_000, moderada: 5000, mala: 4990, umbral_de_cuidado: 10 }
      otro.totales.lecturas = 51_234
      otro.alcance.estaciones_con_pm_horario = 15
      otro.alcance.distritos_con_pm_horario = 9
      otro.metadatos.reglas.eca_pm2_5 = 37
      otro.metadatos.reglas.eca_pm10 = 88
      otro.senales.regimen_alto.precision = 0.5123
      otro.senales.regimen_alto.cobertura = 0.6789
      otro.senales.regimen_alto.horas = 2222
      otro.senales.lectura_horaria.precision = 0.1111
      otro.senales.lectura_horaria.cobertura = 0.9876
      otro.anticipacion.total.episodios = 31
      otro.anticipacion.total.lectura_horaria.horas_antes!.mediana = 7
      otro.anticipacion.total.lectura_horaria.horas_seguidas!.mediana = 2.5
      otro.anticipacion.total.regimen.horas_antes!.mediana = 6
      otro.anticipacion.total.regimen.horas_seguidas!.mediana = 3
      otro.anticipacion.lo_que_no_es = 'Una nota distinta, escrita para la prueba.'
      otro.estaciones.find((e) => e.estacion === 'CA-HU-04')!.horas_sobre_eca = 111
      otro.distritos.find((d) => d.distrito === 'LURIGANCHO')!.poblacion = 123_456
      montar(otro)

      const tiempo = await seccion(/Las horas en que la media de 24 horas superó el ECA/)
      expect(tiempo).toHaveTextContent('4,321horas sobre el ECA')
      expect(tiempo).toHaveTextContent('50,000horas · 8.6 %')
      expect(tiempo).toHaveTextContent('40,000horas · 80.0 %')
      expect(tiempo).toHaveTextContent('10horas · menos de 0.1 %')
      expect(tiempo).toHaveTextContent('Son las de 15 estaciones')
      expect(await seccion(/ECA y qué es el INCA/)).toHaveTextContent('37 µg/m³ de PM2.5 ni de 88 µg/m³ de PM10')

      const alerta = await seccion(/La alerta del sistema es la norma misma/)
      expect(alerta).toHaveTextContent('acierta el 51 % de las veces que suena (precisión) y detecta el 68 %')
      expect(alerta).toHaveTextContent('51.2%')
      expect(alerta).toHaveTextContent('98.8%')
      expect(alerta).toHaveTextContent('pase de 37 µg/m³ de PM2.5 o de 88 de PM10')

      const anticipacion = await seccion(/las dos medidas van juntas/)
      expect(anticipacion).toHaveTextContent('Hubo 31 en todo el gold')
      expect(anticipacion).toHaveTextContent('7 horas antes del cruce o más; pero en la mitad llevaba encendida sin interrupción 2.5 horas o menos')
      expect(anticipacion).toHaveTextContent('Con el régimen alto, 6 horas y 3 horas')
      expect(anticipacion).toHaveTextContent('No es un aviso sostenido de 7 horas')
      expect(anticipacion).toHaveTextContent('Una nota distinta, escrita para la prueba.')

      expect(within(await seccion(/Cada estación vigila/)).getByText('CA-HU-04').closest('tr')).toHaveTextContent('111de 237')
      expect(within(await seccion(/Quiénes tendrían un aviso/)).getByText('Lurigancho').closest('tr')).toHaveTextContent('123,456')
      expect(await seccion(/Quiénes tendrían un aviso/)).toHaveTextContent('en los 9 distritos')

      // Nada de lo que traía el archivo real quedó escrito en la pantalla.
      const pagina = texto()
      for (const vieja of ['1,184horas sobre el ECA', '64,748horas', '65,873 lecturas horarias', '35.0', '44.3', '21.6', '54.8', 'No es un aviso sostenido de 20']) {
        expect(pagina).not.toContain(vieja)
      }
      expect(pagina).not.toContain(REAL.anticipacion.lo_que_no_es)
    })

    it('cada parte con cifras dice de dónde sale su número: la regla, los archivos con su huella y quién lo calcula', async () => {
      montar()
      await seccion(/ECA y qué es el INCA/)

      const partes = [
        /Las horas en que la media de 24 horas superó el ECA/,
        /Cada estación vigila una actividad/,
        /Quiénes tendrían un aviso/,
        /La alerta del sistema es la norma misma/,
        /las dos medidas van juntas/,
      ]
      for (const nombre of partes) {
        const parte = await seccion(nombre)
        const desplegable = within(parte).getByText('¿De dónde sale este número?').closest('details')!
        expect(desplegable).toHaveTextContent('scripts/impacto.py')
        expect(desplegable).toHaveTextContent('Ninguna cifra de esta página se escribe a mano')
        expect(desplegable).toHaveTextContent(/SHA-256 [0-9a-f]{12}…/)
      }
      const tiempo = within(await seccion(/Las horas en que la media/)).getByText('¿De dónde sale este número?').closest('details')!
      expect(tiempo).toHaveTextContent(REAL.metadatos.reglas.alerta)
      expect(tiempo).toHaveTextContent(`data/gold/oefa_features.csv · 65,873 filas · SHA-256 ${REAL.metadatos.fuentes.gold!.sha256.slice(0, 12)}…`)
      const anticipacion = within(await seccion(/las dos medidas van juntas/)).getByText('¿De dónde sale este número?').closest('details')!
      expect(anticipacion).toHaveTextContent(String(REAL.anticipacion.definicion.episodio))
      expect(anticipacion).toHaveTextContent(String(REAL.anticipacion.definicion.horas_seguidas))
    })
  })

  it('dice a la vista lo que el dato no puede decir, y qué estaciones quedaron fuera', async () => {
    montar()
    const limites = await seccion(/Lo que este dato no puede decir/)

    expect(limites).toHaveTextContent('No dice quién respiró ese aire')
    expect(limites).toHaveTextContent('No hay un total de personas')
    expect(limites).toHaveTextContent('10 de las 17 estaciones son campañas de pocas semanas')
    expect(limites).toHaveTextContent('Solo entran las 17 estaciones con PM por hora, de 33')
    expect(limites).toHaveTextContent('no es un distrito con aire limpio')
    expect(limites).toHaveTextContent('no con cuántos estuvieron expuestos')
    expect(within(limites).getByText(/Las 16 estaciones que no entran en ninguna cifra/).closest('details')).toHaveTextContent(
      REAL.alcance.estaciones_sin_pm_horario.join(', '),
    )
  })
})
