<!-- Generado por tablas.py desde los _test.go del TB2 y los *.test.ts(x) de web/. No editar a mano. -->

::: {#tbl-pruebas-tb2 tbl-colwidths="[10,26,46,18]"}
| Lado | Dónde | Qué comprueban | Pruebas |
|:---|:---|:---|---:|
| Go | `api/regimenes_test.go` | `GET /api/regimenes`: el estado de todas las estaciones en un pedido | 5 |
| Go | `api/avisos_test.go` | El evento `estacion.regimen` por WebSocket: solo la lectura más reciente, agrupado por estación | 12 |
| Go | `api/eventos_test.go` | `GET /api/cluster/eventos`: los «logs» del clúster | 4 |
| Go | `api/cors_test.go` | CORS: orígenes permitidos, preflight, `Vary` y el origen del WebSocket | 3 |
| Go | `api/invitado_test.go` | `POST /api/auth/invitado`: el token de la cuenta designada, rol `usuario`, 404 sin cuenta | 4 |
| Go | `cmd/api/invitado_test.go` | El arranque con `API_INVITADO`: no arranca si la cuenta no existe o es de administrador | 4 |
| Interfaz | `web/src/sesion/` | El token en `sessionStorage`, el rol, las guardas de ruta y los dos ingresos | 7 en 2 archivos |
| Interfaz | `web/src/api/` | El cliente tipado, la traducción de errores, el WebSocket (reconexión, 1008) y la caché en vivo | 98 en 6 archivos |
| Interfaz | `web/src/pantallas/` | Las siete pantallas: estados, frases, orden por gravedad, simulador, reproducción de rondas, CPU | 324 en 19 archivos |
| Interfaz | `web/src/componentes/` | Los componentes base y los formatos (números, fechas en hora de Lima) | 60 en 2 archivos |
| Interfaz | `web/src/graficos/` | Los gráficos de D3: mapa, curva, inercia y tiempos por nodo | 26 en 4 archivos |
| Interfaz | `web/src/App.test.tsx` | Las rutas de la aplicación (`App.test.tsx`) | 58 en 1 archivo |
| Interfaz | `web/pruebas/` | Fuera del navegador: el `Caddyfile` (CSP y su hash) y el contraste de los tokens de color | 14 en 2 archivos |

Pruebas automáticas del Entregable 3: las de la API que agregó el TB2 y las de la interfaz
:::

*Nota.* Fuente: las funciones `Test…` de cada archivo de Go y los casos `it(…)` y `test(…)` de los archivos de prueba de la interfaz, contados del código en cada render. Las de Go corren con `go test -race` y las de la interfaz con Vitest y Testing Library (`pnpm test`), en la integración continua de GitHub en cada cambio.
