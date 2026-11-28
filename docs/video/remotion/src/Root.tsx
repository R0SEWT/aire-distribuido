import { Composition } from "remotion";
import { Arquitectura, BASE_ARQUITECTURA } from "./Arquitectura";
import { BASE_PIPELINE, Pipeline } from "./Pipeline";
import { BASE_RONDA, Ronda } from "./Ronda";
import { metadatosConDuracion } from "./tiempo";

const FPS = 30;

// Cada composición dura su base si no se le pide otra duración; con --props='{"segundos":N}' dura N
// segundos y sus tiempos internos se estiran en proporción (src/tiempo.ts). El montaje del TB2
// (docs/video/montar_tb2.py) la pide a la duración del clip de voz de su bloque.
export const RemotionRoot: React.FC = () => {
  return (
    <>
      {/* Cifras de reports/limpieza.json y reports/benchmark_cargador.json (CSV completo de OEFA). */}
      <Composition
        id="PipelineCargador"
        component={Pipeline}
        durationInFrames={BASE_PIPELINE * FPS}
        fps={FPS}
        width={1920}
        height={1080}
        defaultProps={{
          bytesGB: "1.32",
          filasLargas: "2,883,877",
          bloques: 20,
          bloqueMB: 64,
          workers: 12,
          procesadas: "1,434,498",
          descartadas: "1,449,379",
          filasAnchas: "236,001",
          filasGold: "65,873",
          duracionS: "1.33",
          speedup: "4.94",
          cpus: 14,
        }}
        calculateMetadata={metadatosConDuracion(BASE_PIPELINE, FPS)}
      />
      {/* Video del TB2, bloque 02. Cifras de src/datos-tb2.json (datos.py). */}
      <Composition
        id="Arquitectura"
        component={Arquitectura}
        durationInFrames={BASE_ARQUITECTURA * FPS}
        fps={FPS}
        width={1920}
        height={1080}
        defaultProps={{ segundos: BASE_ARQUITECTURA }}
        calculateMetadata={metadatosConDuracion(BASE_ARQUITECTURA, FPS)}
      />
      {/* Video del TB2, bloque 04. Cifras de src/datos-tb2.json (datos.py). */}
      <Composition
        id="RondaDistribuida"
        component={Ronda}
        durationInFrames={BASE_RONDA * FPS}
        fps={FPS}
        width={1920}
        height={1080}
        defaultProps={{ segundos: BASE_RONDA }}
        calculateMetadata={metadatosConDuracion(BASE_RONDA, FPS)}
      />
    </>
  );
};
