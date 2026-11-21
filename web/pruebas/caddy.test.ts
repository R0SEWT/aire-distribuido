// @vitest-environment node

// Las cabeceras de seguridad de web/Caddyfile (spec del frontend, §10). Lo que
// de verdad hace Caddy se comprueba contra el Compose levantado; esto cuida dos
// cosas que se rompen sin avisar: que la política de la UI no se afloje, y que
// la de /docs siga correspondiendo al script en línea que sirve la API.

import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const leer = (ruta: string) => readFileSync(fileURLToPath(new URL(ruta, import.meta.url)), 'utf8')

const caddyfile = leer('../Caddyfile')

function politicas(): Record<string, string>[] {
  return [...caddyfile.matchAll(/^\s*header Content-Security-Policy "(.*)"$/gm)].map((m) =>
    Object.fromEntries(
      m[1]!.split(';').map((directiva) => {
        const [nombre, ...valores] = directiva.trim().split(/\s+/)
        return [nombre!, valores.join(' ')]
      }),
    ),
  )
}

/** La política de /docs es la que nombra el CDN de Swagger; la otra es la de la UI. */
const esDeDocs = (p: Record<string, string>) => (p['script-src'] ?? '').includes('cdn.jsdelivr.net')
const politicaDeDocs = () => politicas().find(esDeDocs)!
const politicaDeLaUi = () => politicas().find((p) => !esDeDocs(p))!

/** El contenido de un bloque `nombre {` … `}` del Caddyfile, con sus llaves anidadas. */
function bloque(encabezado: string): string {
  const inicio = caddyfile.indexOf(`${encabezado} {`)
  expect(inicio, `el Caddyfile tiene un bloque «${encabezado}»`).toBeGreaterThanOrEqual(0)
  let nivel = 0
  for (let i = caddyfile.indexOf('{', inicio); i < caddyfile.length; i++) {
    if (caddyfile[i] === '{') nivel++
    if (caddyfile[i] === '}' && --nivel === 0) return caddyfile.slice(inicio, i + 1)
  }
  throw new Error(`el bloque «${encabezado}» no cierra`)
}

describe('las cabeceras de web/Caddyfile', () => {
  it('hay dos políticas: una para /docs y otra para la UI, que usan los dos bloques que sirven archivos', () => {
    expect(politicas()).toHaveLength(2)
    expect(politicas().filter(esDeDocs)).toHaveLength(1)
    expect(caddyfile.match(/^\s*import politica_ui$/gm)).toHaveLength(2)
  })

  it('la UI solo carga scripts, estilos y fuentes de su propio origen, sin nada en línea', () => {
    const ui = politicaDeLaUi()

    expect(ui).toMatchObject({
      'default-src': "'self'",
      'script-src': "'self'",
      'style-src': "'self'",
      'font-src': "'self'",
      'connect-src': "'self'",
      'frame-ancestors': "'none'",
      'object-src': "'none'",
    })
    expect(Object.values(ui).join(' ')).not.toMatch(/unsafe|https?:|\*/)
  })

  it('manda X-Content-Type-Options y Referrer-Policy en todas las respuestas', () => {
    expect(caddyfile).toMatch(/^\s*X-Content-Type-Options nosniff$/m)
    expect(caddyfile).toMatch(/^\s*Referrer-Policy no-referrer$/m)
  })

  it('pasa a la API /api, /ws, /docs y /openapi.yaml', () => {
    expect(caddyfile).toMatch(/@api path \/api\/\* \/ws \/openapi\.yaml/)
    expect(caddyfile).toMatch(/handle \/docs \{/)
    expect(caddyfile.match(/reverse_proxy api:8080/g)).toHaveLength(2)
  })

  it('un archivo de /assets o de /datos existe o es un 404: no vuelve a index.html', () => {
    const archivos = bloque('handle @archivos')

    expect(caddyfile).toMatch(/@archivos path \/assets\/\* \/datos\/\*/)
    expect(archivos).toContain('file_server')
    expect(archivos).not.toContain('try_files')
  })

  it('solo sale como inmutable un archivo de /assets que existe; index.html, nunca', () => {
    const archivos = bloque('handle @archivos')
    const compilado = bloque('@compilado')
    const pagina = caddyfile.slice(caddyfile.lastIndexOf('\thandle {'))

    expect(compilado).toMatch(/path \/assets\/\*/)
    expect(compilado).toMatch(/^\s*file$/m)
    expect(archivos).toMatch(/header @compilado Cache-Control "public, max-age=31536000, immutable"/)
    expect(caddyfile.match(/immutable/g)).toHaveLength(1)
    expect(pagina).toContain('try_files {path} /index.html')
    expect(pagina).toMatch(/^\s*header Cache-Control "no-cache"$/m)
    expect(pagina).not.toContain('immutable')
  })

  it('el token no llega al log: ni Sec-WebSocket-Protocol ni la consulta ?token=', () => {
    const registro = bloque('log default')

    // Caddy escribe los encabezados con la forma canónica de Go.
    expect(registro).toMatch(/request>headers>Sec-Websocket-Protocol delete/)
    expect(registro).toMatch(/request>uri query \{\s*replace token REDACTED\s*\}/)
  })

  it('la política de /docs admite el script en línea de Swagger por su hash, que es el de api/rutas.go', () => {
    const pagina = leer('../../api/rutas.go')
    const enLinea = [...pagina.matchAll(/<script>([\s\S]*?)<\/script>/g)]
    expect(enLinea, 'la página de /docs tiene un solo script en línea').toHaveLength(1)
    const hash = createHash('sha256').update(enLinea[0]![1]!).digest('base64')

    const docs = politicaDeDocs()

    // Si falla, la página de /docs cambió: el hash nuevo va en web/Caddyfile.
    expect(docs['script-src']).toContain(`'sha256-${hash}'`)
    expect(docs['script-src']).not.toContain('unsafe-inline')
  })

  it('la política de /docs solo admite la versión de Swagger que la página carga', () => {
    const pagina = leer('../../api/rutas.go')
    const origenes = new Set(
      [...pagina.matchAll(/(?:src|href)="(https:\/\/[^"]+\/)[^"/]+"/g)].map((m) => m[1]!),
    )
    expect(origenes.size).toBe(1)
    const [cdn] = [...origenes]

    const docs = politicaDeDocs()

    expect(docs['script-src']).toContain(cdn)
    expect(docs['style-src']).toContain(cdn)
    expect(docs['frame-ancestors']).toBe("'none'")
  })
})
