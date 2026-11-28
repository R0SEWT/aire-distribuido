import React from "react";
import { AbsoluteFill, interpolate, useCurrentFrame } from "remotion";
import { Aparece, C, Caja, clamp, entre, FUENTE, Linea, MONO, Titulo, Viajero } from "./comun";
import { useFpsEscena } from "./tiempo";
import datos from "./datos-tb2.json";

// Escena del bloque 04 del guion del TB2: una iteración del K-Means distribuido por sockets TCP
// (protocolo/, cluster/; docs/diseno.md §4). Los tiempos siguen la narración de Rody, en segundos base:
//   0–8      «la API escucha a los nodos en el 7070 con net.Listen, y cada nodo se conecta con
//            net.Dial»: el coordinador, los nodos que marcan y su HOLA
//   8–12     «hablan en JSON por línea sobre esa conexión TCP»: una línea del protocolo
//   12–17    «con CARGAR, cada nodo lee su rango del gold una sola vez»: CARGAR, rangos y LISTO
//   17–21,3  «en cada iteración el coordinador manda ITERAR con los centroides»
//   21,3–27,6 «cada nodo reparte su rango entre sus goroutines y responde PARCIAL con sumas y conteos»
//   27,6–35  «un WaitGroup hace de barrera, con un plazo, y los parciales se fusionan en orden de nodo»
//   35–40    «con uno a cuatro nodos sale el mismo modelo que el secuencial»: la tabla
// La esquina de abajo a la derecha queda libre para la cámara del que habla.
export const BASE_RONDA = 40;

const CX = 90; // coordinador
const CY = 210;
const CW = 560;
const CH = 600;
const NX = 1100; // nodos
const NW = 470;
const NH = 170;
const NY = [210, 425, 640];
const centroNodo = (i: number) => NY[i] + NH / 2;
const borde = CX + CW;

const Rango: React.FC<{ fps: number; i: number }> = ({ fps, i }) => {
  const frame = useCurrentFrame();
  const p = entre(frame, fps, 12.9 + i * 0.25, 15.2 + i * 0.25);
  return (
    <div style={{ position: "absolute", left: 22, right: 22, top: 66 }}>
      <div style={{ height: 10, borderRadius: 5, background: C.panelClaro, overflow: "hidden" }}>
        <div style={{ width: `${p * 100}%`, height: "100%", background: C.ambar }} />
      </div>
      <div style={{ fontSize: 20, color: p > 0 ? C.ambar : C.mudo, fontFamily: MONO, marginTop: 4 }}>
        filas {datos.rangos_txt[i]}
      </div>
    </div>
  );
};

// Las goroutines del nodo: carriles con chunks que se procesan entre 21,6 y 25,4.
const Goroutines: React.FC<{ fps: number; i: number }> = ({ fps, i }) => {
  const frame = useCurrentFrame();
  const carriles = datos.workers_por_nodo;
  const visible = entre(frame, fps, 21.3, 21.8) * (1 - entre(frame, fps, 31.5, 32));
  return (
    <div style={{ position: "absolute", left: 22, right: 22, top: 116, opacity: visible }}>
      {Array.from({ length: carriles }).map((_, c) => {
        const t = entre(frame, fps, 21.6 + c * 0.15 + i * 0.1, 25.2 + c * 0.1 + i * 0.4);
        return (
          <div key={c} style={{ display: "flex", alignItems: "center", gap: 10, marginTop: c ? 4 : 0, height: 18 }}>
            <div style={{ fontSize: 16, color: C.mudo, fontFamily: MONO, width: 110 }}>goroutine {c + 1}</div>
            <div style={{ flex: 1, height: 10, borderRadius: 5, background: C.panelClaro, overflow: "hidden" }}>
              <div style={{ width: `${t * 100}%`, height: "100%", background: C.cian }} />
            </div>
          </div>
        );
      })}
    </div>
  );
};

