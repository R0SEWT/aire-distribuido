#!/usr/bin/env node
'use strict';

// Verifica la colección con Newman y un servidor efímero exclusivamente local.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { createRequire } = require('node:module');
const { crearServidor } = require('./api-simulada-j2.cjs');

const raiz = path.resolve(__dirname, '..');
const carpeta = path.join(raiz, 'docs/postman');
const archivoColeccion = path.join(carpeta, 'aire-distribuido.postman_collection.json');
const textoColeccion = fs.readFileSync(archivoColeccion, 'utf8');
const coleccion = JSON.parse(textoColeccion);
const entorno = JSON.parse(fs.readFileSync(path.join(carpeta, 'simulado.postman_environment.json'), 'utf8'));

function cargarNewman() {
  let cargar = require;
  try { require.resolve('newman'); }
  catch (err) {
    if (err.code !== 'MODULE_NOT_FOUND') throw err;
    if (!process.env.APPDATA) {
      throw new Error('Newman no encontrado. Instalarlo localmente o configurar NODE_PATH con npm root -g.');
    }
    cargar = createRequire(path.join(process.env.APPDATA, 'npm', 'package.json'));
  }
  return { ejecutar: cargar('newman'), version: cargar('newman/package.json').version };
}

function comprobarCobertura() {
  const esperadas = new Set([
    'POST /api/auth/token', 'GET /api/salud', 'POST /api/predicciones',
    'GET /api/estaciones', 'GET /api/estaciones/{id}/regimen', 'GET /api/modelos/activo',
    'GET /api/cluster/nodos', 'POST /api/entrenamientos', 'GET /api/entrenamientos',
    'GET /api/entrenamientos/{id}', 'GET /api/metricas', 'GET /openapi.yaml', 'GET /docs',
  ]);
  const actuales = new Set();
  let peticiones = 0;
  for (const grupo of coleccion.item) {
    for (const item of grupo.item) {
      let ruta = item.request.url.replace('{{base_url}}', '');
      if (/^\/api\/estaciones\/.+\/regimen$/.test(ruta)) ruta = '/api/estaciones/{id}/regimen';
      if (/^\/api\/entrenamientos\/.+$/.test(ruta)) ruta = '/api/entrenamientos/{id}';
      actuales.add(`${item.request.method} ${ruta}`);
      peticiones++;
    }
  }
  const faltantes = [...esperadas].filter(ruta => !actuales.has(ruta));
  const extras = [...actuales].filter(ruta => !esperadas.has(ruta));
  if (faltantes.length || extras.length) {
    throw new Error(`Cobertura distinta de J1: faltan ${faltantes.join(', ')}; sobran ${extras.join(', ')}`);
  }
  return { operacionesHTTP: actuales.size, peticionesDefinidas: peticiones };
}

async function ejecutar(newman, { fallo = '', comoReal = false, grupo, caracteresEspeciales = false } = {}) {
  const marca = String.fromCharCode(34, 92, 10);
  const credenciales = caracteresEspeciales ? {
    usuario: { usuario: 'demo' + marca, clave: 'demo' + marca },
    admin: { usuario: 'admin' + marca, clave: 'demo' + marca },
  } : undefined;
  const servidor = crearServidor({ fallo, credenciales });
  await new Promise((resolve, reject) => {
    servidor.once('error', reject);
    servidor.listen(0, '127.0.0.1', resolve);
  });
  const configuracion = structuredClone(entorno);
  configuracion.values.find(v => v.key === 'base_url').value =
    `http://127.0.0.1:${servidor.address().port}`;
  if (comoReal) configuracion.values.find(v => v.key === 'modo').value = 'real';
  if (fallo === 'entrenamiento-sin-fin') {
    configuracion.values.find(v => v.key === 'max_consultas_entrenamiento').value = '3';
  }
  if (caracteresEspeciales) {
    for (const [rol, cuenta] of Object.entries(credenciales)) {
      configuracion.values.find(v => v.key === `${rol}_usuario`).value = cuenta.usuario;
      configuracion.values.find(v => v.key === `${rol}_clave`).value = cuenta.clave;
    }
  }
  try {
    return await new Promise((resolve, reject) => {
      newman.run({
        collection: structuredClone(coleccion), environment: configuracion,
        reporters: [], delayRequest: 5, timeoutRequest: 3000, timeout: 30000,
        ...(grupo ? { folder: grupo } : {}),
      }, (err, resultado) => err ? reject(err) : resolve(resultado));
    });
  } finally {
    await new Promise(resolve => servidor.close(resolve));
  }
}

