import React from "react";
import {
  AbsoluteFill,
  Easing,
  interpolate,
  useCurrentFrame,
  useVideoConfig,
} from "remotion";

// Escena: el pipeline del cargador concurrente (parte A). Las cifras son las de
// reports/limpieza.json y reports/benchmark_cargador.json sobre el CSV completo.
export type PipelineProps = {
  readonly bytesGB: string;
  readonly filasLargas: string;
  readonly bloques: number;
  readonly bloqueMB: number;
  readonly workers: number;
  readonly procesadas: string;
  readonly descartadas: string;
  readonly filasAnchas: string;
  readonly filasGold: string;
  readonly duracionS: string;
  readonly speedup: string;
  readonly cpus: number;
};

const C = {
  bg: "#0f1117",
  panel: "#171a23",
  borde: "#2a2f3d",
  texto: "#e5e7eb",
  mudo: "#9ca3af",
  cian: "#22d3ee",
  verde: "#4ade80",
  ambar: "#fbbf24",
  rojo: "#f87171",
  violeta: "#a78bfa",
};

// Estaciones: inicio y fin (en segundos) de su fase activa.
const ESTACIONES = [
  { id: "csv", titulo: "CSV de OEFA", x: 60, desde: 1.5, hasta: 4 },
  { id: "bloques", titulo: "Bloques por bytes", x: 430, desde: 4, hasta: 8 },
  { id: "workers", titulo: "Workers", x: 800, desde: 8, hasta: 14 },
  { id: "fusion", titulo: "Fusión y pivot", x: 1170, desde: 14, hasta: 18 },
  { id: "gold", titulo: "Gold", x: 1540, desde: 18, hasta: 21.5 },
] as const;

const ANCHO = 320;
const ALTO = 540;
const ARRIBA = 270;

const clamp = { extrapolateLeft: "clamp", extrapolateRight: "clamp" } as const;
const suave = Easing.bezier(0.16, 1, 0.3, 1);

const fase = (frame: number, fps: number, desde: number, hasta: number) =>
  interpolate(frame, [desde * fps, hasta * fps], [0, 1], { ...clamp, easing: Easing.linear });

const Titulo: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  return (
    <div
      style={{
        position: "absolute",
        top: 90,
        left: 100,
        right: 100,
        opacity: interpolate(frame, [0, 0.8 * fps], [0, 1], { ...clamp, easing: suave }),
        translate: `0px ${interpolate(frame, [0, 0.8 * fps], [24, 0], { ...clamp, easing: suave })}px`,
      }}
    >
      <div style={{ fontSize: 72, fontWeight: 700, color: C.texto, letterSpacing: -1 }}>
        Cargador concurrente · parte A
      </div>
      <div style={{ fontSize: 40, color: C.mudo, marginTop: 10 }}>
        del CSV largo de OEFA al gold estandarizado, con un pool de workers
      </div>
    </div>
  );
};

const Estacion: React.FC<{
  titulo: string;
  x: number;
  desde: number;
  hasta: number;
  children: React.ReactNode;
}> = ({ titulo, x, desde, hasta, children }) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const aparece = interpolate(frame, [(desde - 0.4) * fps, desde * fps], [0, 1], {
    ...clamp,
    easing: suave,
  });
  const activa = interpolate(
    frame,
    [desde * fps, (desde + 0.3) * fps, hasta * fps, (hasta + 0.5) * fps],
    [0, 1, 1, 0.35],
    clamp,
  );
  return (
    <div
      style={{
        position: "absolute",
        left: x,
        top: ARRIBA,
        width: ANCHO,
        height: ALTO,
        opacity: aparece,
        scale: String(interpolate(aparece, [0, 1], [0.92, 1])),
        borderRadius: 24,
        background: C.panel,
        border: `3px solid ${activa > 0.5 ? C.cian : C.borde}`,
        boxShadow: `0 0 ${40 * activa}px rgba(34, 211, 238, ${0.35 * activa})`,
      }}
    >
      <div
        style={{
          position: "absolute",
          top: 18,
          left: 0,
          right: 0,
          textAlign: "center",
          fontSize: 34,
          fontWeight: 700,
          color: activa > 0.5 ? C.cian : C.mudo,
        }}
      >
        {titulo}
      </div>
      <div style={{ position: "absolute", top: 80, left: 24, right: 24, bottom: 24 }}>{children}</div>
    </div>
  );
};

