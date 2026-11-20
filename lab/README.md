# Smoke test del clúster en el laboratorio

Comprueba, antes de distribuir el K-Means de la PC4, que cada equipo del laboratorio llega al
coordinador por TCP, habla el protocolo de `protocolo/` y responde a tiempo. Deja medida la ida y
vuelta con cada nodo y la de la barrera con todos a la vez (`docs/diseno.md`, sección 4.5).

- `cmd/nodo`: el nodo. Llama al coordinador, se presenta con `HOLA` y responde cada `PING` con
  `PONG` hasta el `FIN`. Después vuelve a llamar solo, así que se lanza una vez por equipo y sirve
  para varias corridas. A `CARGAR` e `ITERAR` responde con un `ERROR` hasta que la PC4 los implemente.
- `cmd/smoke`: el coordinador de prueba. Espera N nodos, manda rondas de `PING` a todos a la vez
  con `protocolo.Latido` y cierra con `FIN`.

En los equipos del laboratorio no hace falta instalar nada ni ser administrador: `nodo.exe` es un
solo ejecutable y solo abre conexiones de salida, que el Firewall de Windows permite.

## Pasos

**1. Armar el USB** desde la raíz del repo (compila `nodo.exe` para Windows y escribe la IP de la
laptop en `coordinador.txt`):

```bash
lab/armar-usb.sh /run/media/$USER/<USB>/NODO-AIRE
```

**2. Abrir el puerto del coordinador** en la laptop, solo para la subred del laboratorio:

```bash
sudo ufw allow from 10.11.36.0/22 to any port 7070 proto tcp
```

**3. Lanzar el coordinador** en la laptop. Espera hasta 10 minutos; Ctrl+C empieza con los que
hayan llegado:

```bash
go run ./cmd/smoke -nodos 4 -latidos 200 -salida reports/smoke_lab_4.json
```

**4. En cada equipo**, doble clic en `NODO-AIRE\EJECUTAR-NODO.bat`. Copia el nodo a
`%USERPROFILE%\aire-nodo`, lo abre en su propia ventana y avisa cuando ya se puede sacar el USB.

**5. Escalar de a poco: 2 → 4 → 8 → 20.** Los nodos siguen llamando entre corridas: basta volver a
correr el paso 3 con otro `-nodos`. Si se lanzan más nodos de los que espera el coordinador, entran
los primeros que llegan y los demás reintentan.

## Cómo leer el resultado

| Lo que se ve | Qué significa |
|---|---|
| `SMOKE TEST OK` | Llegaron los N y respondieron todos los latidos |
| `INCOMPLETO: llegaron 3 de 4` | Algún equipo no llega al coordinador: puerto 7070 cerrado en la laptop, IP equivocada en `coordinador.txt` o el equipo sin red |
| Un nodo con error de plazo | Llegó pero dejó de responder: equipo suspendido, ventana cerrada o red saturada |
| El nodo repite `el coordinador no responde` | El coordinador no está corriendo o el firewall de la laptop lo bloquea |

Al terminar se cierra el puerto con `sudo ufw delete allow from 10.11.36.0/22 to any port 7070 proto tcp`.
