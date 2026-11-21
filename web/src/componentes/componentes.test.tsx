import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { errorDeRespuesta } from '../api/errores'
import {
  Banda,
  BANDAS,
  Cifra,
  Desplegable,
  EnConstruccion,
  Estado,
  EstadoDeError,
  Etiqueta,
  fechaHora,
  Frase,
  numero,
  Pildoras,
  Tabla,
  Tarjeta,
  type Columna,
} from './indice'

describe('Tarjeta', () => {
  it('muestra el rótulo, el título y el contenido', () => {
    render(
      <Tarjeta rotulo="CA-AYAS-01 · media de 24 h" titulo="San Marcos" aLaDerecha="14 nov 2026">
        contenido
      </Tarjeta>,
    )

    expect(screen.getByRole('heading', { name: 'San Marcos' })).toBeInTheDocument()
    expect(screen.getByText('CA-AYAS-01 · media de 24 h')).toBeInTheDocument()
    expect(screen.getByText('14 nov 2026')).toBeInTheDocument()
    expect(screen.getByText('contenido')).toBeInTheDocument()
  })

  it('con acento lleva el borde izquierdo de ese color', () => {
    const { container, rerender } = render(<Tarjeta acento="alerta">x</Tarjeta>)
    expect(container.firstElementChild).toHaveClass('border-l-4', 'border-l-inca-cuidado')

    rerender(<Tarjeta>x</Tarjeta>)
    expect(container.firstElementChild).not.toHaveClass('border-l-4')
  })
})

describe('Cifra', () => {
  it('muestra el número en monoespaciada con su unidad al lado', () => {
    render(<Cifra valor={121.44} decimales={1} unidad="µg/m³" rotulo="PM10 · media de 24 h" />)

    const valor = screen.getByText('121.4')
    expect(valor).toHaveClass('font-mono')
    expect(screen.getByText('µg/m³')).toBeInTheDocument()
    expect(screen.getByText('PM10 · media de 24 h')).toBeInTheDocument()
  })

  it('separa los miles', () => {
    render(<Cifra valor={48161} unidad="horas" />)

    expect(screen.getByText('48,161')).toBeInTheDocument()
  })

  it('sin valor muestra una raya y dice por qué, en vez de un cero', () => {
    render(<Cifra valor={null} unidad="µg/m³" sinDato="sin datos suficientes" />)

    expect(screen.getByText('—')).toBeInTheDocument()
    expect(screen.getByText('sin datos suficientes')).toBeInTheDocument()
    expect(screen.queryByText('µg/m³')).not.toBeInTheDocument()
    expect(screen.queryByText('0')).not.toBeInTheDocument()
  })

  it('el cero es un valor, no una falta de dato', () => {
    render(<Cifra valor={0} unidad="estaciones en alerta" />)

    expect(screen.getByText('0')).toBeInTheDocument()
    expect(screen.getByText('estaciones en alerta')).toBeInTheDocument()
  })
})

describe('Frase', () => {
  it('dice la frase con el borde del tono', () => {
    render(<Frase tono="mala">La media de las últimas 24 horas de PM10 es 121 µg/m³: supera el ECA (100).</Frase>)

    expect(screen.getByText(/supera el ECA \(100\)/)).toHaveClass('border-l-inca-mala')
  })
})

describe('Banda INCA', () => {
  it.each([
    ['buena', 'Buena', 'bg-inca-buena'],
    ['moderada', 'Moderada', 'bg-inca-moderada'],
    ['mala', 'Mala', 'bg-inca-mala'],
    ['umbral_de_cuidado', 'Umbral de cuidado', 'bg-inca-cuidado'],
    ['no_mide', 'No mide PM por hora', 'bg-no-mide'],
  ] as const)('la banda «%s» lleva su palabra («%s») además de su color', (valor, palabra, color) => {
    const { container } = render(<Banda valor={valor} />)

    expect(screen.getByText(palabra)).toBeInTheDocument()
    expect(container.querySelector('[aria-hidden="true"]')).toHaveClass(color)
  })

  it('null, que es lo que manda la API con menos de 18 lecturas, es «sin datos suficientes»', () => {
    render(<Banda valor={null} />)

    expect(screen.getByText('Sin datos suficientes')).toBeInTheDocument()
  })

  it('«no mide PM por hora» lleva trama, para no depender del gris', () => {
    const { container } = render(<Banda valor="no_mide" />)

    expect(container.querySelector('[aria-hidden="true"]')).toHaveClass('trama')
  })

  it('con prefijo dice «INCA mala»', () => {
    render(<Banda valor="mala" conPrefijo />)

    expect(screen.getByText('INCA mala')).toBeInTheDocument()
  })

  it('cada estado tiene una palabra distinta y un color distinto', () => {
    const info = Object.values(BANDAS)

    expect(new Set(info.map((b) => b.palabra)).size).toBe(info.length)
    expect(new Set(info.map((b) => b.color)).size).toBe(info.length)
  })

  it('las bandas se ordenan por gravedad', () => {
    const orden = (['buena', 'moderada', 'mala', 'umbral_de_cuidado'] as const).map((b) => BANDAS[b].gravedad)

    expect(orden).toEqual([...orden].sort((a, b) => a - b))
  })
})

