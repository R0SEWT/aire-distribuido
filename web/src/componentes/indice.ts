// Los componentes base (spec §8). Una pantalla los importa de aquí.

export { Advertencia, Aviso } from './Aviso'
export { Banda } from './Banda'
export { Barrera } from './Barrera'
export { BANDAS, estadoDeBanda, type BandaINCA, type EstadoBanda, type InfoBanda } from './bandas'
export { Cifra } from './Cifra'
export { Dato } from './Dato'
export { Desplegable } from './Desplegable'
export { Encabezado } from './Encabezado'
export { Estado, EstadoDeError, type TipoEstado } from './Estado'
export { Etiqueta } from './Etiqueta'
export {
  compacto,
  decimalesFrenteA,
  diaCorto,
  diaDeInstante,
  diasEnLlano,
  duracion,
  enumerar,
  fechaConSegundos,
  fechaDia,
  fechaHora,
  hora,
  horaConSegundos,
  horaDeLima,
  lapso,
  milisegundos,
  nombrePropio,
  numero,
  porciento,
  proporcion,
} from './formato'
export { Frase } from './Frase'
export { Pildoras, type Opcion } from './Pildoras'
export { Tabla, type Columna } from './Tabla'
export { Tarjeta } from './Tarjeta'
export type { Tono } from './tonos'