const Flecha: React.FC<{ x: number; desde: number; hasta: number; fichas: number; color: string }> = ({
  x,
  desde,
  hasta,
  fichas,
  color,
}) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const y = ARRIBA + ALTO / 2;
  const largo = 50;
  const p = fase(frame, fps, desde, hasta);
  return (
    <div style={{ position: "absolute", left: x, top: y - 3, width: largo, height: 6 }}>
      <div style={{ position: "absolute", inset: 0, background: C.borde, borderRadius: 3 }} />
      <div
        style={{
          position: "absolute",
          right: -10,
          top: -9,
          width: 0,
          height: 0,
          borderTop: "12px solid transparent",
          borderBottom: "12px solid transparent",
          borderLeft: `16px solid ${C.borde}`,
        }}
      />
      {Array.from({ length: fichas }).map((_, i) => {
        const t = (p * (fichas + 1) - i) / 1.5;
        const visible = t > 0 && t < 1;
        return (
          <div
            key={i}
            style={{
              position: "absolute",
              top: -8,
              left: interpolate(t, [0, 1], [-12, largo + 8], clamp),
              width: 22,
              height: 22,
              borderRadius: 6,
              background: color,
              opacity: visible ? 1 : 0,
            }}
          />
        );
      })}
    </div>
  );
};

const Csv: React.FC<{ bytesGB: string; filasLargas: string }> = ({ bytesGB, filasLargas }) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const lineas = 14;
  return (
    <div style={{ position: "relative", height: "100%" }}>
      <div
        style={{
          position: "absolute",
          left: 56,
          top: 10,
          width: 160,
          height: 220,
          borderRadius: 12,
          background: "#0b0d13",
          border: `2px solid ${C.borde}`,
          overflow: "hidden",
        }}
      >
        {Array.from({ length: lineas }).map((_, i) => (
          <div
            key={i}
            style={{
              position: "absolute",
              left: 14,
              top: 14 + i * 14,
              height: 6,
              borderRadius: 3,
              background: i % 5 === 0 ? C.ambar : C.borde,
              width: interpolate(
                frame,
                [(1.6 + i * 0.08) * fps, (2.0 + i * 0.08) * fps],
                [0, 90 + ((i * 37) % 40)],
                { ...clamp, easing: suave },
              ),
            }}
          />
        ))}
      </div>
      <div style={{ position: "absolute", top: 250, left: 0, right: 0, textAlign: "center" }}>
        <div style={{ fontSize: 52, fontWeight: 700, color: C.texto }}>{bytesGB} GB</div>
        <div style={{ fontSize: 28, color: C.mudo, marginTop: 6 }}>{filasLargas} filas largas</div>
        <div style={{ fontSize: 24, color: C.mudo, marginTop: 6 }}>una fila por parámetro y hora</div>
      </div>
    </div>
  );
};

