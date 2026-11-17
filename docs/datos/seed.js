// Seed idempotente de las colecciones e índices de PC4.
// mongosh lo ejecuta automáticamente al inicializar un volumen vacío y también
// se puede volver a ejecutar manualmente. La ficha versionada es la única
// fuente del catálogo; este script no completa ubicaciones por inferencia.

const fs = require("fs");

const databaseName = process.env.MONGO_INITDB_DATABASE || "aire_distribuido";
const estacionesPath = process.env.ESTACIONES_CSV || "/seed/estaciones_ficha.csv";
const database = db.getSiblingDB(databaseName);

const colecciones = [
  "modelos",
  "entrenamientos",
  "predicciones",
  "eventos_cluster",
  "estaciones",
];

const existentes = new Set(database.getCollectionNames());
for (const nombre of colecciones) {
  if (!existentes.has(nombre)) {
    database.createCollection(nombre);
  }
}

database.modelos.createIndex({ activo: 1 }, { name: "activo" });
database.modelos.createIndex({ creado: 1 }, { name: "creado" });
database.entrenamientos.createIndex({ inicio: 1 }, { name: "inicio" });
database.predicciones.createIndex(
  { estacion: 1, ts: 1 },
  { name: "estacion_ts" },
);
database.eventos_cluster.createIndex({ ts: 1 }, { name: "ts" });

function parsearCSV(texto) {
  const filas = [];
  let fila = [];
  let campo = "";
  let entreComillas = false;

  for (let i = 0; i < texto.length; i += 1) {
    const caracter = texto[i];
    if (entreComillas) {
      if (caracter === '"' && texto[i + 1] === '"') {
        campo += '"';
        i += 1;
      } else if (caracter === '"') {
        entreComillas = false;
      } else {
        campo += caracter;
      }
    } else if (caracter === '"') {
      entreComillas = true;
    } else if (caracter === ",") {
      fila.push(campo);
      campo = "";
    } else if (caracter === "\n") {
      fila.push(campo.replace(/\r$/, ""));
      filas.push(fila);
      fila = [];
      campo = "";
    } else {
      campo += caracter;
    }
  }

  if (entreComillas) {
    throw new Error("CSV inválido: campo entrecomillado sin cerrar");
  }
  if (campo !== "" || fila.length > 0) {
    fila.push(campo.replace(/\r$/, ""));
    filas.push(fila);
  }
  return filas;
}

function textoONull(valor) {
  const limpio = valor.trim();
  return limpio === "" || limpio === "-" || limpio.toUpperCase() === "NA"
    ? null
    : limpio;
}

function numeroONull(valor, campo, estacion) {
  const texto = textoONull(valor);
  if (texto === null) {
    return null;
  }
  const numero = Number(texto);
  if (!Number.isFinite(numero)) {
    throw new Error(`${estacion}: ${campo} no es numérico: ${JSON.stringify(valor)}`);
  }
  return numero;
}

const filas = parsearCSV(fs.readFileSync(estacionesPath, "utf8"));
if (filas.length < 2) {
  throw new Error(`la ficha de estaciones está vacía: ${estacionesPath}`);
}

const cabecera = filas[0];
const indice = Object.fromEntries(cabecera.map((nombre, i) => [nombre, i]));
const requeridas = [
  "estacion",
  "objeto_fiscalizado",
  "tipo_actividad",
  "departamento",
  "provincia",
  "distrito",
  "lat",
  "lon",
];
for (const nombre of requeridas) {
  if (indice[nombre] === undefined) {
    throw new Error(`falta la columna ${nombre} en ${estacionesPath}`);
  }
}

const operaciones = [];
const ids = new Set();
for (const fila of filas.slice(1)) {
  if (fila.length === 1 && fila[0] === "") {
    continue;
  }
  if (fila.length !== cabecera.length) {
    throw new Error(
      `fila ${operaciones.length + 2}: ${fila.length} campos; se esperaban ${cabecera.length}`,
    );
  }

  const estacion = textoONull(fila[indice.estacion]);
  if (estacion === null) {
    throw new Error(`fila ${operaciones.length + 2}: estación vacía`);
  }
  if (ids.has(estacion)) {
    throw new Error(`estación duplicada en la ficha: ${estacion}`);
  }
  ids.add(estacion);

  const documento = {
    _id: estacion,
    objeto_fiscalizado: textoONull(fila[indice.objeto_fiscalizado]),
    tipo_actividad: textoONull(fila[indice.tipo_actividad]),
    departamento: textoONull(fila[indice.departamento]),
    provincia: textoONull(fila[indice.provincia]),
    distrito: textoONull(fila[indice.distrito]),
    lat: numeroONull(fila[indice.lat], "lat", estacion),
    lon: numeroONull(fila[indice.lon], "lon", estacion),
  };

  operaciones.push({
    replaceOne: {
      filter: { _id: estacion },
      replacement: documento,
      upsert: true,
    },
  });
}

if (operaciones.length === 0) {
  throw new Error("la ficha no contiene estaciones");
}

database.estaciones.bulkWrite(operaciones, { ordered: true });

printjson({
  database: databaseName,
  estaciones_en_fuente: operaciones.length,
  estaciones_en_mongo: database.estaciones.countDocuments({}),
  colecciones: database.getCollectionNames().sort(),
});
