# Mensaje para el grupo de WhatsApp (borrador)

**No enviado.** Lo aprueba Rody antes de mandarlo (skill `wpp-grupo`). Son tres mensajes: el general y uno
con los textos de cada uno, para que los copien a su celular. Antes de enviarlo: poner el enlace de la
carpeta de Drive donde dice `[enlace de Drive]` y, si la rama ya está en GitHub, el enlace al guion.

---

## Mensaje 1 (para los dos)

> Chicos, el video del TB2 ya está armado salvo nuestras voces y caras 🎬. Las animaciones, la interfaz y
> la terminal se graban solas; nos toca a cada uno grabar unos clips cortos leyendo el guion a la cámara.
> El video total dura 5:30 (el máximo es 6:00) y salimos los tres.
>
> *Dayana*: 3 clips → `03-dayana` (0:35, el dato y el cargador), `06-dayana` (0:25, MongoDB y Redis) y
> `10-dayana` (0:40, impacto social).
> *Julio*: 4 clips → `05-julio` (0:35, la API), `07-julio` (0:30, demo: ingreso y tiempo real),
> `08-julio` (0:20, demo: simulación) y `11-julio` (0:20, pruebas y despliegue).
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
> *03-dayana · El dato de OEFA y el cargador concurrente (0:35)*
> Soy Dayana. El dato de OEFA es un CSV de 1,3 gigabytes con 2 millones 883 mil filas, una por parámetro
> y por hora. El cargador lo parte en bloques de 64 megabytes y manda los índices por un canal a un pool
> de 12 workers. Cada worker limpia su bloque en su propia memoria, sin candados, y un recolector fusiona
> los parciales en orden de bloque. Por eso el resultado es el mismo con cualquier número de workers: el
> archivo entero en 1,3 segundos.
>
> *06-dayana · MongoDB y Redis (0:25)*
> Guardamos en dos niveles. MongoDB es el persistente: predicciones, entrenamientos, modelos, estaciones
> y eventos del clúster. Redis guarda el último estado de cada estación y el modelo activo. La predicción
> no espera a las bases: va a un canal con búfer y una goroutine por base escribe en lotes. En la prueba
> de carga entraron 80 800 escrituras sin perder ninguna.
>
> *10-dayana · Impacto social (0:40)*
> ¿Y para qué sirve? De 64 748 horas con media de 24 horas, 1 184 superaron el ECA. En los 30 episodios
> en que la media cruzó la norma, la primera lectura sobre el umbral apareció una mediana de 20 horas
> antes: es una medida sobre los mismos datos, no un pronóstico. Cada estación muestra la población de su
> distrito, sin sumarla. Y una advertencia: solo 17 de las 33 estaciones miden por hora, y una zona sin
> estación no es una zona con aire limpio.

## Mensaje 3 (Julio)

> *Julio: tus 4 textos*
>
> *05-julio · La API coordinadora (0:35)*
> Soy Julio. La API está hecha con net/http, sin frameworks, y su contrato está en OpenAPI. Los pedidos
> llevan un token JWT con su rol, usuario o admin. Una lectura entra por POST: la API le asigna el
> régimen, actualiza la media de 24 horas de la estación, decide la alerta y la avisa por WebSocket. Con
> 80 mil pedidos y hasta 64 clientes a la vez, el percentil 99 fue de 3 milisegundos, muy debajo de los
> 100.
>
> *07-julio · Demo: ingreso, escenario y tiempo real (0:30)*
> Ahora la interfaz, de punta a punta. Entro con usuario y clave, y la API me devuelve el token. El panel
> empieza vacío, porque el sistema no recibe lecturas solo. Cargo el escenario: 48 horas reales de
> diciembre de 2021. Cada lectura es un POST, y el mapa se pinta a medida que llegan los eventos por el
> WebSocket. Las estaciones en alerta suben primero.
>
> *08-julio · Demo: simular un día (0:20)*
> En Simulación elijo una estación, un día y la velocidad. Las lecturas se mandan una por una: se ve cómo
> se llena la ventana de 24 horas, cuándo aparece la media, con 18 lecturas, y en qué hora se enciende la
> alerta.
>
> *11-julio · Pruebas y despliegue (0:20)*
> Todo se prueba con go test y el detector de carreras, guion race, también en la integración continua de
> GitHub. Docker Compose levanta la interfaz, la API, MongoDB, Redis y los nodos, que se escalan con
> scale. Y está desplegado en aire punto rosewt punto dev.
