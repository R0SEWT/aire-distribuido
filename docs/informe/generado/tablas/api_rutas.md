<!-- Generado por tablas.py desde docs/api/openapi.yaml. No editar a mano. -->

::: {#tbl-api-rutas tbl-colwidths="[34,48,18]"}
| Método y ruta | Qué hace | Acceso |
|:---|:---|:---|
| `POST /api/auth/token` | Obtiene un JWT HS256 para un usuario de demostración | libre |
| `GET /api/salud` | Consulta la API, sus almacenes, el pool y el modelo activo | libre |
| `POST /api/predicciones` | Asigna una lectura al régimen más cercano del modelo activo | usuario o admin |
| `GET /api/estaciones` | Lista las estaciones almacenadas en MongoDB | usuario o admin |
| `GET /api/estaciones/{id}/regimen` | Obtiene de Redis el último régimen registrado de una estación | usuario o admin |
| `GET /api/regimenes` | Obtiene de Redis el último estado de todas las estaciones que tienen uno | usuario o admin |
| `GET /api/modelos/activo` | Consulta el modelo activo y sus regímenes en unidades originales | usuario o admin |
| `GET /api/cluster/nodos` | Lista nodos, workers, latencia y disponibilidad del pool | admin |
| `GET /api/cluster/eventos` | Lista los eventos del clúster, del más reciente al más antiguo | admin |
| `POST /api/entrenamientos` | Inicia un entrenamiento distribuido de forma asíncrona | admin |
| `GET /api/entrenamientos` | Lista el historial de entrenamientos, sin las rondas | admin |
| `GET /api/entrenamientos/{id}` | Consulta un entrenamiento con sus rondas o el error de aborto | admin |
| `GET /api/metricas` | Consulta pedidos, percentiles de latencia, CPU y memoria de la API y estado del clúster | admin |
| `GET /openapi.yaml` | Sirve este contrato OpenAPI sin transformación | libre |
| `GET /docs` | Muestra Swagger UI con el contrato de /openapi.yaml | libre |
| `GET /ws` | Abre un WebSocket con el estado de las estaciones y los eventos del entrenamiento | usuario o admin |

Rutas de la API: el contrato OpenAPI, que la API sirve en `/openapi.yaml` y Swagger UI muestra en `/docs`
:::

*Nota.* Fuente: `docs/api/openapi.yaml` (el texto de «Qué hace» es el `summary` de cada operación; el acceso, su `x-roles`), comprobado contra las rutas que registra `api/servidor.go`. «libre»: sin token. En las demás hace falta un JWT con alguno de esos roles; sin token la respuesta es 401 y con otro rol, 403.
