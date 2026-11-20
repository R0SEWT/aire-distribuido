# K-Means del gold en la GPU

Experimento del laboratorio, no parte del sistema: el sistema es Go y Python se queda del lado del
laboratorio, como los notebooks. Corre el mismo K-Means que `cmd/entrenar` en la GPU de cada PC
(NVIDIA T1000 8 GB, Turing sm_75) y responde dos preguntas de la PC4:

1. **¿Da lo mismo?** Lloyd desde el mismo arranque que eligió `cmd/entrenar` (semilla 42, de
   `reports/centroides_iniciales.json`), comparado con `reports/kmeans.json`, en fp64 y fp32.
2. **¿Desde qué tamaño conviene?** ms por iteración con el gold replicado 1, 10, 100 y 300 veces.

Respeta el contrato numérico de `kmeans/doc.go` (distancia euclídea al cuadrado, desempate por el menor
índice, clusters vacíos que conservan su centroide, la misma parada y la inercia final). Lo que no puede
respetar es el orden de la suma, y eso es parte del experimento: suma por cluster con `index_add_`
(**atómica**: miles de hilos suman en el orden en que llegan) o con `one_hot(asignación)ᵀ X`
(**matmul**: orden de reducción fijo para la misma forma).

## Archivos

| Archivo | Qué es |
|---|---|
| `kmeans_gpu.py` | El experimento. También corre en CPU (`--cpu`), que es como se probó en la laptop. |
| `armar-paquete.sh` | Arma desde Linux el Python portable para Windows: Python 3.12, torch 2.14.1+cu126, numpy y el runtime de Visual C++ sacado del instalador oficial de Microsoft (torch lo necesita y un PC sin el redistribuible fallaría al importarlo). Deja todo en `~/.cache/gpu-lab/servir`. |
| `servidor.py` | Sirve esa carpeta en la red del laboratorio y recibe los resultados (`PUT /resultados/*.json`). Su página muestra cómo lanzarlo y la tabla de resultados. |
| `gpu.bat` | Lo que corre cada PC: baja el paquete la primera vez, el script y los datos, corre, abre `nvidia-smi` y sube el resultado. Sin administrador ni instalación. |

## Cómo se corre

```bash
lab/gpu/armar-paquete.sh                      # ~1.5 min con la rueda de torch ya en caché
sudo ufw allow from 10.11.36.0/22 to any port 8090 proto tcp
python3 lab/gpu/servidor.py --dir ~/.cache/gpu-lab/servir --bind 10.11.37.73 --puerto 8090
```

En cada PC del laboratorio, Win+R:

```
cmd /k curl.exe -fsSo %TEMP%\gpu.bat http://10.11.37.73:8090/gpu.bat && %TEMP%\gpu.bat
```

## Primera corrida: 4 PC del laboratorio, 14/11

`reports/kmeans_gpu/*.json`, H45-022 a H45-025, cada uno con una T1000 8 GB y torch 2.14.1+cu126.
Los cuatro dieron lo mismo:

| Suma | Iteraciones (Go: 44) | Máx. diferencia con los centroides de Go | ¿Repetible bit a bit? |
|---|---|---|---|
| fp64 matmul | 44 | 4.4e-16 | sí |
| fp64 atómica | 44 | 5.7e-15 | **no** |
| fp32 matmul | 44 | 9.1e-07 | sí |
| fp32 atómica | **45** | 1.1e-05 | **no** |

La suma atómica no se repite bit a bit entre corridas en ningún PC (en la CPU de la laptop sí), y en fp32
ese orden cambiante mueve la convergencia a 45 iteraciones. Es el problema que el K-Means de Go evita
fusionando en orden de chunk, medido en la GPU.

ms por iteración, mediana de los 4 PC (10 iteraciones fijas, mediana de 5 mediciones por PC):

| Gold replicado | Filas | fp64 matmul | fp64 atómica | fp32 matmul | fp32 atómica |
|---|---|---|---|---|---|
| 1× | 65 873 | 1.02 | 0.63 | 0.72 | 0.87 |
| 10× | 658 730 | 7.76 | 4.55 | 3.78 | 3.54 |
| 100× | 6.6 M | 86.4 | 43.5 | 35.3 | 33.4 |
| 300× | 19.8 M | 259.9 | 131.7 | 108.2 | 101.4 |

Con el gold real la GPU no gana: lanzar y sincronizar cuesta más que el cálculo (Go concurrente con 12
workers hace 0.55 ms por iteración en la laptop, `reports/benchmark_kmeans.json`). Desde unos cientos de
miles de filas, la GPU sostiene unos 190 millones de filas por segundo en fp32. Falta medir Go en esos
mismos PC (24 hilos) para comparar en el mismo hardware. Es PyTorch directo, no un kernel a medida: queda
lejos del ancho de banda de la tarjeta.