describe('Etiqueta', () => {
  it('lleva su texto y el color del tono', () => {
    render(<Etiqueta tono="alerta">En alerta</Etiqueta>)

    expect(screen.getByText('En alerta')).toHaveClass('text-inca-cuidado', 'border-inca-cuidado')
  })
})

describe('Pildoras', () => {
  const opciones = [
    { valor: 'pm2_5', etiqueta: 'PM2.5' },
    { valor: 'pm10', etiqueta: 'PM10' },
    { valor: 'ambos', etiqueta: 'Los dos', deshabilitada: true },
  ] as const

  it('marca la opción elegida y avisa al cambiar', async () => {
    const alCambiar = vi.fn()
    render(<Pildoras etiqueta="Contaminante" opciones={opciones} valor="pm2_5" alCambiar={alCambiar} />)

    const grupo = screen.getByRole('group', { name: 'Contaminante' })
    expect(within(grupo).getByRole('button', { name: 'PM2.5' })).toHaveAttribute('aria-pressed', 'true')
    expect(within(grupo).getByRole('button', { name: 'PM10' })).toHaveAttribute('aria-pressed', 'false')

    await userEvent.click(within(grupo).getByRole('button', { name: 'PM10' }))
    expect(alCambiar).toHaveBeenCalledExactlyOnceWith('pm10')
  })

  it('una opción deshabilitada no se puede elegir', async () => {
    const alCambiar = vi.fn()
    render(<Pildoras etiqueta="Contaminante" opciones={opciones} valor="pm2_5" alCambiar={alCambiar} />)

    await userEvent.click(screen.getByRole('button', { name: 'Los dos' }))

    expect(alCambiar).not.toHaveBeenCalled()
  })
})

describe('Tabla', () => {
  interface Fila {
    id: string
    distrito: string
    pm10: number
  }
  const columnas: Columna<Fila>[] = [
    { clave: 'id', titulo: 'Estación', celda: (f) => f.id },
    { clave: 'distrito', titulo: 'Distrito', celda: (f) => f.distrito },
    { clave: 'pm10', titulo: 'PM10', numerica: true, celda: (f) => numero(f.pm10, 1) },
  ]

  it('dibuja una fila por elemento, con sus encabezados', () => {
    const filas = [
      { id: 'CA-AYAS-01', distrito: 'San Marcos', pm10: 121.4 },
      { id: 'CA-MAC-01', distrito: 'Ilo', pm10: 40 },
    ]
    render(<Tabla titulo="Estaciones" columnas={columnas} filas={filas} claveDe={(f) => f.id} />)

    const tabla = screen.getByRole('table', { name: 'Estaciones' })
    expect(within(tabla).getAllByRole('columnheader').map((c) => c.textContent)).toEqual(['Estación', 'Distrito', 'PM10'])
    expect(within(tabla).getAllByRole('row')).toHaveLength(3)
    expect(within(tabla).getByRole('cell', { name: '121.4' })).toHaveClass('text-right', 'font-mono')
  })

  it('sin filas dice por qué, en vez de una tabla en blanco', () => {
    render(
      <Tabla
        titulo="Estaciones"
        columnas={columnas}
        filas={[]}
        claveDe={(f) => f.id}
        vacio="Ninguna estación tiene estado todavía."
      />,
    )

    expect(screen.getByRole('cell', { name: 'Ninguna estación tiene estado todavía.' })).toHaveAttribute('colspan', '3')
  })
})

