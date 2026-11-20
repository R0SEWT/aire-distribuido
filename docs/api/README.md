# Contrato de la API

[`openapi.yaml`](openapi.yaml) es el contrato de la API de la PC4: las 13 rutas de la spec §6.1,
con los ejemplos de §6.5. La API lo sirve tal cual en `/openapi.yaml` y Swagger UI lo muestra en
`/docs`. Se revisa importándolo en [Swagger Editor](https://editor.swagger.io/) (**File → Import
file**).

![Contrato cargado en Swagger Editor](../informe/capturas/pc4/swagger-editor.png)

## Acceso

Libres: token, salud, `/openapi.yaml` y `/docs`. Con rol `usuario` o `admin`: predicciones,
estaciones, modelo activo y `/ws`. Solo `admin`: nodos del clúster, entrenamientos y métricas;
un `usuario` recibe 403. `x-roles` deja el rol escrito en cada operación.

## Lo que el contrato fija y la spec no decía

- **Alerta e INCA**: la predicción trae el régimen (el tipo de aire, con `excede_eca` del régimen) y,
  aparte, la media de 24 h de la estación (`media_24h`, en memoria en la API). `alerta` es que esa
  media supere el ECA (PM2.5 > 50 o PM10 > 100 µg/m³) e `inca` es su banda del INCA; las dos son
  normas de 24 h y se informan por separado. Hacen falta 18 lecturas en la ventana. La ventana se
  calcula respecto de la lectura que llega (su `fecha_hora`, truncada a la hora), así que sirven los
  datos históricos y las lecturas desordenadas; por estación se conservan las 72 lecturas horarias
  más cercanas a la última recibida. La estación tiene que estar en el catálogo, y se rechazan un PM mayor
  de 10 000 µg/m³ y una fecha más de 1 hora adelante del reloj del servidor. El régimen por estación guarda la `fecha_hora` de su lectura y no
  retrocede con una lectura más antigua. En el gold, el
  régimen alto solo acierta el 35 % de las horas sobre el ECA de 24 h: por eso la alerta no sale
  del régimen.
- **Estaciones**: `id` es el `_id` de la colección (el código OEFA) y la ficha trae todos los campos
  de §7.1; un dato que falte en la fuente llega como `null`.
- **Entrenamientos**: tres estados, `en_curso`, `terminado` y `abortado`. El listado no trae las
  rondas; se piden por id. `paro` es `tolerancia` o `max_iter`, los motivos de `kmeans`.
- **Errores**: siempre `{"error": "…"}`. 404 cuando no existe el recurso (para el régimen de una
  estación, también cuando todavía no tiene predicciones), 503 cuando no hay modelo activo o no
  responde el almacén de la ruta.
- **WebSocket**: cada trama es `{"evento": "entrenamiento.ronda" | "entrenamiento.fin", "datos":
  {…}}`, con los datos de §6.4.
