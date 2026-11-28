import React from "react";
import { Easing, interpolate, useCurrentFrame } from "remotion";

// Lo que comparten las escenas del TB2: la paleta (la del pipeline de la PC3, oscura como la SPA),
// las cajas con su encendido y los mensajes que viajan por una línea.

export const C = {
  bg: "#0f1117",
  panel: "#171a23",
  panelClaro: "#1e2230",
  borde: "#2a2f3d",
  texto: "#e5e7eb",
  mudo: "#9ca3af",
  cian: "#22d3ee",
  verde: "#4ade80",
  ambar: "#fbbf24",
  rojo: "#f87171",
  violeta: "#a78bfa",
  azul: "#60a5fa",
};

export const FUENTE = "Inter, 'Segoe UI', Ubuntu, sans-serif";
export const MONO = "'JetBrains Mono', 'DejaVu Sans Mono', monospace";

export const clamp = { extrapolateLeft: "clamp", extrapolateRight: "clamp" } as const;
export const suave = Easing.bezier(0.16, 1, 0.3, 1);

// Progreso 0→1 entre dos segundos base.
export const entre = (frame: number, fps: number, a: number, b: number, easing = suave) =>
  interpolate(frame, [a * fps, b * fps], [0, 1], { ...clamp, easing });

export const Titulo: React.FC<{ fps: number; titulo: string; bajada: string }> = ({ fps, titulo, bajada }) => {
  const frame = useCurrentFrame();
  const p = entre(frame, fps, 0, 0.8);
  return (
    <div style={{ position: "absolute", top: 60, left: 90, right: 90, opacity: p, translate: `0px ${(1 - p) * 24}px` }}>
      <div style={{ fontSize: 64, fontWeight: 700, color: C.texto, letterSpacing: -1 }}>{titulo}</div>
      <div style={{ fontSize: 34, color: C.mudo, marginTop: 6 }}>{bajada}</div>
    </div>
  );
};

// Una caja que aparece en `aparece` y está encendida (borde de color y halo) entre `desde` y `hasta`.
export const Caja: React.FC<{
  fps: number;
  x: number;
  y: number;
  w: number;
  h: number;
  color: string;
  aparece: number;
  desde: number;
  hasta: number;
  apagada?: number; // opacidad después de `hasta` (por omisión, 0.55)
  children: React.ReactNode;
}> = ({ fps, x, y, w, h, color, aparece, desde, hasta, apagada = 0.55, children }) => {
  const frame = useCurrentFrame();
  const a = entre(frame, fps, aparece - 0.4, aparece);
  const encendida = interpolate(
    frame,
    [desde * fps, (desde + 0.35) * fps, hasta * fps, (hasta + 0.5) * fps],
    [0, 1, 1, 0],
    clamp,
  );
  const antes = frame < desde * fps;
  const opacidad = a * (antes ? 0.75 : interpolate(encendida, [0, 1], [apagada, 1]));
  return (
    <div
      style={{
        position: "absolute",
        left: x,
        top: y,
        width: w,
        height: h,
        opacity: opacidad,
        scale: String(interpolate(a, [0, 1], [0.94, 1])),
        borderRadius: 22,
        background: C.panel,
        border: `3px solid ${encendida > 0.5 ? color : C.borde}`,
        boxShadow: `0 0 ${46 * encendida}px ${color}55`,
        boxSizing: "border-box",
      }}
    >
      {children}
    </div>
  );
};

// Un mensaje (una píldora con su tipo) que viaja de (x0, y0) a (x1, y1) entre los segundos a y b.
export const Viajero: React.FC<{
  fps: number;
  x0: number;
  y0: number;
  x1: number;
  y1: number;
  a: number;
  b: number;
  texto: string;
  color: string;
}> = ({ fps, x0, y0, x1, y1, a, b, texto, color }) => {
  const frame = useCurrentFrame();
  const p = entre(frame, fps, a, b, Easing.inOut(Easing.cubic));
  const visible = frame >= a * fps && frame <= (b + 0.25) * fps;
  return (
    <div
      style={{
        position: "absolute",
        left: x0 + (x1 - x0) * p,
        top: y0 + (y1 - y0) * p,
        translate: "-50% -50%",
        opacity: visible ? interpolate(frame, [b * fps, (b + 0.25) * fps], [1, 0], clamp) : 0,
        background: color,
        color: "#0b0d13",
        fontFamily: MONO,
        fontWeight: 700,
        fontSize: 22,
        padding: "5px 14px",
        borderRadius: 999,
        whiteSpace: "nowrap",
        boxShadow: `0 0 18px ${color}88`,
      }}
    >
      {texto}
    </div>
  );
};

// Una línea recta entre dos puntos que se dibuja entre los segundos a y b.
export const Linea: React.FC<{
  fps: number;
  x0: number;
  y0: number;
  x1: number;
  y1: number;
  a: number;
  b: number;
  color?: string;
  grosor?: number;
}> = ({ fps, x0, y0, x1, y1, a, b, color = C.borde, grosor = 5 }) => {
  const frame = useCurrentFrame();
  const p = entre(frame, fps, a, b);
  const largo = Math.hypot(x1 - x0, y1 - y0);
  const ang = (Math.atan2(y1 - y0, x1 - x0) * 180) / Math.PI;
  return (
    <div
      style={{
        position: "absolute",
        left: x0,
        top: y0 - grosor / 2,
        width: largo * p,
        height: grosor,
        background: color,
        borderRadius: grosor,
        transformOrigin: "0 50%",
        rotate: `${ang}deg`,
      }}
    />
  );
};

// Un texto que entra entre a y a+0.5 y sale entre b-0.3 y b.
export const Aparece: React.FC<{
  fps: number;
  a: number;
  b?: number;
  style?: React.CSSProperties;
  children: React.ReactNode;
}> = ({ fps, a, b, style, children }) => {
  const frame = useCurrentFrame();
  const entra = entre(frame, fps, a, a + 0.5);
  const sale = b === undefined ? 1 : 1 - entre(frame, fps, b - 0.3, b);
  return <div style={{ position: "absolute", opacity: entra * sale, translate: `0px ${(1 - entra) * 14}px`, ...style }}>{children}</div>;
};