const Bloques: React.FC<{ bloques: number; bloqueMB: number }> = ({ bloques, bloqueMB }) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const cols = 4;
  const w = 58;
  const h = 40;
  const gap = 12;
  return (
    <div style={{ position: "relative", height: "100%" }}>
      {Array.from({ length: bloques }).map((_, i) => {
        const col = i % cols;
        const fila = Math.floor(i / cols);
        const t0 = 4.2 + i * 0.12;
        return (
          <div
            key={i}
            style={{
              position: "absolute",
              left: 10 + col * (w + gap),
              top: 6 + fila * (h + gap),
              width: w,
              height: h,
              borderRadius: 8,
              background: C.ambar,
              color: "#1f1300",
              fontSize: 20,
              fontWeight: 700,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              opacity: interpolate(frame, [t0 * fps, (t0 + 0.25) * fps], [0, 1], clamp),
              scale: String(
                interpolate(frame, [t0 * fps, (t0 + 0.35) * fps], [0.4, 1], { ...clamp, easing: suave }),
              ),
            }}
          >
            {i + 1}
          </div>
        );
      })}
      <div style={{ position: "absolute", top: 272, left: 0, right: 0, textAlign: "center" }}>
        <div style={{ fontSize: 44, fontWeight: 700, color: C.texto }}>
          {bloques} × {bloqueMB} MB
        </div>
        <div style={{ fontSize: 24, color: C.mudo, marginTop: 8, lineHeight: 1.25 }}>
          cortados en el salto de línea siguiente: ninguna fila se parte
        </div>
      </div>
    </div>
  );
};

const Workers: React.FC<{ workers: number; bloques: number; procesadas: string; descartadas: string }> = ({
  workers,
  bloques,
  procesadas,
  descartadas,
}) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const altoCarril = 19;
  const gap = 5;
  return (
    <div style={{ position: "relative", height: "100%" }}>
      {Array.from({ length: workers }).map((_, i) => {
        // cada worker toma bloques del canal: el reparto es dinámico, aquí se dibuja i, i+W, ...
        const propios = Array.from({ length: bloques }).filter((_, b) => b % workers === i).length;
        const t0 = 8.3 + (i % 4) * 0.15;
        const t1 = t0 + 1.4 + propios * 1.3 + (i % 3) * 0.25;
        const p = interpolate(frame, [t0 * fps, t1 * fps], [0, 1], { ...clamp, easing: Easing.linear });
        return (
          <div key={i} style={{ position: "absolute", top: i * (altoCarril + gap), left: 0, right: 0 }}>
            <div
              style={{
                position: "absolute",
                left: 0,
                top: 0,
                width: 40,
                fontSize: 17,
                color: C.mudo,
                fontFamily: "monospace",
                lineHeight: `${altoCarril}px`,
              }}
            >
              w{i + 1}
            </div>
            <div
              style={{
                position: "absolute",
                left: 44,
                right: 0,
                top: 2,
                height: altoCarril - 4,
                borderRadius: 6,
                background: "#0b0d13",
                border: `1px solid ${C.borde}`,
                overflow: "hidden",
              }}
            >
              <div
                style={{
                  position: "absolute",
                  left: 0,
                  top: 0,
                  bottom: 0,
                  width: `${p * 100}%`,
                  background: p >= 1 ? C.verde : C.cian,
                }}
              />
            </div>
          </div>
        );
      })}
      <div style={{ position: "absolute", top: 296, left: 0, right: 0, textAlign: "center" }}>
        <div style={{ fontSize: 24, color: C.mudo, lineHeight: 1.3 }}>
          parseo · limpieza por regla · pivot parcial
        </div>
        <div style={{ fontSize: 24, color: C.verde, marginTop: 6 }}>procesadas {procesadas}</div>
        <div style={{ fontSize: 24, color: C.rojo }}>descartadas {descartadas}</div>
      </div>
    </div>
  );
};

