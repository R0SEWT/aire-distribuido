// @vitest-environment node

// El contraste de los tokens de color (spec §8), en modo normal y en modo
// proyector, con la fórmula de las WCAG 2. Texto: 4.5:1 o más. Borde de un
// control donde se escribe: 3:1 o más. Y el proyector, que existe para verse
// mejor, nunca por debajo del modo normal.

import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const leer = (ruta: string) => readFileSync(fileURLToPath(new URL(ruta, import.meta.url)), 'utf8')

function tokens(css: string): Record<string, string> {
  return Object.fromEntries([...css.matchAll(/--color-([a-z-]+):\s*(#[0-9a-f]{6})\b/gi)].map((m) => [m[1]!, m[2]!]))
}

const normal = tokens(leer('../src/estilos/tokens.css'))
// El proyector solo redefine algunos tokens: el resto son los del modo normal.
const proyector = { ...normal, ...tokens(leer('../src/estilos/proyector.css')) }

function luminancia(hex: string): number {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = Number.parseInt(hex.slice(i, i + 2), 16) / 255
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * r! + 0.7152 * g! + 0.0722 * b!
}

function contraste(a: string, b: string): number {
  const [clara, oscura] = [luminancia(a), luminancia(b)].sort((x, y) => y - x)
  return (clara! + 0.05) / (oscura! + 0.05)
}

const CAPAS = ['fondo', 'superficie', 'relieve'] as const
// Los colores que se usan como texto sobre cualquiera de las tres capas.
const TEXTOS = ['texto', 'suave', 'tenue', 'acento-claro', 'aviso', 'falla'] as const

const pares: [string, string, number][] = [
  ...TEXTOS.flatMap((texto) => CAPAS.map((capa): [string, string, number] => [texto, capa, 4.5])),
  // El botón principal: su texto contra su fondo.
  ['boton-texto', 'boton', 4.5],
  // El borde de un campo contra el fondo del campo y contra la tarjeta que lo contiene.
  ['borde-control', 'fondo', 3],
  ['borde-control', 'superficie', 3],
  // El botón se distingue de la tarjeta donde está.
  ['boton', 'superficie', 3],
]

describe('el contraste de los tokens de color', () => {
  it('lee los tokens de los dos modos', () => {
    for (const token of [...CAPAS, ...TEXTOS, 'boton', 'boton-texto', 'borde-control']) {
      expect(normal[token], `--color-${token} en tokens.css`).toMatch(/^#[0-9a-f]{6}$/i)
    }
    expect(Object.keys(tokens(leer('../src/estilos/proyector.css'))).length).toBeGreaterThan(10)
  })

  it.each(pares)('modo normal: %s sobre %s da %f:1 o más', (delante, detras, minimo) => {
    expect(contraste(normal[delante]!, normal[detras]!)).toBeGreaterThanOrEqual(minimo)
  })

  it.each(pares)('modo proyector: %s sobre %s da %f:1 o más', (delante, detras, minimo) => {
    expect(contraste(proyector[delante]!, proyector[detras]!)).toBeGreaterThanOrEqual(minimo)
  })

  it.each(pares)('el proyector no baja el contraste de %s sobre %s', (delante, detras) => {
    expect(contraste(proyector[delante]!, proyector[detras]!)).toBeGreaterThanOrEqual(
      contraste(normal[delante]!, normal[detras]!),
    )
  })

  it('la fórmula da los valores conocidos', () => {
    expect(contraste('#ffffff', '#000000')).toBeCloseTo(21, 5)
    expect(contraste('#777777', '#ffffff')).toBeCloseTo(4.48, 2)
  })
})
