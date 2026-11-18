#!/usr/bin/env node
'use strict';

// Herramienta local de J2: ejemplos controlados, sin K-Means, JWT ni bases de datos.
const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const fixtures = JSON.parse(fs.readFileSync(
  path.join(__dirname, '../docs/postman/simulador-ejemplos.json'), 'utf8'));

const copiar = value => structuredClone(value);
const error = message => ({ error: message });

function crearServidor({ fallo = '', credenciales = {
  usuario: { usuario: 'demo', clave: 'demo' },
  admin: { usuario: 'admin-demo', clave: 'demo' },
} } = {}) {
  let secuencia = 0;
  let activo = null;
  const trabajos = new Map();
  const roles = new Map([['simulado-usuario', 'usuario'], ['simulado-admin', 'admin']]);

  return http.createServer(async (req, res) => {
    const enviar = (status, body, type = 'application/json') => {
      res.writeHead(status, { 'Content-Type': `${type}; charset=utf-8`,
        'X-Aire-Simulado': 'true', 'Cache-Control': 'no-store' });
      res.end(type === 'application/json' ? JSON.stringify(body) : body);
    };
    try {
      const ruta = new URL(req.url, 'http://127.0.0.1').pathname;
      const verbo = req.method;
      let cuerpo = {};
      if (verbo === 'POST') {
        const partes = [];
        let bytes = 0;
        for await (const parte of req) {
          bytes += parte.length;
          if (bytes > 65536) { enviar(413, error('pedido demasiado grande')); return; }
          partes.push(parte);
        }
        try { cuerpo = JSON.parse(Buffer.concat(partes).toString('utf8')); }
        catch { enviar(400, error('JSON inválido')); return; }
        if (!cuerpo || Array.isArray(cuerpo) || typeof cuerpo !== 'object') {
          enviar(400, error('se requiere un objeto JSON')); return;
        }
      }
      if (ruta === '/api/auth/token' && verbo === 'POST') {
        const cuenta = Object.entries(credenciales).find(([, c]) =>
          c.usuario === cuerpo.usuario && c.clave === cuerpo.clave);
        if (!cuenta) {
          enviar(401, error('usuario o clave incorrectos')); return;
        }
        const [rol] = cuenta;
        enviar(200, { token: `simulado-${rol}`, rol,
          expira: new Date(Date.now() + 3600000).toISOString() }); return;
      }
      if (ruta === '/api/salud' && verbo === 'GET') {
        enviar(200, fixtures.salud); return;
      }
      if (ruta === '/openapi.yaml' && verbo === 'GET') {
        enviar(200, 'openapi: 3.0.3\ninfo:\n  title: API SIMULADA J2\n  version: 0.0.0\npaths: {}\n',
          'application/yaml'); return;
      }
      if (ruta === '/docs' && verbo === 'GET') {
        enviar(200, '<!doctype html><html lang="es"><title>SIMULADO J2</title>'
          + '<body>Documentación simulada. No ejecuta Swagger UI.</body></html>', 'text/html'); return;
      }
      const bearer = /^Bearer (.+)$/.exec(req.headers.authorization || '');
      const rol = bearer ? roles.get(bearer[1]) : undefined;
      if (!rol) { enviar(401, error('token ausente, inválido o expirado')); return; }
      const admin = ruta === '/api/cluster/nodos' || ruta === '/api/metricas'
        || ruta === '/api/entrenamientos' || ruta.startsWith('/api/entrenamientos/');
      if (admin && rol !== 'admin' && fallo !== 'permisos') {
        enviar(403, error('se requiere el rol admin')); return;
      }
      if (ruta === '/api/predicciones' && verbo === 'POST') {
        const fecha = cuerpo.fecha_hora;
        const valida = typeof cuerpo.estacion === 'string' && cuerpo.estacion.length > 0
          && typeof fecha === 'string'
          && /T.*(?:Z|[+-]\d{2}:\d{2})$/.test(fecha) && Number.isFinite(Date.parse(fecha))
          && ['pm2_5', 'pm10'].every(c => typeof cuerpo[c] === 'number'
            && Number.isFinite(cuerpo[c]) && cuerpo[c] >= 0);
        if (!valida) { enviar(400, error('lectura inválida')); return; }
        const prediccion = copiar(fixtures.prediccion);
        prediccion.estacion = cuerpo.estacion;
        if (fallo === 'tipo-prediccion') prediccion.regimen = 'incorrecto';
        enviar(fallo === 'codigo-prediccion' ? 201 : 200, prediccion); return;
      }
      if (ruta === '/api/estaciones' && verbo === 'GET') {
        enviar(200, [{ id: fixtures.prediccion.estacion }]); return;
      }
      if (ruta.startsWith('/api/estaciones/') && ruta.endsWith('/regimen') && verbo === 'GET') {
        if (ruta !== `/api/estaciones/${fixtures.prediccion.estacion}/regimen`) {
          enviar(404, error('recurso no encontrado')); return;
        }
        enviar(200, fixtures.regimen); return;
      }
      if (ruta === '/api/modelos/activo' && verbo === 'GET') {
        enviar(200, fixtures.modelo); return;
      }
      if (ruta === '/api/cluster/nodos' && verbo === 'GET') {
        enviar(200, fixtures.nodos); return;
      }
      if (ruta === '/api/metricas' && verbo === 'GET') {
        const metricas = copiar(fixtures.metricas);
        metricas.ultimo_entrenamiento = secuencia ? `simulado-e-${secuencia}` : null;
        enviar(200, metricas); return;
      }
      if (ruta === '/api/entrenamientos' && verbo === 'POST') {
        if (!Number.isInteger(cuerpo.nodos) || cuerpo.nodos < 1
          || !Number.isInteger(cuerpo.replicas) || cuerpo.replicas < 1
          || !Number.isSafeInteger(cuerpo.semilla)) {
          enviar(400, error('parámetros de entrenamiento inválidos')); return;
        }
        if (activo) { enviar(409, error(`ya hay un entrenamiento en curso: ${activo}`)); return; }
        if (cuerpo.nodos > 4) { enviar(422, error('hay 4 nodos libres en el pool')); return; }
        const id = `simulado-e-${++secuencia}`;
        const trabajo = copiar(fixtures.entrenamiento);
        Object.assign(trabajo, { id, estado: 'en_curso', replicas: cuerpo.replicas,
          nodos: fixtures.entrenamiento.nodos.slice(0, cuerpo.nodos) });
        trabajos.set(id, { trabajo, consultas: 0 });
        activo = id;
        enviar(202, { id, estado: 'en_curso' }); return;
      }
      if (ruta === '/api/entrenamientos' && verbo === 'GET') {
        enviar(200, [...trabajos.values()].map(t => t.trabajo)); return;
      }
      if (ruta.startsWith('/api/entrenamientos/') && verbo === 'GET') {
        const id = decodeURIComponent(ruta.slice('/api/entrenamientos/'.length));
        const registro = trabajos.get(id);
        if (!registro) { enviar(404, error('recurso no encontrado')); return; }
        if (++registro.consultas >= 2 && fallo !== 'entrenamiento-sin-fin') {
          registro.trabajo.estado = 'terminado';
          if (activo === id) activo = null;
        }
        enviar(200, registro.trabajo); return;
      }
      enviar(404, error('ruta no simulada'));
    } catch {
      if (!res.headersSent) enviar(500, error('error del simulador'));
      else res.destroy();
    }
  });
}

if (require.main === module) {
  const args = process.argv.slice(2);
  let puerto = 18080;
  for (let i = 0; i < args.length; i += 2) {
    if (args[i] !== '--puerto' || !/^\d+$/.test(args[i + 1] || '')) {
      throw new Error('Uso: node scripts/api-simulada-j2.cjs [--puerto 18080]');
    }
    puerto = Number(args[i + 1]);
  }
  if (puerto < 1 || puerto > 65535) throw new Error('Puerto fuera de rango');
  const servidor = crearServidor();
  servidor.on('error', err => { console.error(`Simulador: ${err.code}`); process.exitCode = 1; });
  servidor.listen(puerto, '127.0.0.1', () => {
    console.log(`API SIMULADA J2 en http://127.0.0.1:${puerto}`);
    console.log('No acredita la API real ni la latencia. Detener con Ctrl+C.');
  });
}

module.exports = { crearServidor };