const Fusion: React.FC<{ bloques: number; filasAnchas: string }> = ({ bloques, filasAnchas }) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const filas = 10;
  return (
    <div style={{ position: "relative", height: "100%" }}>
      {Array.from({ length: filas }).map((_, i) => {
        const t0 = 14.2 + i * 0.22;
        const op = interpolate(frame, [t0 * fps, (t0 + 0.3) * fps], [0, 1], clamp);
        return (
          <div
            key={i}
            style={{
              position: "absolute",
              top: 4 + i * 23,
              left: 0,
              right: 0,
              height: 18,
              display: "flex",
              gap: 6,
              opacity: op,
              translate: `${interpolate(frame, [t0 * fps, (t0 + 0.4) * fps], [-30, 0], { ...clamp, easing: suave })}px 0px`,
            }}
          >
            <div style={{ width: 54, background: C.ambar, borderRadius: 4, fontSize: 14, color: "#1f1300", textAlign: "center", lineHeight: "18px", fontWeight: 700 }}>
              b{i + 1 === filas ? bloques : i + 1}
            </div>
            {Array.from({ length: 5 }).map((__, j) => (
              <div key={j} style={{ flex: 1, background: j < 2 ? C.cian : C.borde, borderRadius: 4, opacity: 0.8 }} />
            ))}
          </div>
        );
      })}
      <div style={{ position: "absolute", top: 240, left: 0, right: 0, textAlign: "center" }}>
        <div style={{ fontSize: 24, color: C.mudo, lineHeight: 1.3 }}>
          se cosen en orden de bloque: igual con 1 o con 12 workers
        </div>
        <div style={{ fontSize: 44, fontWeight: 700, color: C.texto, marginTop: 10 }}>{filasAnchas}</div>
        <div style={{ fontSize: 24, color: C.mudo }}>filas anchas (estación × hora)</div>
      </div>
    </div>
  );
};

const Gold: React.FC<{ filasGold: string }> = ({ filasGold }) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const puntos = 28;
  const p = interpolate(frame, [18.3 * fps, 20.3 * fps], [0, 1], { ...clamp, easing: suave });
  return (
    <div style={{ position: "relative", height: "100%" }}>
      <div style={{ position: "absolute", left: 20, top: 0, width: 232, height: 232, borderRadius: 12, background: "#0b0d13", border: `1px solid ${C.borde}` }}>
        {Array.from({ length: puntos }).map((_, i) => {
          // nube sesgada (unidades crudas) que se recentra y reescala (z-score)
          const a = (i * 2.399) % (2 * Math.PI);
          const r = 8 + ((i * 53) % 60);
          const r1 = 14 + ((i * 37) % 58);
          const x0 = 40 + Math.cos(a) * r * 1.6 + r * 0.9;
          const y0 = 190 - Math.sin(a) * r * 0.9 - r * 0.8;
          const x1 = 116 + Math.cos(a) * r1;
          const y1 = 116 + Math.sin(a) * r1;
          return (
            <div
              key={i}
              style={{
                position: "absolute",
                left: interpolate(p, [0, 1], [x0, x1]) - 5,
                top: interpolate(p, [0, 1], [y0, y1]) - 5,
                width: 10,
                height: 10,
                borderRadius: 5,
                background: p > 0.5 ? C.verde : C.cian,
                opacity: interpolate(frame, [18 * fps, 18.4 * fps], [0, 1], clamp),
              }}
            />
          );
        })}
        <div style={{ position: "absolute", left: 8, bottom: 4, fontSize: 16, color: C.mudo, fontFamily: "monospace" }}>pm10</div>
        <div style={{ position: "absolute", left: 4, top: 6, fontSize: 16, color: C.mudo, fontFamily: "monospace" }}>pm2_5</div>
      </div>
      <div style={{ position: "absolute", top: 250, left: 0, right: 0, textAlign: "center" }}>
        <div style={{ fontSize: 24, color: C.mudo, lineHeight: 1.3 }}>
          z-score por chunks de 1000 filas (Welford, fusión de Chan)
        </div>
        <div style={{ fontSize: 44, fontWeight: 700, color: C.verde, marginTop: 10 }}>{filasGold}</div>
        <div style={{ fontSize: 24, color: C.mudo }}>filas para el K-Means</div>
      </div>
    </div>
  );
};

