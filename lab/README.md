# El clúster en el laboratorio

Dos corridas sobre los mismos nodos: el **smoke test**, que comprueba que cada equipo llega al
coordinador por TCP, habla el protocolo de `protocolo/` y responde a tiempo, y el **K-Means
distribuido** de la PC4, que reparte el gold entre los equipos y corre Lloyd con una barrera de red
por iteración (`docs/diseno.md`, sección 4).

- `cmd/nodo`: el nodo. Llama al coordinador y se presenta con `HOLA`. Responde `PING` con `PONG`;
  con `CARGAR` lee su rango del gold que tiene al lado (después de comprobar que su sha256 es la
  huella pedida); con cada `ITERAR` calcula sumas, conteos e inercia de su rango con todos sus
  núcleos y responde `PARCIAL`; `FIN` cierra la corrida. Después vuelve a llamar solo, así que se
  lanza una vez por equipo y sirve para varias corridas, de los dos tipos.
- `cmd/smoke`: coordinador de prueba. Espera N nodos, manda rondas de `PING` a todos a la vez con
  `protocolo.Latido` y cierra con `FIN`.
- `cmd/distribuido`: coordinador del K-Means. Espera N nodos, les reparte el gold en rangos
  contiguos iguales, corre Lloyd y escribe el modelo, los tiempos de cada ronda y la diferencia con
  `reports/kmeans.json`.

En los equipos del laboratorio no hace falta instalar nada ni ser administrador: `nodo.exe` es un
solo ejecutable y solo abre conexiones de salida, que el Firewall de Windows permite.

## Pasos

**1. Armar el USB** desde la raíz del repo. Compila `nodo.exe` para Windows, copia el gold
(`data/gold/oefa_features.csv`, unos 5 MB) y escribe la IP de la laptop en `coordinador.txt`:

```bash
lab/armar-usb.sh /run/media/$USER/<USB>/NODO-AIRE
```

Al final imprime el sha256 de cada archivo: el de `oefa_features.csv` tiene que ser la huella que
imprime `cmd/distribuido` al arrancar (`huella sha256:2fdace5ad859…` con el gold actual). Si el gold
está en otro lado, `GOLD=/ruta/oefa_features.csv lab/armar-usb.sh …`; para un USB solo de smoke
test, `GOLD=ninguno`.

**2. Abrir el puerto del coordinador** en la laptop, solo para la subred del laboratorio:

```bash
sudo ufw allow from 10.11.36.0/22 to any port 7070 proto tcp
```

**3. Lanzar el coordinador** en la laptop. Espera hasta 10 minutos:

```bash
go run ./cmd/smoke -nodos 4 -latidos 200 -salida reports/smoke_lab_4.json        # smoke test
go run ./cmd/distribuido -nodos 4 -salida reports/kmeans_distribuido_lab_4.json   # K-Means
```

**4. En cada equipo**, doble clic en `NODO-AIRE\EJECUTAR-NODO.bat`. Copia `nodo.exe` y el gold a
`%USERPROFILE%\aire-nodo`, abre el nodo en su propia ventana y avisa cuando ya se puede sacar el USB.
Un nodo que ya estaba corriendo de una sesión anterior sirve si se lanzó con este USB; si se lanzó
con uno sin gold, hay que cerrar su ventana y volver a hacer doble clic.

**5. Escalar de a poco: 2 → 4 → 8 → 20.** Los nodos siguen llamando entre corridas: basta volver a
correr el paso 3 con otro `-nodos` y otro `-salida`. Si se lanzan más nodos de los que espera el
coordinador, entran los primeros que llegan y los demás reintentan y quedan para la corrida
siguiente.

## El K-Means distribuido

```bash
go run ./cmd/distribuido -nodos 2  -salida reports/kmeans_distribuido_lab_2.json
go run ./cmd/distribuido -nodos 4  -salida reports/kmeans_distribuido_lab_4.json
go run ./cmd/distribuido -nodos 8  -salida reports/kmeans_distribuido_lab_8.json
go run ./cmd/distribuido -nodos 20 -salida reports/kmeans_distribuido_lab_20.json
```

