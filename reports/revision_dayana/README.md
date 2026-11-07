# Reproducción de EDA y auditoría de limpieza

Evidencias de la ejecución de los notebooks 01 y 03 en el Mac de Dayana sobre el CSV completo
de OEFA. Los resultados originales del equipo siguen en el directorio padre.

- `ejecucion.json`: entorno, rama, commit base, hashes de entradas y fuentes, y resultado de la comparación.
- `limpieza.json` y `estandarizacion.json`: nueva corrida del cargador Go.
- `eda_criba.md`: nueva ejecución del notebook 01, con matriz de completitud por estación y parámetro.
- `auditoria.md` y `auditoria.json`: nueva ejecución del notebook 03 sobre las salidas de revisión.

El estado del árbol tenía cambios de documentación y del notebook 01; los hashes de las
fuentes registran exactamente qué se ejecutó. Los centroides iniciales son los publicados
en el proyecto. Los benchmarks son referencias históricas: no se midieron nuevamente.

Para repetir la ejecución, ver [`docs/limpieza.md`](../../docs/limpieza.md).
Las copias ejecutadas de los notebooks, con figuras y logs, están localmente en
`data/bronze/revision-dayana/ejecutados/`; los CSV e intermedios no se versionan.
