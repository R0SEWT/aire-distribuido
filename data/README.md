# Datos

Nada de esta carpeta se versiona, salvo este README y `sample/`.

Las dos fuentes son del Portal de Datos Abiertos del Perú, que es lo que exige el enunciado.

| Fuente | Qué es | Volumen | Dónde |
|--------|--------|---------|-------|
| OEFA — Vigilancia y Seguimiento ambiental en la calidad del aire | Monitoreo continuo de 34 estaciones, con contaminantes y parámetros meteorológicos. Fuente principal: es la que pasa el millón de filas. | CSV de 1.3 GB (1,317,949,839 bytes, modificado 2024-07-17). Filas: por contar. | [dataset](https://www.datosabiertos.gob.pe/dataset/vigilancia-y-seguimiento-ambiental-en-la-calidad-del-aire-organismo-de-evaluaci%C3%B3n-y) · [CSV](https://www.datosabiertos.gob.pe/sites/default/files/1a_Vigilancia%20y%20Seguimiento%20ambiental%20en%20la%20calidad%20del%20aire.csv) · [diccionario](https://www.datosabiertos.gob.pe/sites/default/files/1c_Formato_DiccionarioDatos_Vigilancia%20y%20Seguimiento%20ambiental%20en%20la%20calidad%20del%20aire.xlsx) |
| SENAMHI — Monitoreo de los contaminantes del aire en Lima Metropolitana | Red automática de Lima: PM10, PM2.5, NO2 por estación, fecha y hora. Complementa a OEFA en Lima. | 577,794 registros, CSV de ~68 MB (por confirmar al bajarlo). | [dataset](https://www.datosabiertos.gob.pe/dataset/monitoreo-de-los-contaminantes-del-aire-en-lima-metropolitana-servicio-nacional-de) |

`sample/` lleva una muestra pequeña de cada fuente para las pruebas y el CI.