const Nodo: React.FC<{ fps: number; i: number }> = ({ fps, i }) => {
  const llega = 3.2 + i * 1.1;
  return (
    <Caja fps={fps} x={NX} y={NY[i]} w={NW} h={NH} color={C.cian} aparece={llega} desde={llega} hasta={llega + 1.6} apagada={0.9}>
      <div style={{ padding: "14px 22px 0", display: "flex", justifyContent: "space-between", alignItems: "baseline" }}>
        <span style={{ fontSize: 30, fontWeight: 700, color: C.texto }}>nodo-{i + 1}</span>
        <span style={{ fontSize: 19, color: C.mudo, fontFamily: MONO }}>net.Dialer → api:{datos.puerto_nodos}</span>
      </div>
      <Rango fps={fps} i={i} />
      <Goroutines fps={fps} i={i} />
    </Caja>
  );
};

// La espera: una goroutine por nodo manda ITERAR y lee su PARCIAL; el WaitGroup espera a las tres.
// Cada casilla se llena cuando llega el PARCIAL de su nodo.
const Barrera: React.FC<{ fps: number; llegadas: number[] }> = ({ fps, llegadas }) => {
  const frame = useCurrentFrame();
  const n = llegadas.filter((t) => frame >= t * fps).length;
  const libre = frame >= 28.2 * fps;
  const plazo = interpolate(frame, [17.6 * fps, 28 * fps], [1, 0.18], clamp);
  return (
    <Aparece fps={fps} a={17.4} b={33} style={{ left: 28, top: 225, width: CW - 56 }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <div style={{ fontSize: 26, color: C.texto, fontWeight: 600 }}>
          <span style={{ fontFamily: MONO, color: libre ? C.verde : C.ambar }}>WaitGroup</span> · 3 goroutines
        </div>
        <div style={{ fontSize: 22, color: C.mudo, fontFamily: MONO, whiteSpace: "nowrap" }}>
          {libre ? "Wait() volvió" : `${n}/${llegadas.length} parciales`}
        </div>
      </div>
      <div style={{ display: "flex", gap: 12, marginTop: 12 }}>
        {llegadas.map((t, i) => {
          const lleno = frame >= t * fps;
          return (
            <div
              key={i}
              style={{
                flex: 1,
                height: 46,
                borderRadius: 12,
                border: `2px solid ${lleno ? C.verde : C.borde}`,
                background: lleno ? `${C.verde}33` : "transparent",
                color: lleno ? C.verde : C.mudo,
                fontSize: 20,
                fontFamily: MONO,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
              }}
            >
              nodo-{i + 1}
            </div>
          );
        })}
      </div>
      <div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 14, opacity: libre ? 0.35 : 1 }}>
        <div style={{ fontSize: 20, color: C.mudo }}>plazo de la ronda</div>
        <div style={{ flex: 1, height: 10, borderRadius: 5, background: C.panelClaro, overflow: "hidden" }}>
          <div style={{ width: `${plazo * 100}%`, height: "100%", background: C.ambar }} />
        </div>
      </div>
    </Aparece>
  );
};

