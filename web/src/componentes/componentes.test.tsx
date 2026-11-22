import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { errorDeFetch, errorDeRespuesta } from '../api/errores'
import {
  Advertencia,
  Aviso,
  Banda,
  BANDAS,
  Barrera,
  Cifra,
  Dato,
  Desplegable,
  Encabezado,
  EnConstruccion,
  Estado,
  EstadoDeError,
  Etiqueta,
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

    expect(screen.getByText('En alerta')).toHaveClass('text-falla', 'border-inca-cuidado')
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

describe('Encabezado', () => {
  it('pone el título de la pantalla como h1, lo que lo acompaña a su lado y la bajada debajo', () => {
    render(
      <Encabezado titulo="Estación" junto={<p>CA-HU-04</p>} bajada="El último estado recibido de una estación.">
        <button type="button">Elegir estación</button>
      </Encabezado>,
    )

    const titulo = screen.getByRole('heading', { level: 1, name: 'Estación' })
    const encabezado = titulo.closest('header')!
    expect(encabezado).toHaveAttribute('data-encabezado')
    expect(within(encabezado).getByText('CA-HU-04')).toBeInTheDocument()
    expect(within(encabezado).getByRole('button', { name: 'Elegir estación' })).toBeInTheDocument()
    // La bajada va después del renglón del título, no dentro de él.
    const bajada = within(encabezado).getByText('El último estado recibido de una estación.')
    expect(bajada.previousElementSibling).toContainElement(titulo)
  })

  it('solo con título no deja huecos', () => {
    render(<Encabezado titulo="Impacto" />)

    const encabezado = screen.getByRole('heading', { level: 1, name: 'Impacto' }).closest('header')!
    expect(encabezado.children).toHaveLength(1)
    expect(encabezado.querySelector('p')).toBeNull()
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

describe('Estado compacto', () => {
  it('dentro de una tarjeta va sin su borde, con el mismo título, el mismo dibujo y la misma acción', async () => {
    const reintentar = vi.fn()
    render(
      <Estado tipo="vacio" titulo="Todavía no hay eventos" compacto accion={{ etiqueta: 'Volver a pedir', alHacer: reintentar }}>
        Aparecen cuando un nodo entra al pool.
      </Estado>,
    )

    const estado = screen.getByRole('status')
    expect(estado).toHaveAttribute('data-estado', 'vacio')
    expect(estado).toHaveAttribute('data-compacto', 'true')
    expect(estado.className).not.toMatch(/border/)
    expect(within(estado).getByText('Todavía no hay eventos')).toBeInTheDocument()
    expect(estado.querySelector('svg')).toBeInTheDocument()
    await userEvent.click(within(estado).getByRole('button', { name: 'Volver a pedir' }))
    expect(reintentar).toHaveBeenCalledOnce()
  })

  it('el de siempre lleva su borde y no se marca como compacto', () => {
    render(<Estado tipo="cargando" />)

    expect(screen.getByRole('status')).not.toHaveAttribute('data-compacto')
    expect(screen.getByRole('status').className).toMatch(/border-dashed/)
  })

  it('un error compacto sigue interrumpiendo y ofreciendo reintentar', () => {
    render(<EstadoDeError error={errorDeRespuesta(503, { error: 'MongoDB no disponible' }, true)} alReintentar={vi.fn()} compacto />)

    expect(screen.getByRole('alert')).toHaveAttribute('data-compacto', 'true')
    expect(screen.getByRole('button', { name: 'Reintentar' })).toBeInTheDocument()
  })
})

describe('Aviso y Advertencia', () => {
  it('un rechazo interrumpe (alert), con su título, las palabras de la API y su acción', async () => {
    const reintentar = vi.fn()
    render(
      <Aviso tono="alerta" titulo="La API rechazó el pedido" accion={{ etiqueta: 'Reintentar', alHacer: reintentar }}>
        <p>La API dice: «hay 2 nodos libres en el pool y se pidieron 4»</p>
      </Aviso>,
    )

    const aviso = screen.getByRole('alert')
    expect(aviso).toHaveAttribute('data-aviso', 'alerta')
    expect(within(aviso).getByText('La API rechazó el pedido')).toBeInTheDocument()
    expect(aviso).toHaveTextContent('hay 2 nodos libres en el pool y se pidieron 4')
    expect(aviso.className).toMatch(/border-l-inca-cuidado/)
    await userEvent.click(within(aviso).getByRole('button', { name: 'Reintentar' }))
    expect(reintentar).toHaveBeenCalledOnce()
  })

  it('un aviso que no es un error no interrumpe (status) y lleva el tono «aviso»', () => {
    render(<Aviso titulo="La sesión venció.">Vuelve a ingresar para seguir.</Aviso>)

    const aviso = screen.getByRole('status')
    expect(aviso).toHaveAttribute('data-aviso', 'aviso')
    expect(aviso.className).toMatch(/border-l-aviso/)
    expect(aviso).toHaveTextContent('La sesión venció.Vuelve a ingresar para seguir.')
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('solo con título no deja un hueco para el detalle', () => {
    render(<Aviso tono="alerta" titulo="Usuario o clave incorrectos." />)

    expect(screen.getByRole('alert')).toHaveTextContent(/^Usuario o clave incorrectos\.$/)
    expect(screen.getByRole('alert').children).toHaveLength(1)
  })

  it('la acción se puede deshabilitar mientras se reintenta', () => {
    render(<Aviso tono="alerta" titulo="La API no responde." accion={{ etiqueta: 'Reintentar', alHacer: vi.fn(), deshabilitada: true }} />)

    expect(screen.getByRole('button', { name: 'Reintentar' })).toBeDisabled()
  })

  it('la advertencia es una línea en el tono «aviso», y acepta atributos data-', () => {
    render(<Advertencia data-aviso="faltan-nodos">Hay 1 nodo libre y pides 2.</Advertencia>)

    const linea = screen.getByText('Hay 1 nodo libre y pides 2.')
    expect(linea.className).toMatch(/text-aviso/)
    expect(linea).toHaveAttribute('data-aviso', 'faltan-nodos')
  })

  it('el tono «aviso» también sirve para una etiqueta, una frase y una tarjeta', () => {
    render(
      <Tarjeta acento="aviso" titulo="En pausa">
        <Etiqueta tono="aviso">Detenida</Etiqueta>
        <Frase tono="aviso">Solo difiere el régimen.</Frase>
      </Tarjeta>,
    )

    expect(screen.getByText('Detenida').className).toMatch(/border-aviso/)
    expect(screen.getByText('Detenida').className).toMatch(/text-aviso/)
    expect(screen.getByText('Solo difiere el régimen.').className).toMatch(/border-l-aviso/)
    expect(screen.getByRole('heading', { name: 'En pausa' }).closest('section')!.className).toMatch(/border-l-aviso/)
  })
})

describe('Dato', () => {
  it('muestra un dato escrito con su rótulo, en la monoespaciada de las cifras', () => {
    render(
      <Dato rotulo="Motivo de paro" titulo="Los centroides dejaron de moverse (tolerancia)">
        Convergió
      </Dato>,
    )

    expect(screen.getByText('Motivo de paro').className).toMatch(/rotulo/)
    expect(screen.getByText('Convergió').className).toMatch(/font-mono/)
    expect(screen.getByText('Convergió').closest('[title]')).toHaveAttribute('title', 'Los centroides dejaron de moverse (tolerancia)')
  })

  it('lleva un detalle chico debajo cuando se le da', () => {
    render(
      <Dato tamano="mediana" rotulo="Último estado recibido" detalle="CA-TAL-01 · llegó el 22 nov 2026, 00:28">
        16 dic 2021, 00:00
      </Dato>,
    )

    expect(screen.getByText('16 dic 2021, 00:00')).toBeInTheDocument()
    expect(screen.getByText('CA-TAL-01 · llegó el 22 nov 2026, 00:28').className).toMatch(/text-xs/)
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
    render(<EstadoDeError error={errorDeFetch(new TypeError('Failed to fetch'))} alReintentar={vi.fn()} />)

    expect(screen.getByText('La API no responde')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Reintentar' })).toBeInTheDocument()
  })

  it('un error del programa se dice como tal y no ofrece reintentar', () => {
    render(<EstadoDeError error={new TypeError('x is not a function')} alReintentar={vi.fn()} />)

    expect(screen.getByText('Algo falló')).toBeInTheDocument()
    expect(screen.getByText('x is not a function')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Reintentar' })).not.toBeInTheDocument()
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

describe('Barrera', () => {
  function Rompe({ error }: { error: Error | null }): never | string {
    if (error) throw error
    return 'pantalla dibujada'
  }

  it('si lo de adentro lanza, lo dice en vez de dejar la página en blanco, y reintentar lo vuelve a dibujar', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    let error: Error | null = new Error('se rompió al dibujar')
    function Pantalla() {
      return <Rompe error={error} />
    }
    render(
      <Barrera alcance="pantalla">
        <Pantalla />
      </Barrera>,
    )

    const alerta = screen.getByRole('alert')
    expect(within(alerta).getByText('Esta pantalla falló')).toBeInTheDocument()
    expect(within(alerta).getByText('se rompió al dibujar')).toBeInTheDocument()

    error = null
    await userEvent.click(within(alerta).getByRole('button', { name: 'Reintentar' }))

    expect(screen.getByText('pantalla dibujada')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('un trozo de la app que no se pudo descargar ofrece recargar la página, no reintentar', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const recargar = vi.fn()
    const fallo = new TypeError('Failed to fetch dynamically imported module: http://localhost:8081/assets/Panel-abc.js')
    render(
      <Barrera alcance="pantalla" recargar={recargar}>
        <Rompe error={fallo} />
      </Barrera>,
    )

    expect(screen.getByText('Esta pantalla no se pudo cargar')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Reintentar' })).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Recargar la página' }))
    expect(recargar).toHaveBeenCalledOnce()
  })

  it('en la raíz dice que la aplicación falló y ofrece recargar', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const recargar = vi.fn()
    render(
      <Barrera alcance="aplicacion" recargar={recargar}>
        <Rompe error={new Error('proveedor roto')} />
      </Barrera>,
    )

    expect(screen.getByText('La aplicación falló')).toBeInTheDocument()
    expect(screen.getByText('proveedor roto')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Recargar la página' }))
    expect(recargar).toHaveBeenCalledOnce()
  })

  it('sin errores no se nota', () => {
    render(
      <Barrera alcance="pantalla">
        <Rompe error={null} />
      </Barrera>,
    )

    expect(screen.getByText('pantalla dibujada')).toBeInTheDocument()
  })
})