const Pie: React.FC<{ duracionS: string; speedup: string; cpus: number; workers: number }> = ({
  duracionS,
  speedup,
  cpus,
  workers,
}) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const op = interpolate(frame, [21.6 * fps, 22.4 * fps], [0, 1], { ...clamp, easing: suave });
  return (
    <div
      style={{
        position: "absolute",
        left: 100,
        right: 100,
        bottom: 40,
        display: "flex",
        justifyContent: "space-between",
        gap: 40,
        opacity: op,
        translate: `0px ${interpolate(op, [0, 1], [20, 0])}px`,
      }}
    >
      {[
        [duracionS + " s", `el CSV completo en ${cpus} CPU`],
        [speedup + "×", `speedup con ${workers} workers (IC 95 %)`],
        ["idéntico", "silver y gold con el mismo sha256 para todo W"],
      ].map(([grande, chico]) => (
        <div key={grande} style={{ flex: 1, background: C.panel, border: `2px solid ${C.borde}`, borderRadius: 20, padding: "18px 28px" }}>
          <div style={{ fontSize: 50, fontWeight: 700, color: C.verde, lineHeight: 1.1 }}>{grande}</div>
          <div style={{ fontSize: 24, color: C.mudo, marginTop: 4 }}>{chico}</div>
        </div>
      ))}
    </div>
  );
};

const Leyenda: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const textos: Array<[number, number, string]> = [
    [1.5, 4, "El archivo entero no cabe en una sola pasada cómoda: se lee por bloques."],
    [4, 8, "Un lector corta el archivo en bloques de 64 MB alineados al salto de línea y los manda por un canal."],
    [8, 14, "Un pool de workers consume el canal: cada bloque se parsea, se limpia regla por regla y se pivotea en paralelo."],
    [14, 18, "Los parciales se fusionan en orden de bloque; el resultado no depende del entrelazado de las goroutines."],
    [18, 21.5, "La estandarización corre por chunks con Welford y se funde con Chan: el gold que entra al K-Means."],
  ];
  return (
    <div style={{ position: "absolute", left: 100, right: 100, top: 850, height: 90 }}>
      {textos.map(([a, b, t]) => (
        <div
          key={t}
          style={{
            position: "absolute",
            inset: 0,
            fontSize: 34,
            color: C.texto,
            textAlign: "center",
            lineHeight: 1.3,
            opacity: interpolate(frame, [a * fps, (a + 0.4) * fps, (b - 0.2) * fps, b * fps], [0, 1, 1, 0], clamp),
          }}
        >
          {t}
        </div>
      ))}
    </div>
  );
};

export const Pipeline: React.FC<PipelineProps> = (p) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const vaPie = frame > 21.6 * fps;
  return (
    <AbsoluteFill style={{ background: C.bg, fontFamily: "Inter, 'Segoe UI', Ubuntu, sans-serif" }}>
      <Titulo />
      <Estacion {...ESTACIONES[0]}>
        <Csv bytesGB={p.bytesGB} filasLargas={p.filasLargas} />
      </Estacion>
      <Flecha x={ESTACIONES[0].x + ANCHO + 0} desde={3.4} hasta={5.5} fichas={4} color={C.ambar} />
      <Estacion {...ESTACIONES[1]}>
        <Bloques bloques={p.bloques} bloqueMB={p.bloqueMB} />
      </Estacion>
      <Flecha x={ESTACIONES[1].x + ANCHO} desde={7.4} hasta={11} fichas={8} color={C.ambar} />
      <Estacion {...ESTACIONES[2]}>
        <Workers workers={p.workers} bloques={p.bloques} procesadas={p.procesadas} descartadas={p.descartadas} />
      </Estacion>
      <Flecha x={ESTACIONES[2].x + ANCHO} desde={13.2} hasta={16.5} fichas={8} color={C.cian} />
      <Estacion {...ESTACIONES[3]}>
        <Fusion bloques={p.bloques} filasAnchas={p.filasAnchas} />
      </Estacion>
      <Flecha x={ESTACIONES[3].x + ANCHO} desde={17.4} hasta={19.5} fichas={4} color={C.cian} />
      <Estacion {...ESTACIONES[4]}>
        <Gold filasGold={p.filasGold} />
      </Estacion>
      <div style={{ opacity: vaPie ? 0 : 1 }}>
        <Leyenda />
      </div>
      <Pie duracionS={p.duracionS} speedup={p.speedup} cpus={p.cpus} workers={p.workers} />
    </AbsoluteFill>
  );
};