Lee la receta (`configs/kmeans.json`), el gold y los centroides iniciales de la semilla 42
(`reports/centroides_iniciales.json`). Mientras espera, **Ctrl+C empieza con los nodos que ya
llegaron**; si vence `-plazo` (10 min) sin los N, no corre: una medición con menos nodos rotulada como
de N confunde. Los nodos se ordenan por nombre (el de cada equipo) y ese es el orden de los rangos y
el de la fusión.

Con 65 873 filas, cada nodo de 20 tiene unas 3 300: su cómputo por ronda es de microsegundos y cada
iteración cuesta lo que la red (`docs/diseno.md`, §4.5). Para ver cuándo el cómputo empieza a pesar,
**`-replicas r`** reparte una tabla virtual de r × 65 873 filas (la fila i es la i mod 65 873 del gold)
sin copiar nada nuevo al USB: cada nodo arma su rango repitiendo filas. Los centroides son los mismos
salvo redondeo, la inercia y los tamaños se multiplican por r. Cada réplica ocupa 1 MB en memoria,
repartido entre los nodos:

```bash
go run ./cmd/distribuido -nodos 20 -replicas 100 -salida reports/kmeans_distribuido_lab_20_r100.json
```

Otra semilla: `-semilla 43`. Las diez de la receta son diez corridas (`-semilla 42` … `51`) y gana la de
menor inercia, como en `cmd/entrenar`; con una basta para medir.

## Cómo leer el resultado

Smoke test:

| Lo que se ve | Qué significa |
|---|---|
| `SMOKE TEST OK` | Llegaron los N y respondieron todos los latidos |
| `INCOMPLETO: llegaron 3 de 4` | Algún equipo no llega al coordinador: puerto 7070 cerrado en la laptop, IP equivocada en `coordinador.txt` o el equipo sin red |
| Un nodo con error de plazo | Llegó pero dejó de responder: equipo suspendido, ventana cerrada o red saturada |
| El nodo repite `el coordinador no responde` | El coordinador no está corriendo o el firewall de la laptop lo bloquea |

K-Means distribuido (la línea `referencia` del resumen y `referencia` en el JSON):

| Lo que se ve | Qué significa |
|---|---|
| `BIT A BIT` | El modelo es exactamente el de `cmd/entrenar`. Pasa con un nodo, sin réplicas y con el chunk de la receta |
| `DENTRO DE TOLERANCIA 1e-09` | Mismas iteraciones y tamaños; los centroides difieren en los últimos bits porque la suma de K parciales se asocia distinto (en loopback: 4e-16 con 2 nodos, 9e-16 con 3) |
| `FUERA DE TOLERANCIA` (código 1) | Alguna fila cambió de cluster: el modelo se escribe igual, pero algo está mal (un nodo con otro binario, otro gold que pasó la huella…) |
| `llegaron 3 de 4 nodos … no hay corrida` | Venció el plazo de espera; ver la tabla del smoke test |
| `carga abortada: nodo "pc-07": … huella …` | Ese equipo tiene otro gold: volver a copiar el USB |
| `carga abortada: nodo "pc-07": … no encuentro el gold` | Ese nodo se lanzó sin gold al lado: cerrar su ventana y volver a lanzarlo con el USB nuevo |
| `ronda 12 abortada: nodo "pc-07": … plazo vencido` | El nodo no respondió en `-plazo-ronda` (10 s): se suspendió, se cerró o la red se cortó |

Si una corrida aborta, el coordinador no escribe nada, sale con código 1 y los demás nodos ven el
motivo en su ventana; vuelven a llamar solos y basta relanzar el coordinador. El JSON trae, por
iteración, el tiempo de la ronda (`ms_ronda_por_iteracion`: la barrera de red, del primer `ITERAR` al
último `PARCIAL`) separado del de la fusión y la actualización (`ms_fusion_por_iteracion`). Cada nodo
imprime al terminar su cómputo medio por ronda: la diferencia con la ronda del coordinador es red y
protocolo.

Al terminar se cierra el puerto con `sudo ufw delete allow from 10.11.36.0/22 to any port 7070 proto tcp`.
