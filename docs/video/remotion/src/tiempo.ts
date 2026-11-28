import { useVideoConfig } from "remotion";

// Cada escena se escribe en «segundos base» (los de su línea de tiempo original)
// y se renderiza a la duración que pida el montaje con la prop `segundos`: sus
// tiempos internos se estiran en proporción. El montaje (docs/video/montar_tb2.py)
// la pide a la duración del clip de voz de su bloque.
//
// useFpsEscena devuelve los cuadros por «segundo base»: con la duración original
// es el fps de la composición; con el doble de duración, el doble.
export const useFpsEscena = (base: number): number => {
  const { durationInFrames } = useVideoConfig();
  return durationInFrames / base;
};

// Para calculateMetadata: la duración en cuadros a partir de la prop `segundos` (o la base).
export const metadatosConDuracion =
  (base: number, fps: number) =>
  ({ props }: { props: Record<string, unknown> }) => {
    const s = typeof props.segundos === "number" && props.segundos > 0 ? props.segundos : base;
    return { durationInFrames: Math.max(1, Math.round(s * fps)) };
  };
