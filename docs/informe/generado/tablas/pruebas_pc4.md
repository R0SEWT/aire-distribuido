<!-- Generado por tablas.py desde los archivos _test.go de cluster/, protocolo/, api/, almacen/, modelo/, ventana/ y cmd/. No editar a mano. -->

::: {#tbl-pruebas-pc4 tbl-colwidths="[18,58,11,13]"}
| Paquete | Qué comprueban | Pruebas | De integración |
|:---|:---|---:|---:|
| `protocolo` | Mensajes y codificación, la barrera de una ronda con su plazo, la fusión en orden de nodo y el parcial rezagado | 25 | — |
| `cluster` | El nodo, la espera de nodos, el reparto de rangos, Lloyd sobre el clúster (bit a bit con un nodo, con tolerancia con varios, un nodo caído) y el pool | 55 | — |
| `cmd/distribuido` | El coordinador de línea de comandos de punta a punta, con nodos reales por TCP | 11 | — |
| `cmd/nodo` | El comando del nodo | 1 | — |
| `cmd/smoke` | El smoke test del clúster | 3 | — |
| `modelo` | Sembrar el modelo desde `reports/` y predecir: casos fijos del gold y el ejemplo del contrato | 14 | — |
| `ventana` | La media de 24 horas: ventana, lecturas desordenadas, mínimo de lecturas, ECA e INCA, concurrencia | 14 | — |
| `api` | Rutas y errores, JWT, métricas, el escritor por colas, el coordinador de entrenamientos, el hub y el WebSocket, CORS | 114 | — |
| `almacen` | La ficha de estaciones y los documentos frente a sus ejemplos | 5 | — |
| `almacen/memoria` | El doble en memoria que usan las pruebas y el CI | 13 | — |
| `almacen/mongo` | MongoDB real: colecciones, índices del seed, un solo modelo activo, el ciclo de un entrenamiento | 4 | 9 |
| `almacen/redis` | Redis real: el régimen que no retrocede, escritores concurrentes, modelo activo y punto de control | — | 14 |
| `cmd/api` | El arranque, el apagado en orden, los almacenes reales y el coordinador con nodos reales | 16 | 2 |

Pruebas automáticas de los paquetes del Entregable 2, contadas hoy
:::

*Nota.* Fuente: las funciones `Test…` de los archivos `_test.go` de cada paquete, contadas del código en cada render. Se cuentan del código actual, así que incluyen las que el TB2 sumó a estos paquetes (en `api/` y `cmd/api/`). «De integración» son las que llevan la etiqueta de compilación `integracion`: corren contra el MongoDB y el Redis del Compose (`go test -race -tags integracion ./almacen/... ./cmd/api/`), no en la integración continua, que usa el doble en memoria.