// La fusión en orden de nodo y la inercia que baja iteración a iteración.
const Fusion: React.FC<{ fps: number }> = ({ fps }) => {
  const frame = useCurrentFrame();
  const inercias = datos.inercias;
  const p = entre(frame, fps, 31, 34.6);
  const iter = Math.max(1, Math.min(datos.iteraciones, Math.ceil(p * datos.iteraciones)));
  const max = inercias[0];
  const min = inercias[inercias.length - 1];
  const w = CW - 56;
  const h = 78;
  const puntos = inercias
    .slice(0, iter)
    .map((v, k) => `${(k / (inercias.length - 1)) * w},${h - ((v - min) / (max - min)) * h}`)
    .join(" ");
  return (
    <>
      <Aparece fps={fps} a={28.6} style={{ left: 28, top: 378, width: CW - 56 }}>
        <div style={{ fontSize: 24, color: C.texto }}>
          <span style={{ fontFamily: MONO, color: C.violeta }}>Fusionar</span> en orden de nodo:
        </div>
        <div style={{ display: "flex", gap: 10, marginTop: 8, fontFamily: MONO, fontSize: 20, whiteSpace: "nowrap" }}>
          {[0, 1, 2].map((i) => (
            <div
              key={i}
              style={{
                padding: "3px 10px",
                borderRadius: 10,
                background: frame >= (29 + i * 0.55) * fps ? `${C.violeta}44` : "transparent",
                border: `2px solid ${frame >= (29 + i * 0.55) * fps ? C.violeta : C.borde}`,
                color: C.texto,
              }}
            >
              nodo-{i + 1}
            </div>
          ))}
          <div style={{ color: C.mudo, alignSelf: "center" }}>→ centroides</div>
        </div>
      </Aparece>
      <Aparece fps={fps} a={30.8} style={{ left: 28, top: 462, width: w }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline" }}>
          <div style={{ fontSize: 22, color: C.mudo }}>inercia por iteración</div>
          <div style={{ fontSize: 28, color: C.texto, fontFamily: MONO, fontWeight: 700 }}>
            iter {iter}/{datos.iteraciones}
          </div>
        </div>
        <svg width={w} height={h + 6} style={{ marginTop: 8, overflow: "visible" }}>
          <polyline points={puntos} fill="none" stroke={C.verde} strokeWidth={4} strokeLinejoin="round" />
        </svg>
      </Aparece>
    </>
  );
};

const Tabla: React.FC<{ fps: number }> = ({ fps }) => {
  const frame = useCurrentFrame();
  const fondo = entre(frame, fps, 34.8, 35.4);
  const columnas = "150px 240px 340px";
  return (
    <>
      <div style={{ position: "absolute", inset: 0, background: C.bg, opacity: fondo * 0.95 }} />
      <Aparece fps={fps} a={35} style={{ left: 260, top: 250, width: 1080 }}>
        <div style={{ fontSize: 46, fontWeight: 700, color: C.texto }}>El mismo modelo con 1 a 4 nodos</div>
        <div style={{ fontSize: 26, color: C.mudo, marginTop: 6 }}>
          {datos.filas_txt} filas del gold, k = {datos.k}, la misma semilla; nodos como procesos locales en una misma máquina
        </div>
        <div style={{ display: "grid", gridTemplateColumns: columnas, marginTop: 28, fontSize: 24, color: C.mudo }}>
          <div>nodos</div>
          <div>iteraciones</div>
          <div>inercia final</div>
        </div>
        {datos.corridas.map((c, k) => (
          <div
            key={c.nodos}
            style={{
              display: "grid",
              gridTemplateColumns: columnas,
              marginTop: 12,
              fontSize: 36,
              fontFamily: MONO,
              color: C.texto,
              opacity: entre(frame, fps, 35.4 + k * 0.3, 35.8 + k * 0.3),
            }}
          >
            <div style={{ color: C.cian }}>{c.nodos}</div>
            <div>{c.iteraciones}</div>
            <div style={{ color: C.verde }}>{c.inercia}</div>
          </div>
        ))}
        <div style={{ fontSize: 26, color: C.mudo, marginTop: 26, opacity: entre(frame, fps, 37, 37.5) }}>
          Diferencia máxima de los centroides con el K-Means secuencial: {datos.dif_max.mantisa} × 10
          <sup>{datos.dif_max.exponente}</sup>
        </div>
      </Aparece>
    </>
  );
};

export type RondaProps = { readonly segundos?: number };

export const Ronda: React.FC<RondaProps> = () => {
  const fps = useFpsEscena(BASE_RONDA);
  const llegadas = [26.0, 26.7, 27.4];
  return (
    <AbsoluteFill style={{ background: C.bg, fontFamily: FUENTE }}>
      <Titulo fps={fps} titulo="Clúster · K-Means distribuido" bajada="sockets TCP entre la API coordinadora y los nodos" />

      <Caja fps={fps} x={CX} y={CY} w={CW} h={CH} color={C.violeta} aparece={0.8} desde={0.8} hasta={3} apagada={0.95}>
        <div style={{ padding: "16px 28px 0" }}>
          <div style={{ fontSize: 34, fontWeight: 700, color: C.texto }}>Coordinador · la API</div>
          <div style={{ fontSize: 22, color: C.violeta, fontFamily: MONO, marginTop: 8 }}>
            net.Listen("tcp", ":{datos.puerto_nodos}")
          </div>
        </div>
        <Aparece fps={fps} a={1.6} b={17.2} style={{ left: 28, top: 140, width: CW - 56, fontSize: 25, color: C.mudo, lineHeight: 1.4 }}>
          Acepta las conexiones con Accept(); los nodos marcan hacia aquí, nunca al revés.
        </Aparece>
        <Aparece fps={fps} a={17.4} b={33} style={{ left: 28, top: 140, width: CW - 56 }}>
          <div style={{ fontSize: 22, color: C.mudo }}>centroides de la ronda (k = {datos.k})</div>
          <div style={{ display: "flex", gap: 10, marginTop: 10 }}>
            {[C.verde, C.ambar, C.rojo].map((c) => (
              <div key={c} style={{ width: 36, height: 36, borderRadius: 18, background: c }} />
            ))}
          </div>
        </Aparece>
        <Barrera fps={fps} llegadas={llegadas} />
        <Fusion fps={fps} />
      </Caja>

      {[0, 1, 2].map((i) => (
        <React.Fragment key={i}>
          <Linea fps={fps} x0={NX} y0={centroNodo(i)} x1={borde} y1={centroNodo(i)} a={3.2 + i * 1.1} b={3.9 + i * 1.1} color={C.borde} grosor={6} />
          <Nodo fps={fps} i={i} />
          <Viajero fps={fps} x0={NX - 30} y0={centroNodo(i) - 26} x1={borde + 50} y1={centroNodo(i) - 26} a={4 + i * 1.1} b={5.2 + i * 1.1} texto="HOLA" color={C.cian} />
          <Viajero fps={fps} x0={borde + 50} y0={centroNodo(i) - 26} x1={NX - 40} y1={centroNodo(i) - 26} a={12.2} b={13.3} texto="CARGAR" color={C.ambar} />
          <Viajero fps={fps} x0={NX - 30} y0={centroNodo(i) + 26} x1={borde + 50} y1={centroNodo(i) + 26} a={15.6 + i * 0.25} b={16.7 + i * 0.25} texto="LISTO" color={C.ambar} />
          <Viajero fps={fps} x0={borde + 50} y0={centroNodo(i) - 26} x1={NX - 40} y1={centroNodo(i) - 26} a={18.2} b={19.6} texto="ITERAR" color={C.violeta} />
          <Viajero fps={fps} x0={NX - 30} y0={centroNodo(i) + 26} x1={borde + 60} y1={centroNodo(i) + 26} a={llegadas[i] - 1.2} b={llegadas[i]} texto="PARCIAL" color={C.verde} />
        </React.Fragment>
      ))}

      <Aparece fps={fps} a={8} b={12} style={{ left: CX, top: 850, width: NX + NW - CX }}>
        <div style={{ fontSize: 24, color: C.mudo }}>Un mensaje por línea, en JSON, sobre la misma conexión TCP (json.Marshal al escribir, bufio.Reader al leer):</div>
        <div
          style={{
            marginTop: 10,
            padding: "14px 20px",
            borderRadius: 14,
            background: "#0b0d13",
            border: `2px solid ${C.borde}`,
            fontFamily: MONO,
            fontSize: 29,
            color: C.texto,
          }}
        >
          {`{"tipo":"ITERAR","iter":${datos.iteraciones},"centroides":[${datos.centroides_txt}]}\\n`}
        </div>
      </Aparece>
      <Aparece fps={fps} a={22.5} b={28} style={{ left: CX, top: 850, width: NX + NW - CX }}>
        <div style={{ fontSize: 24, color: C.mudo }}>Cada nodo responde con los parciales de su rango:</div>
        <div
          style={{
            marginTop: 10,
            padding: "14px 20px",
            borderRadius: 14,
            background: "#0b0d13",
            border: `2px solid ${C.borde}`,
            fontFamily: MONO,
            fontSize: 29,
            color: C.texto,
          }}
        >
          {`{"tipo":"PARCIAL","iter":${datos.iteraciones},"sumas":[…],"conteos":[…],"inercia":…}\\n`}
        </div>
      </Aparece>

      <Tabla fps={fps} />
    </AbsoluteFill>
  );
};
