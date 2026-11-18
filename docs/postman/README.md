# Pruebas de la API — J2

La colección prueba el contrato de J1 (PR #55, commit `96afa6f`). Los nombres
`id`, `abortado` y algunas respuestas 404/503 son propuestas pendientes de acuerdo
con la implementación. El alcance del límite de 100 ms también está pendiente de
respuesta en la PR #51. Estos archivos no implementan la API coordinadora.

## Verificación automática sin API real

Desde la raíz de la copia de J2:

```powershell
node scripts/probar-postman-simulado.cjs
```

El comando inicia su propio servidor en un puerto local libre, ejecuta Newman y
lo detiene al terminar. No utiliza ni modifica un simulador que esté abierto en
otra terminal. Comprueba las 13 operaciones HTTP de J1, la corrida completa y cinco
controles negativos: tipo de respuesta incorrecto, código HTTP incorrecto, permisos
incorrectos, entrenamiento que no termina y uso del simulador con el entorno marcado
como real. Los fallos de estos
controles se introducen deliberadamente para verificar que la colección los detecta.
Si un control deja de detectarse, el comando termina con error.
También ejecuta `Base HTTP` con credenciales que contienen comillas, barra y salto
de línea, para comprobar que el cuerpo de autenticación se construye como JSON válido.

Para regenerar el resumen sin cuerpos, credenciales ni tokens:

```powershell
node scripts/probar-postman-simulado.cjs `
  --informe docs/postman/evidencias/validacion-simulada.json
```

En Windows reconoce Newman instalado globalmente en la ubicación estándar de npm
del usuario. También admite Newman local o disponible mediante `NODE_PATH`.

## Prueba interactiva con el simulador

Requiere Node.js y Newman. Instalar Newman con `npm install -g newman@6.2.3`.
En PowerShell se pueden usar `npm.cmd` y `newman.cmd` sin cambiar la política de
ejecución de scripts.

Desde la raíz, en una terminal:

```powershell
node scripts/api-simulada-j2.cjs
```

En otra terminal:

```powershell
newman.cmd run `
  docs/postman/aire-distribuido.postman_collection.json `
  -e docs/postman/simulado.postman_environment.json `
  --delay-request 50
```

El simulador solo escucha en `127.0.0.1:18080`. Se detiene con Ctrl+C. Opcionalmente
se puede elegir otro puerto con `--puerto 18081` y ajustar `base_url` en el entorno.
No conecta con MongoDB, Redis, nodos ni servicios externos.

Todas sus respuestas incluyen `X-Aire-Simulado: true`. Las comprobaciones se
identifican como `[SIMULADO]` y la prueba de latencia queda omitida expresamente.
El entorno real falla si recibe una respuesta con esa marca y el simulado falla
si recibe una respuesta sin ella. Esto evita confundir las respuestas de este
simulador con las del entorno real configurado.

La predicción devuelve un ejemplo fijo del contrato; no ejecuta K-Means. Los nodos,
las métricas y los tiempos son ilustrativos. El entrenamiento termina después de
consultar su estado dos veces. La autenticación solo reconoce tokens de simulación;
no implementa JWT HS256. `/openapi.yaml` devuelve un documento mínimo de simulación.
Una corrida exitosa aquí valida las peticiones y sus comprobaciones, no la API de
Rody, la precisión del modelo, la tolerancia distribuida ni el objetivo de 100 ms.

## Organización y precondiciones

- `Base HTTP`: salud, autenticación usuario/admin, predicción, consultas, documentación
  y errores de validación/autenticación/autorización. No inicia entrenamientos.
- `Entrenamiento`: requiere nodos libres y un entorno de prueba dedicado. Prueba
  parámetros inválidos, insuficiencia de nodos, aceptación 202 y consulta hasta
  terminar. El máximo de consultas es configurable; no espera indefinidamente.
- `Conflicto controlado`: requiere que el primer entrenamiento siga activo al
  enviar el segundo pedido. El simulador lo garantiza. Para la API real se necesita
  un escenario controlado acordado con Rody; una corrida que termine antes del
  segundo pedido no permite exigir 409 de forma reproducible. Incluye su cierre.

La colección obtiene los tokens al iniciar cada carpeta y conserva tokens e IDs
solo como variables locales de la ejecución. No se exportan credenciales reales
ni tokens en los archivos versionados. El entorno simulado contiene exclusivamente
credenciales de demostración que solo reconoce este simulador.
Las peticiones de token construyen su cuerpo con `JSON.stringify`; las credenciales
del entorno no se interpolan directamente en texto JSON.

## API real

Copiar `local.postman_environment.json` a `mi.local.postman_environment.json`
(ignorado por git) y configurar allí `base_url` y las credenciales de demostración
que indique Rody. No versionar esa copia ni exportar tokens. Obtener primero sus
instrucciones para levantar la API.

Para ejecutar solamente las pruebas que no inician entrenamientos:

```powershell
newman.cmd run `
  docs/postman/aire-distribuido.postman_collection.json `
  -e docs/postman/mi.local.postman_environment.json `
  --folder "Base HTTP"
```

La corrida completa también necesita las precondiciones de entrenamiento y conflicto.
`nodos_insuficientes` debe ser mayor que el número de nodos libres del entorno;
`nodos_entrenamiento` debe poder asignarse. Las consultas del trabajo usan el ID
devuelto por el servidor, no un ID fijo de la spec.

Para probar el ciclo de entrenamiento con un límite global de ejecución:

```powershell
newman.cmd run `
  docs/postman/aire-distribuido.postman_collection.json `
  -e docs/postman/mi.local.postman_environment.json `
  --folder "Entrenamiento" `
  --delay-request 100 `
  --timeout-request 5000 `
  --timeout 120000
```

El grupo de conflicto se ejecuta por separado cuando se pueda garantizar un
entrenamiento activo. Un 202 en el segundo pedido no se reclasifica como 409 ni
se oculta como éxito: se revisa si se cumplió la precondición del escenario.

Se validan código HTTP, tipo de contenido y esquema de las respuestas. Los tokens
deben corresponder al rol solicitado. Los casos negativos esperan un código concreto,
sin aceptar alternativamente éxito o error. En la API real, el tiempo se obtiene de
`pm.response.responseTime`, no del campo `ms` del JSON. El entorno propone inicialmente
`latencia_ambito: todos` y `limite_ms: 100`, siguiendo §9. Si Rody acuerda limitarlo a
predicción, se cambia explícitamente a `prediccion`. Las pruebas restantes se muestran
como omitidas; no se modifica silenciosamente el criterio.

WebSocket requiere una prueba específica de conexión y eventos. No se cuenta `/ws`
como probado por esta colección HTTP. Los 30 registros y el Runner con CSV son J3;
las métricas de carga p50/p95/p99 y las capturas de la API real corresponden a J5.

J2 queda aceptado cuando la colección se ejecute contra la API real, con el contrato
y las precondiciones acordados, y se publique la evidencia en su PR. Las corridas
simuladas deben conservar esa identificación.

## Evidencia disponible

- [Salida de Newman compartida por Julio](evidencias/newman-simulado-terminal.txt):
  texto de la terminal conservado byte a byte, sin reconstruir capturas. Registra
  la primera corrida de 40 peticiones y cero fallos, anterior al control adicional
  de credenciales especiales. Sus tiempos pertenecen al simulador local.
- [Resumen reproducible](evidencias/validacion-simulada.json): 13 operaciones HTTP,
  38 peticiones definidas, 40 ejecutadas por la consulta repetida de los trabajos,
  214 comprobaciones aprobadas y 40 de latencia omitidas. Incluye cinco controles
  negativos detectados, el control positivo de credenciales especiales y la huella
  SHA-256 de la colección actual.

El SHA-256 de la salida de terminal aportada es
`8428b0dcd42cadb1ace4441c6dea333c843bc40a003bbc4e77d03059de2fbbdd`.
El resumen registra `pendienteAPIReal: true`. No hay una medición real de latencia
ni un resultado de WebSocket en estos archivos.

## Check de GitHub

`.github/workflows/postman.yml` ejecuta esta verificación en los PR con Node.js 26
y Newman 6.2.3. No necesita credenciales, bases de datos ni una API desplegada.
El check es exclusivamente simulado y mantiene las pruebas de latencia omitidas.
El workflow Go existente se conserva. El resultado del nuevo check en GitHub
se podrá comprobar después de publicar la rama y abrir el PR.
