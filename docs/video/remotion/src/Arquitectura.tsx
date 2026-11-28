import React from "react";
import { AbsoluteFill, useCurrentFrame } from "remotion";
import { Aparece, C, Caja, entre, FUENTE, Linea, MONO, Titulo, Viajero } from "./comun";
import { useFpsEscena } from "./tiempo";
import datos from "./datos-tb2.json";

// Escena del bloque 02 del guion del TB2: las cuatro partes del enunciado y cómo se hablan.
// Los tiempos siguen la narración de Rody (guion-tb2.md, bloque 02), en segundos base:
//   0–4,4   «cuatro partes, todas en Go salvo la interfaz»: aparecen las cuatro, apagadas
//   4,4–6,8 «un cargador concurrente limpia el CSV»: A
//   6,8–10  «un clúster de nodos entrena un K-Means distribuido»: B
//   10–18   «una API coordinadora habla con los nodos por TCP, guarda en MongoDB y Redis
//            y atiende a una SPA en React»: C, el enlace TCP, las bases y D
//   18–20   «Docker Compose levanta todo eso, menos el cargador»: el marco del Compose, sin A
// La esquina de abajo a la derecha queda libre para la cámara del que habla.
export const BASE_ARQUITECTURA = 20;

const Encabezado: React.FC<{ letra: string; titulo: string; color: string }> = ({ letra, titulo, color }) => (
  <div style={{ display: "flex", alignItems: "center", gap: 14, padding: "18px 22px 6px" }}>
    <div
      style={{
        width: 46,
        height: 46,
        borderRadius: 12,
        background: color,
        color: "#0b0d13",
        fontWeight: 800,
        fontSize: 28,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
      }}
    >
      {letra}
    </div>
    <div style={{ fontSize: 32, fontWeight: 700, color: C.texto }}>{titulo}</div>
  </div>
);

const Renglones: React.FC<{ lineas: string[] }> = ({ lineas }) => (
  <div style={{ padding: "8px 24px", display: "flex", flexDirection: "column", gap: 8 }}>
    {lineas.map((l) => (
      <div key={l} style={{ fontSize: 25, color: C.mudo, lineHeight: 1.25 }}>
        {l}
      </div>
    ))}
  </div>
);

const NodoMini: React.FC<{ i: number }> = ({ i }) => (
  <div
    style={{
      margin: "10px 22px 0",
      padding: "10px 16px",
      borderRadius: 14,
      background: C.panelClaro,
      border: `2px solid ${C.borde}`,
      display: "flex",
      justifyContent: "space-between",
      alignItems: "center",
    }}
  >
    <span style={{ fontSize: 25, color: C.texto, fontWeight: 600 }}>nodo {i}</span>
    <span style={{ fontSize: 21, color: C.mudo, fontFamily: MONO }}>goroutines</span>
  </div>
);

export type ArquitecturaProps = { readonly segundos?: number };

