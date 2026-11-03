# Escenas animadas con Remotion

Prueba de Remotion (video programado en React) para el video del TB2: una escena del pipeline
del cargador concurrente, con las cifras de `reports/limpieza.json` y
`reports/benchmark_cargador.json` escritas como `defaultProps` en `src/Root.tsx`.

```bash
npm i                                                   # una vez (Node 22)
npx remotion studio                                     # vista previa en el navegador
npx remotion render PipelineCargador out/pipeline-cargador.mp4   # 24 s, 1920×1080, 30 fps, sin audio
```

`out/` y `node_modules/` no se versionan. La escena vive en `src/Pipeline.tsx`; cada estación
(CSV, bloques, workers, fusión y pivot, gold) se activa en orden y las fichas viajan por las
flechas en su fase. Para cambiar una cifra basta editar `defaultProps`.
