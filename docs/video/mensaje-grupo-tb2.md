# Mensaje para el grupo de WhatsApp (borrador)

**No enviado.** Lo aprueba Rody antes de mandarlo (skill `wpp-grupo`). Son tres mensajes: el general y uno
con los textos de cada uno, para que los copien a su celular. Antes de enviarlo: poner el enlace de la
carpeta de Drive donde dice `[enlace de Drive]` y, si la rama ya está en GitHub, el enlace al guion.

---

## Mensaje 1 (para los dos)

> Chicos, el video del TB2 ya está armado salvo nuestras voces y caras 🎬. Las animaciones, la interfaz y
> la terminal se graban solas; nos toca a cada uno grabar unos clips cortos leyendo el guion a la cámara.
> El video total dura 5:37 (el máximo es 6:00) y salimos los tres.
>
> *Dayana*: 3 clips → `03-dayana` (0:41, el dato, el cargador y tu reproducción en la Mac), `06-dayana` (0:27, MongoDB y Redis) y
> `10-dayana` (0:40, impacto social).
> *Julio*: 4 clips → `05-julio` (0:34, la API), `07-julio` (0:26, demo: ingreso y tiempo real),
> `08-julio` (0:17, demo: simulación) y `11-julio` (0:24, pruebas, tu corrida en Windows y despliegue).
> Yo grabo los míos (apertura, arquitectura, clúster, entrenamiento y cierre).
>
> Cómo grabar:
> • Cámara *horizontal*, cara y hombros, luz de frente, lugar en silencio. Laptop o celular, 720p o 1080p.
> • Un clip por bloque, de una sola vez: 1 segundo en silencio, el texto, 1 segundo en silencio. Sin editar.
> • Ritmo de conversación, más o menos la duración que dice. Si se pasan por más de 5 s, repítanlo más suelto.
> • Pueden decirlo con sus palabras, pero las *cifras tal cual*.
> • Nombre del archivo: `03-dayana.mp4`, `05-julio.mov`, etc.
> • Súbanlos a [enlace de Drive] o mándenlos por aquí *como Documento* (como video, WhatsApp los comprime).
>
> ⏰ *Plazo: sábado 28 a las 12:00*. En la tarde lo monto y se los paso para que lo revisen antes de
> entregarlo el domingo.
>
> Abajo les mando el texto de cada uno. Revísenlo: si algo no lo dirían así, me avisan y lo cambiamos.
> La guía completa está en `docs/video/grabar-tu-parte.md` del repo.

## Mensaje 2 (Dayana)

> *Dayana: tus 3 textos*
>
> *03-dayana · El dato de OEFA y el cargador concurrente (0:41)*
> Soy Dayana. El dato de OEFA es un CSV de 1,3 gigabytes con 2 millones 883 mil filas. El cargador lo
> parte en bloques de 64 megabytes y manda los índices por un canal a un pool de workers. Cada worker
> limpia su bloque en su propia memoria, sin candados, y un recolector fusiona los parciales en orden de
> bloque. La mitad de las filas se descarta, sobre todo ceros que en OEFA significan «sin dato». El
> resultado es el mismo con cualquier número de workers, y el archivo entero se carga en menos de un
> segundo y medio. Y lo reproduje en una Mac: el mismo SHA-256 que en Linux.
>
> *06-dayana · MongoDB y Redis (0:27)*
> Guardamos en dos niveles. MongoDB, el persistente, tiene predicciones, entrenamientos, modelos,
> estaciones y eventos del clúster; Redis, el último estado de cada estación y el modelo activo. La
> predicción no espera a las bases: va a un canal con búfer y una goroutine por base escribe en lotes. En
> la prueba de carga llegaron las 80 800 predicciones, contando el calentamiento, sin descartar ninguna.
>
> *10-dayana · Impacto social (0:40)*
> ¿Y para qué sirve? De 64 748 horas-estación con media de 24 horas, 1 184 superaron el ECA; la misma hora
> en dos estaciones cuenta dos veces. Cada estación muestra la población de su distrito como contexto, sin
> sumarla. En los 30 episodios sobre el ECA, la lectura horaria ya había pasado el umbral una mediana de
> 20 horas antes; pero se prende y se apaga, y seguida llevaba apenas una hora. Es una descripción, no un
> pronóstico. Y solo 17 de las 33 estaciones miden PM por hora: donde no se mide, no se puede afirmar que
> el aire esté limpio.

## Mensaje 3 (Julio)

> *Julio: tus 4 textos*
>
> *05-julio · La API coordinadora (0:34)*
> Soy Julio. La API está hecha con net/http, sin frameworks, con su contrato en OpenAPI y un token JWT por
> rol. Una lectura entra por POST y no pasa por el clúster: la API usa el modelo activo en memoria, asigna
> el régimen, actualiza la media de 24 horas, decide la alerta y manda el nuevo estado por WebSocket. Con
> 80 mil pedidos y hasta 64 clientes a la vez, el percentil 99 no pasó de 6 milisegundos.
>
> *07-julio · Demo: ingreso, escenario y tiempo real (0:26)*
> Ahora la interfaz, de punta a punta. Entro con usuario y clave y la API me da el token. Cargo el
> escenario: 48 horas reales de diciembre de 2021, cada lectura un POST. En la API, un hub recibe por
> canales las altas, las bajas y los eventos, y los reparte a cada cliente del WebSocket: por eso el mapa
> se pinta solo.
>
> *08-julio · Demo: simular un día (0:17)*
> En Simulación elijo una estación y un día. Las lecturas se mandan una por una: se ve cómo se
> llena la ventana de 24 horas, cuándo aparece la media, con 18 lecturas, y en qué hora se enciende la
> alerta.
>
> *11-julio · Pruebas y despliegue (0:24)*
> Las pruebas de Go corren con el detector de carreras, guion race, también en la integración continua. Y
> en Windows comprobé que el cargador da el mismo SHA-256 que en Linux. Docker Compose levanta la
> interfaz, la API, las bases y los nodos, que se escalan con scale. Y está desplegado en aire punto
> rosewt punto dev.