async function main() {
  const args = process.argv.slice(2);
  if (args.length && (args.length !== 2 || args[0] !== '--informe' || !args[1])) {
    throw new Error('Uso: node scripts/probar-postman-simulado.cjs [--informe archivo.json]');
  }
  const cobertura = comprobarCobertura();
  const { ejecutar: newman, version } = cargarNewman();
  console.log('SIMULADO: verifica la colección; no acredita la API real ni su latencia.');

  const base = await ejecutar(newman);
  if (base.run.failures.length) {
    // Solo nombres de pruebas: nunca guardar cuerpos, credenciales ni tokens.
    throw new Error(`Falló la corrida base: ${base.run.failures.map(f => f.error.test || f.error.name).join(', ')}`);
  }
  const aserciones = base.run.executions.flatMap(e => e.assertions || []);
  const omitidas = aserciones.filter(a => a.skipped);
  const latencias = omitidas.filter(a => a.assertion.includes('latencia'));
  if (latencias.length !== base.run.stats.requests.total) {
    throw new Error('La simulación debe omitir la latencia en todas las respuestas.');
  }

  const especiales = await ejecutar(newman, { caracteresEspeciales: true, grupo: 'Base HTTP' });
  if (especiales.run.failures.length) {
    throw new Error('La colección no conserva las credenciales con caracteres especiales.');
  }

  const controles = [];
  for (const [fallo, prueba, grupo = 'Base HTTP'] of [
    ['tipo-prediccion', 'esquema de respuesta'],
    ['codigo-prediccion', 'codigo HTTP 200'],
    ['permisos', 'codigo HTTP 403'],
    ['entrenamiento-sin-fin', 'entrenamiento terminado dentro del limite de consultas', 'Entrenamiento'],
  ]) {
    const resultado = await ejecutar(newman, { fallo, grupo });
    const detectados = resultado.run.failures.filter(f => f.error.test?.includes(prueba)).length;
    if (!detectados) throw new Error(`La colección no detectó el fallo deliberado: ${fallo}`);
    controles.push({ falloDeliberado: fallo, detectado: true });
  }
  const origen = await ejecutar(newman, { comoReal: true, grupo: 'Base HTTP' });
  const erroresOrigen = origen.run.failures.filter(f => f.error.test?.includes('origen del entorno')).length;
  const peticionesBase = coleccion.item.find(grupo => grupo.name === 'Base HTTP').item.length;
  if (erroresOrigen !== peticionesBase) {
    throw new Error('El entorno real debe rechazar cada respuesta de este simulador.');
  }
  controles.push({ falloDeliberado: 'simulador presentado como API real', detectado: true });

  const informe = {
    tipo: 'SIMULADO; no acredita API real ni latencia',
    contrato: 'J1, PR #55, commit 96afa6f; pendiente de revisión e integración',
    node: process.version, newman: version, ...cobertura,
    peticionesEjecutadas: base.run.stats.requests.total,
    comprobacionesRegistradas: aserciones.length,
    comprobacionesAprobadas: aserciones.length - omitidas.length,
    comprobacionesOmitidas: omitidas.length,
    latenciasOmitidas: latencias.length,
    fallos: base.run.failures.length,
    controlesNegativos: controles,
    controlesPositivos: [{ caso: 'credenciales con comillas, barra y salto de línea', aprobado: true }],
    coleccionSHA256: crypto.createHash('sha256').update(textoColeccion).digest('hex'),
    pendienteAPIReal: true,
  };
  if (args.length) {
    const destino = path.resolve(args[1]);
    fs.mkdirSync(path.dirname(destino), { recursive: true });
    fs.writeFileSync(destino, JSON.stringify(informe, null, 2) + '\n', 'utf8');
  }
  console.log(JSON.stringify(informe, null, 2));
}

main().catch(err => { console.error(err.message); process.exitCode = 1; });
