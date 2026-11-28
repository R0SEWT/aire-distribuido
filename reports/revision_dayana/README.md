# Reproducción de EDA y auditoría de limpieza

Evidencias de la ejecución de los notebooks 01 y 03 en el Mac de Dayana sobre el CSV completo
de OEFA. Los resultados originales del equipo siguen en el directorio padre.

- `ejecucion.json`: entorno, rama, commit base, hashes de entradas y fuentes, y resultado de la comparación.
- `auditoria.json`: nueva ejecución del notebook 03 sobre las salidas de revisión.

Los hashes de las fuentes corresponden a los archivos de esta PR. Los centroides iniciales
son los publicados en el proyecto. Los benchmarks son referencias históricas: no se midieron
nuevamente. El notebook 03 no cotejó el SHA del silver con el benchmark porque la ruta de entrada
era distinta; la comparación directa de los SHA de silver y gold está en `ejecucion.json`.

Para repetir la ejecución, ver [`docs/limpieza.md`](../../docs/limpieza.md).
Las copias ejecutadas de los notebooks, con figuras y logs, están localmente en
la carpeta `ejecutados/` de la raíz `data/revision-<fecha>-<pid>/`; los CSV e intermedios no se
versionan.
