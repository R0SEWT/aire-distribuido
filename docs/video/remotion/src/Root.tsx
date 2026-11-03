import { Composition } from "remotion";
import { Pipeline } from "./Pipeline";

// Cifras de reports/limpieza.json y reports/benchmark_cargador.json (CSV completo de OEFA).
export const RemotionRoot: React.FC = () => {
  return (
    <>
      <Composition
        id="PipelineCargador"
        component={Pipeline}
        durationInFrames={24 * 30}
        fps={30}
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
      />
    </>
  );
};