export const Arquitectura: React.FC<ArquitecturaProps> = () => {
  const frame = useCurrentFrame();
  const fps = useFpsEscena(BASE_ARQUITECTURA);
  const yTcp = 470;
  const yHttp = 330;
  const marco = entre(frame, fps, 18, 18.8);
  return (
    <AbsoluteFill style={{ background: C.bg, fontFamily: FUENTE }}>
      <Titulo fps={fps} titulo="Arquitectura" bajada="las cuatro partes del enunciado, en Go salvo la interfaz" />

      <Caja fps={fps} x={70} y={260} w={340} h={330} color={C.ambar} aparece={1.2} desde={4.4} hasta={6.8}>
        <Encabezado letra="A" titulo="Cargador" color={C.ambar} />
        <Renglones lineas={["CSV de OEFA, 1,3 GB", "bloques por un canal a un pool de workers", "escribe silver y gold"]} />
      </Caja>
      <Linea fps={fps} x0={410} y0={yTcp} x1={520} y1={yTcp} a={6.2} b={6.8} color={C.ambar} />
      <Aparece fps={fps} a={6.4} style={{ left: 418, top: yTcp + 14, fontSize: 22, color: C.ambar, fontFamily: MONO }}>
        gold
      </Aparece>

      <Caja fps={fps} x={520} y={230} w={340} h={440} color={C.cian} aparece={1.9} desde={6.8} hasta={10}>
        <Encabezado letra="B" titulo="Clúster" color={C.cian} />
        <Renglones lineas={["K-Means: 3 regímenes por PM2,5 y PM10", "cada nodo, los parciales de su rango"]} />
        {[1, 2, 3].map((i) => (
          <NodoMini key={i} i={i} />
        ))}
      </Caja>

      <Linea fps={fps} x0={860} y0={yTcp} x1={1020} y1={yTcp} a={11} b={11.6} color={C.cian} grosor={6} />
      <Aparece fps={fps} a={11.2} style={{ left: 860, top: yTcp + 18, width: 160, textAlign: "center" }}>
        <div style={{ fontSize: 22, color: C.cian, fontFamily: MONO }}>TCP :{datos.puerto_nodos}</div>
        <div style={{ fontSize: 19, color: C.mudo }}>JSON por línea</div>
      </Aparece>
      <Viajero fps={fps} x0={1012} y0={yTcp - 30} x1={880} y1={yTcp - 30} a={11.7} b={12.6} texto="ITERAR" color={C.cian} />
      <Viajero fps={fps} x0={880} y0={yTcp - 30} x1={1012} y1={yTcp - 30} a={12.7} b={13.6} texto="PARCIAL" color={C.verde} />

      <Caja fps={fps} x={1020} y={230} w={400} h={400} color={C.violeta} aparece={2.6} desde={10} hasta={18}>
        <Encabezado letra="C" titulo="API coordinadora" color={C.violeta} />
        <Renglones
          lineas={[
            "net/http, sin frameworks",
            "REST y WebSocket",
            "JWT con rol usuario o admin",
            "coordina las rondas del clúster",
          ]}
        />
      </Caja>
      <Linea fps={fps} x0={1125} y0={630} x1={1125} y1={700} a={12.8} b={13.2} color={C.violeta} />
      <Linea fps={fps} x0={1325} y0={630} x1={1325} y1={700} a={13.4} b={13.8} color={C.violeta} />
      <Caja fps={fps} x={1020} y={700} w={195} h={140} color={C.verde} aparece={13} desde={13} hasta={15}>
        <div style={{ padding: "22px 20px" }}>
          <div style={{ fontSize: 28, fontWeight: 700, color: C.texto }}>MongoDB</div>
          <div style={{ fontSize: 21, color: C.mudo, marginTop: 6 }}>persistente</div>
        </div>
      </Caja>
      <Caja fps={fps} x={1225} y={700} w={195} h={140} color={C.rojo} aparece={13.6} desde={13.6} hasta={15}>
        <div style={{ padding: "22px 20px" }}>
          <div style={{ fontSize: 28, fontWeight: 700, color: C.texto }}>Redis</div>
          <div style={{ fontSize: 21, color: C.mudo, marginTop: 6 }}>último estado</div>
        </div>
      </Caja>

      <Linea fps={fps} x0={1420} y0={yHttp} x1={1520} y1={yHttp} a={15.2} b={15.7} color={C.verde} grosor={6} />
      <Aparece fps={fps} a={15.4} style={{ left: 1416, top: yHttp + 16, width: 110, textAlign: "center" }}>
        <div style={{ fontSize: 20, color: C.verde, fontFamily: MONO }}>:{datos.puerto_http}</div>
        <div style={{ fontSize: 18, color: C.mudo }}>HTTP y WS</div>
      </Aparece>
      <Caja fps={fps} x={1520} y={230} w={340} h={360} color={C.verde} aparece={3.3} desde={15.2} hasta={18}>
        <Encabezado letra="D" titulo="SPA en React" color={C.verde} />
        <Renglones lineas={["detrás de Caddy, mismo origen", "mapa, alertas, simulación", "entrenamiento y clúster"]} />
      </Caja>

      <div
        style={{
          position: "absolute",
          left: 490,
          top: 200,
          width: 1400,
          height: 750,
          borderRadius: 30,
          border: `3px dashed ${C.azul}`,
          opacity: marco,
        }}
      />
      <Aparece fps={fps} a={18.2} style={{ left: 520, top: 968, fontSize: 32, color: C.azul, fontFamily: MONO }}>
        docker compose up --scale nodo=3
      </Aparece>
      <Aparece fps={fps} a={18.5} style={{ left: 520, top: 1014, fontSize: 24, color: C.mudo }}>
        web · api · nodo ×3 · mongo · redis
      </Aparece>
      <Aparece fps={fps} a={18.5} style={{ left: 70, top: 640, width: 340, fontSize: 22, color: C.mudo, lineHeight: 1.3 }}>
        corre antes, fuera del Compose: el gold se monta en la API y en los nodos
      </Aparece>
    </AbsoluteFill>
  );
};