describe('Estado', () => {
  it.each([
    ['cargando', 'status', 'Cargando…'],
    ['vacio', 'status', 'Todavía no hay nada que mostrar'],
    ['error', 'alert', 'Algo falló'],
    ['sin-permiso', 'status', 'No tienes permiso para ver esto'],
  ] as const)('«%s» se anuncia como %s y dice «%s»', (tipo, rol, titulo) => {
    render(<Estado tipo={tipo} />)

    const estado = screen.getByRole(rol)
    expect(estado).toHaveAttribute('data-estado', tipo)
    expect(within(estado).getByText(titulo)).toBeInTheDocument()
    // Además del color y del título, cada estado tiene su dibujo.
    expect(estado.querySelector('svg')).toBeInTheDocument()
  })

  it('vacío explica por qué y ofrece qué hacer', async () => {
    const cargar = vi.fn()
    render(
      <Estado
        tipo="vacio"
        titulo="Ninguna estación tiene estado"
        accion={{ etiqueta: 'Cargar el escenario', alHacer: cargar }}
      >
        El sistema recién se levantó y todavía no recibió lecturas.
      </Estado>,
    )

    expect(screen.getByText('Ninguna estación tiene estado')).toBeInTheDocument()
    expect(screen.getByText(/todavía no recibió lecturas/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Cargar el escenario' }))
    expect(cargar).toHaveBeenCalledOnce()
  })
})

describe('EstadoDeError', () => {
  it('un 503 se muestra con las palabras de la API y ofrece reintentar', async () => {
    const reintentar = vi.fn()
    render(
      <EstadoDeError error={errorDeRespuesta(503, { error: 'MongoDB no disponible' }, true)} alReintentar={reintentar} />,
    )

    const alerta = screen.getByRole('alert')
    expect(within(alerta).getByText('El servicio no está disponible')).toBeInTheDocument()
    expect(within(alerta).getByText('MongoDB no disponible')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Reintentar' }))
    expect(reintentar).toHaveBeenCalledOnce()
  })

  it('un 403 es «sin permiso» y no ofrece reintentar', () => {
    render(<EstadoDeError error={errorDeRespuesta(403, { error: 'se requiere el rol admin' }, true)} alReintentar={vi.fn()} />)

    expect(screen.getByRole('status')).toHaveAttribute('data-estado', 'sin-permiso')
    expect(screen.getByText('se requiere el rol admin')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Reintentar' })).not.toBeInTheDocument()
  })

  it('si la API no responde lo dice, no se queda en blanco', () => {
    render(<EstadoDeError error={new TypeError('Failed to fetch')} alReintentar={vi.fn()} />)

    expect(screen.getByText('La API no responde')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Reintentar' })).toBeInTheDocument()
  })
})

describe('Desplegable y EnConstruccion', () => {
  it('el desplegable guarda la explicación detrás de su pregunta', () => {
    render(<Desplegable titulo="¿De dónde sale este número?">De la media de las últimas 24 horas.</Desplegable>)

    expect(screen.getByText('¿De dónde sale este número?').closest('details')).not.toHaveAttribute('open')
    expect(screen.getByText('De la media de las últimas 24 horas.')).toBeInTheDocument()
  })

  it('el marcador dice qué pantalla es, dónde está en la spec y en qué archivo se construye', () => {
    render(
      <EnConstruccion pantalla="Panel" seccion="§5.2" archivo="web/src/pantallas/Panel.tsx">
        Aquí va el mapa.
      </EnConstruccion>,
    )

    expect(screen.getByRole('heading', { name: 'Panel' })).toBeInTheDocument()
    expect(screen.getByText('En construcción')).toBeInTheDocument()
    expect(screen.getByText('web/src/pantallas/Panel.tsx')).toBeInTheDocument()
    expect(screen.getByText(/§5\.2/)).toBeInTheDocument()
  })
})

describe('formato', () => {
  it('escribe los números con sus decimales y una raya cuando no hay', () => {
    expect(numero(58.349, 1)).toBe('58.3')
    expect(numero(1536)).toBe('1,536')
    expect(numero(null)).toBe('—')
    expect(numero(Number.NaN)).toBe('—')
  })

  it('escribe las fechas en la hora de Lima, esté donde esté el navegador', () => {
    expect(fechaHora('2021-12-13T20:00:00Z')).toMatch(/13 dic.* 2021.* 15:00/)
    expect(fechaHora('no es una fecha')).toBe('—')
  })
})
