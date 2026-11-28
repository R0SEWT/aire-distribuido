# Escenas animadas con Remotion

Las escenas animadas de los videos, en React. Cada composición tiene una duración base y acepta la prop
`segundos`: con ella dura eso y sus tiempos internos se estiran en proporción (`src/tiempo.ts`). El
montaje del TB2 (`../montar_tb2.py`) la renderiza a la duración de la voz de su bloque.

| Composición | Archivo | Base | Dónde va | Cifras |
|---|---|---|---|---|
| `PipelineCargador` | `src/Pipeline.tsx` | 24 s | PC3, escena 5; TB2, bloque 03 | `defaultProps` en `src/Root.tsx`, de `reports/limpieza.json` y `reports/benchmark_cargador.json` |
| `Arquitectura` | `src/Arquitectura.tsx` | 20 s | TB2, bloque 02 | `src/datos-tb2.json` |
| `RondaDistribuida` | `src/Ronda.tsx` | 40 s | TB2, bloque 04 | `src/datos-tb2.json` |

`src/datos-tb2.json` lo escribe `python3 datos.py` desde `reports/kmeans_distribuido_local_{1..4}.json` y
`reports/api_latencia.json` (`armar-tb2.sh` lo corre antes de montar): ninguna cifra se escribe a mano.
`src/comun.tsx` tiene la paleta (la de la SPA) y las piezas que comparten: cajas que se encienden,
líneas y mensajes que viajan. Las escenas del TB2 dejan libre la esquina de abajo a la derecha, donde va
la cámara de quien habla.

```bash
npm ci                                                   # una vez (Node 22)
npx remotion studio                                      # vista previa en el navegador
npx remotion render RondaDistribuida out/ronda.mp4 --props='{"segundos":45}' --browser-executable=/usr/bin/google-chrome
```

Sin `--browser-executable`, el primer render descarga el Chrome de Remotion. `out/` y `node_modules/` no
se versionan. El `go.mod` de esta carpeta no es Go: separa `node_modules/` del módulo del repo para que
`go test ./...` no entre ahí.
