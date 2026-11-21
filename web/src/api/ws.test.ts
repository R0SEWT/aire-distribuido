import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fabricaDeEnchufes } from '../pruebas/dobles'
import { ConexionWS, urlWS, type EstadoWS } from './ws'

const HORA = 3_600_000

function preparar(opciones: { expiraEn?: number; alFallarApreton?: (fallos: number) => void } = {}) {
  const fabrica = fabricaDeEnchufes()
  const conexion = new ConexionWS({
    alFallarApreton: opciones.alFallarApreton,
    url: 'ws://localhost/ws',
    token: 'el-token',
    expira: Date.now() + (opciones.expiraEn ?? HORA),
    crearEnchufe: fabrica.crearEnchufe,
    azar: () => 1, // sin azar: la espera es exactamente el tope
  })
  const estados: EstadoWS[] = []
  conexion.alCambiar((e) => estados.push(e))
  return { conexion, fabrica, estados }
}

describe('la conexión de tiempo real', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('manda el token como subprotocolo aire.jwt, no en la URL', () => {
    const { conexion, fabrica } = preparar()

    conexion.conectar()

    expect(fabrica.creados).toHaveLength(1)
    expect(fabrica.ultimo().protocolos).toEqual(['aire.jwt', 'el-token'])
    expect(fabrica.ultimo().url).toBe('ws://localhost/ws')
    expect(fabrica.ultimo().url).not.toContain('el-token')
  })

  it('pasa de conectando a conectada, y no abre dos conexiones', () => {
    const { conexion, fabrica, estados } = preparar()

    conexion.conectar()
    conexion.conectar()
    fabrica.ultimo().abrir()

    expect(estados).toEqual(['conectando', 'conectada'])
    expect(fabrica.creados).toHaveLength(1)
  })

  it('reparte cada evento a quien se suscribió a su nombre', () => {
    const { conexion, fabrica } = preparar()
    const rondas = vi.fn()
    const regimenes = vi.fn()
    conexion.suscribir('entrenamiento.ronda', rondas)
    const dejar = conexion.suscribir('estacion.regimen', regimenes)
    conexion.conectar()
    fabrica.ultimo().abrir()

    const ronda = { id: 'e-1', iter: 1, inercia: 73075.5, ms_ronda: 0.61, ms_fusion: 0.003, ms_por_nodo: { n1: 0.11 } }
    fabrica.ultimo().recibir({ evento: 'entrenamiento.ronda', datos: ronda })

    expect(rondas).toHaveBeenCalledExactlyOnceWith(ronda)
    expect(regimenes).not.toHaveBeenCalled()

    dejar()
    fabrica.ultimo().recibir({ evento: 'estacion.regimen', datos: { estacion: 'CA-AYAS-01' } })
    expect(regimenes).not.toHaveBeenCalled()
  })

  it('ignora las tramas que no son de este protocolo', () => {
    const { conexion, fabrica } = preparar()
    const rondas = vi.fn()
    conexion.suscribir('entrenamiento.ronda', rondas)
    conexion.conectar()
    fabrica.ultimo().abrir()

    fabrica.ultimo().recibir('esto no es JSON')
    fabrica.ultimo().recibir({ sin: 'evento' })
    fabrica.ultimo().recibir({ evento: 'otro.evento', datos: {} })

    expect(rondas).not.toHaveBeenCalled()
    expect(conexion.estado).toBe('conectada')
  })

  it('un manejador que falla no deja sin evento a los demás', () => {
    const { conexion, fabrica } = preparar()
    const consola = vi.spyOn(console, 'error').mockImplementation(() => {})
    const segundo = vi.fn()
    conexion.suscribir('entrenamiento.fin', () => {
      throw new Error('se rompió')
    })
    conexion.suscribir('entrenamiento.fin', segundo)
    conexion.conectar()
    fabrica.ultimo().abrir()

    fabrica.ultimo().recibir({ evento: 'entrenamiento.fin', datos: { id: 'e-1', error: 'cayó un nodo' } })

    expect(segundo).toHaveBeenCalledOnce()
    expect(consola).toHaveBeenCalledOnce()
  })

  it('si se cae, reconecta con una espera que crece: 1 s, 2 s, 4 s… hasta 30 s', () => {
    const { conexion, fabrica, estados } = preparar()
    conexion.conectar()
    fabrica.ultimo().abrir()

    const esperas = [1_000, 2_000, 4_000, 8_000, 16_000, 30_000, 30_000]
    esperas.forEach((espera, i) => {
      fabrica.ultimo().cerrarDesdeElServidor(1006)
      expect(conexion.estado).toBe('reconectando')

      vi.advanceTimersByTime(espera - 1)
      expect(fabrica.creados).toHaveLength(i + 1) // todavía no reintentó

      vi.advanceTimersByTime(1)
      expect(fabrica.creados).toHaveLength(i + 2)
    })
    expect(estados).not.toContain('cortada')
  })

  it('al volver a conectar, la espera empieza de nuevo y las suscripciones siguen', () => {
    const { conexion, fabrica } = preparar()
    const regimenes = vi.fn()
    conexion.suscribir('estacion.regimen', regimenes)
    conexion.conectar()
    fabrica.ultimo().abrir()
    fabrica.ultimo().cerrarDesdeElServidor(1006)
    vi.advanceTimersByTime(1_000)
    fabrica.ultimo().cerrarDesdeElServidor(1006)
    vi.advanceTimersByTime(2_000)

    fabrica.ultimo().abrir()
    expect(conexion.estado).toBe('conectada')
    fabrica.ultimo().recibir({ evento: 'estacion.regimen', datos: { estacion: 'CA-AYAS-01' } })
    expect(regimenes).toHaveBeenCalledOnce()

    fabrica.ultimo().cerrarDesdeElServidor(1006)
    vi.advanceTimersByTime(1_000)
    expect(fabrica.creados).toHaveLength(4)
  })

  it('no reintenta tras un cierre 1008 por token vencido: queda «vencida»', () => {
    const { conexion, fabrica, estados } = preparar()
    conexion.conectar()
    fabrica.ultimo().abrir()

    fabrica.ultimo().cerrarDesdeElServidor(1008, 'el token expiró: pide otro y vuelve a conectar')
    vi.advanceTimersByTime(10 * HORA)

    expect(conexion.estado).toBe('vencida')
    expect(estados).toEqual(['conectando', 'conectada', 'vencida'])
    expect(fabrica.creados).toHaveLength(1)
  })

  it('un 1008 justo antes de vencer cuenta como token vencido aunque no lo diga', () => {
    const { conexion, fabrica } = preparar({ expiraEn: 30_000 })
    conexion.conectar()
    fabrica.ultimo().abrir()

    fabrica.ultimo().cerrarDesdeElServidor(1008)

    expect(conexion.estado).toBe('vencida')
  })

  it('tampoco reintenta tras un 1008 por cliente lento: queda «cortada» hasta que la persona reconecte', () => {
    const { conexion, fabrica } = preparar()
    conexion.conectar()
    fabrica.ultimo().abrir()

    fabrica.ultimo().cerrarDesdeElServidor(1008, 'cliente lento: no lee al ritmo de los eventos')
    vi.advanceTimersByTime(10 * 60_000)

    expect(conexion.estado).toBe('cortada')
    expect(fabrica.creados).toHaveLength(1)

    conexion.reconectar()
    expect(fabrica.creados).toHaveLength(2)
    fabrica.ultimo().abrir()
    expect(conexion.estado).toBe('conectada')
  })

  it('no reintenta pasada la hora de expira', () => {
    const { conexion, fabrica } = preparar({ expiraEn: 10_000 })
    conexion.conectar()
    fabrica.ultimo().abrir()

    vi.advanceTimersByTime(10_000) // el token venció con la conexión abierta
    fabrica.ultimo().cerrarDesdeElServidor(1006)
    vi.advanceTimersByTime(HORA)

    expect(conexion.estado).toBe('vencida')
    expect(fabrica.creados).toHaveLength(1)
  })

  it('un reintento que cae después de expira no se hace', () => {
    const { conexion, fabrica } = preparar({ expiraEn: 1_500 })
    conexion.conectar()
    fabrica.ultimo().abrir()
    fabrica.ultimo().cerrarDesdeElServidor(1006) // reintenta en 1 s: todavía no venció
    vi.advanceTimersByTime(1_000)
    expect(fabrica.creados).toHaveLength(2)

    fabrica.ultimo().cerrarDesdeElServidor(1006) // reintentaría en 2 s: ya venció
    vi.advanceTimersByTime(2_000)

    expect(conexion.estado).toBe('vencida')
    expect(fabrica.creados).toHaveLength(2)
  })

  it('con el token ya vencido ni siquiera abre', () => {
    const { conexion, fabrica } = preparar({ expiraEn: -1 })

    conexion.conectar()

    expect(conexion.estado).toBe('vencida')
    expect(fabrica.creados).toHaveLength(0)
  })

  it('cerrar a propósito cierra con 1000 y no reconecta', () => {
    const { conexion, fabrica } = preparar()
    conexion.conectar()
    fabrica.ultimo().abrir()
    const enchufe = fabrica.ultimo()

    conexion.cerrar()
    enchufe.cerrarDesdeElServidor(1000)
    vi.advanceTimersByTime(HORA)

    expect(enchufe.cerradoCon?.codigo).toBe(1000)
    expect(conexion.estado).toBe('inactiva')
    expect(fabrica.creados).toHaveLength(1)
  })

  it('cerrar cancela un reintento en espera', () => {
    const { conexion, fabrica } = preparar()
    conexion.conectar()
    fabrica.ultimo().cerrarDesdeElServidor(1006)

    conexion.cerrar()
    vi.advanceTimersByTime(HORA)

    expect(fabrica.creados).toHaveLength(1)
  })

  describe('cuando el apretón de manos falla', () => {
    // El navegador da 1006 tanto si la API está apagada como si respondió 401
    // porque ya no acepta el token (cambió su secreto): no se distinguen.
    function fallarApretones(fabrica: ReturnType<typeof fabricaDeEnchufes>, veces: number) {
      for (let i = 0; i < veces; i++) {
        fabrica.ultimo().cerrarDesdeElServidor(1006) // se cerró sin llegar a abrirse
        vi.advanceTimersByTime(30_000)
      }
    }

    it('al tercer fallo seguido avisa, para que se compruebe la sesión; y sigue reintentando', () => {
      const alFallarApreton = vi.fn()
      const { conexion, fabrica } = preparar({ alFallarApreton })
      conexion.conectar()

      fallarApretones(fabrica, 2)
      expect(alFallarApreton).not.toHaveBeenCalled()

      fallarApretones(fabrica, 1)
      expect(alFallarApreton).toHaveBeenCalledExactlyOnceWith(3)
      expect(conexion.estado).toBe('reconectando')
      expect(fabrica.creados).toHaveLength(4)
    })

    it('si sigue fallando, vuelve a avisar cada tres', () => {
      const alFallarApreton = vi.fn()
      const { conexion, fabrica } = preparar({ alFallarApreton })
      conexion.conectar()

      fallarApretones(fabrica, 7)

      expect(alFallarApreton.mock.calls).toEqual([[3], [6]])
    })

    it('también cuenta si ya había conectado antes: la API pudo reiniciarse con otro secreto', () => {
      const alFallarApreton = vi.fn()
      const { conexion, fabrica } = preparar({ alFallarApreton })
      conexion.conectar()
      fabrica.ultimo().abrir()
      fabrica.ultimo().cerrarDesdeElServidor(1006) // una caída con la conexión abierta: no es un apretón fallido
      vi.advanceTimersByTime(30_000)
      expect(alFallarApreton).not.toHaveBeenCalled()

      fallarApretones(fabrica, 3)

      expect(alFallarApreton).toHaveBeenCalledExactlyOnceWith(3)
    })

    it('una conexión que abre pone la cuenta en cero', () => {
      const alFallarApreton = vi.fn()
      const { conexion, fabrica } = preparar({ alFallarApreton })
      conexion.conectar()
      fallarApretones(fabrica, 2)

      fabrica.ultimo().abrir()
      fabrica.ultimo().cerrarDesdeElServidor(1006)
      vi.advanceTimersByTime(30_000)
      fallarApretones(fabrica, 2)

      expect(alFallarApreton).not.toHaveBeenCalled()
    })

    it('si el aviso lanza, la conexión sigue reintentando', () => {
      vi.spyOn(console, 'error').mockImplementation(() => {})
      const { conexion, fabrica } = preparar({
        alFallarApreton: () => {
          throw new Error('se rompió')
        },
      })
      conexion.conectar()

      fallarApretones(fabrica, 3)

      expect(conexion.estado).toBe('reconectando')
      expect(fabrica.creados).toHaveLength(4)
    })
  })

  it('arma la URL de /ws en el mismo origen: ws con http y wss con https', () => {
    expect(urlWS({ protocol: 'http:', host: 'localhost:8081' })).toBe('ws://localhost:8081/ws')
    expect(urlWS({ protocol: 'https:', host: 'aire.example.com' })).toBe('wss://aire.example.com/ws')
  })
})
