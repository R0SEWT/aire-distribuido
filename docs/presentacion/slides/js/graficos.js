// Gráficos del deck con D3 (v7, vendorizado en js/d3.v7.min.js para que el deck funcione sin red).
//
// Cada gráfico es un <div data-grafico="nombre"> en index.qmd. Los datos salen de window.DATOS
// (datos.js, generado por datos.py desde reports/), nunca de cifras escritas aquí. Los colores y las
// fuentes los pone custom.scss a través de clases: aquí no hay un solo color.
//
// Ciclo de vida, atado a reveal.js:
//   entrar()  la lámina aparece: se anima desde cero
//   paso(n)   n = el mayor data-paso de los fragmentos visibles de la lámina
//   salir()   la lámina se va: se detiene (la animación del worker pool no corre de fondo)
//   final()   ?print-pdf: el estado completo, sin animar
(function () {
  "use strict";

  // ------------------------------------------------------------------ formato, como en el informe
  const miles = (n) => d3.format(",")(Math.round(n)).replace(/,/g, " ");
  const dec = (x, d = 1) => x.toFixed(d).replace(".", ",");
  const pct = (x, d = 1) => `${dec(100 * x, d)} %`;
  const veces = (x) => `${dec(x, 2)}×`;
  const NIVEL = { bajo: "Bajo", medio: "Medio", alto: "Alto" };

  function lienzo(el, ancho, alto, clase) {
    return d3.select(el).append("svg")
      .attr("viewBox", `0 0 ${ancho} ${alto}`)
      .attr("class", `g ${clase}`)
      .attr("role", "img");
  }

  // ------------------------------------------------------------------ embudo del cargador
  // Dos unidades distintas, en dos filas con su propia escala: filas largas del CSV (una lectura
  // cada una) y horas-estación (una fila ancha por estación y hora). No se mezclan en una barra.
  function embudo(el) {
    const D = window.DATOS.embudo;
    const ancho = 1180, alto = 410, x0 = 10, largo = ancho - 2 * x0;
    const svg = lienzo(el, ancho, alto, "g-embudo");

    const xa = d3.scaleLinear([0, D.leidas], [0, largo]);
    const fila1 = svg.append("g").attr("transform", "translate(0,52)");
    fila1.append("text").attr("class", "g-titulo").attr("x", x0).attr("y", -14)
      .text(`Filas del CSV de OEFA: ${miles(D.leidas)} lecturas`);
    const segmentos = [{ motivo: "lecturas válidas", filas: D.procesadas, clase: "s-valida" }]
      .concat(D.motivos.map((m, i) => ({ ...m, clase: `s-desc s-desc-${i}` })));
    let acum = 0;
    segmentos.forEach((s) => { s.x = acum; acum += s.filas; });
    const barras1 = fila1.selectAll("rect.seg").data(segmentos).join("rect")
      .attr("class", (d) => `seg ${d.clase}`)
      .attr("x", (d) => x0 + xa(d.x)).attr("y", 0).attr("height", 46).attr("width", 0);
    fila1.append("text").attr("class", "g-dentro").attr("x", x0 + 14).attr("y", 31)
      .text(`${miles(D.procesadas)} válidas`);
    fila1.append("text").attr("class", "g-dentro g-dentro-osc").attr("x", x0 + xa(D.procesadas) + 14)
      .attr("y", 31).text(`${miles(D.descartadas)} descartadas, por regla`);

    // Leyenda de los motivos de descarte, en dos columnas
    const ley = svg.append("g").attr("class", "g-ley").attr("transform", "translate(0,128)");
    const items = ley.selectAll("g").data(D.motivos).join("g")
      .attr("transform", (d, i) => `translate(${x0 + (i % 2) * 580},${Math.floor(i / 2) * 32})`);
    items.append("rect").attr("class", (d, i) => `s-desc s-desc-${i}`).attr("width", 20).attr("height", 20).attr("y", -15);
    items.append("text").attr("x", 30).text((d) => d.motivo);
    items.append("text").attr("class", "g-num").attr("x", 540).attr("text-anchor", "end").text((d) => miles(d.filas));

    // Fila 2: horas-estación
    const xb = d3.scaleLinear([0, D.anchas], [0, largo]);
    const fila2 = svg.append("g").attr("class", "g-fila2").attr("transform", "translate(0,320)");
    fila2.append("text").attr("class", "g-titulo").attr("x", x0).attr("y", -14)
      .text(`Horas-estación: ${miles(D.anchas)} filas anchas de ${D.estaciones} estaciones`);
    const total2 = fila2.append("rect").attr("class", "seg s-anchas").attr("x", x0).attr("height", 46).attr("width", 0);
    const gold = fila2.append("rect").attr("class", "seg s-gold").attr("x", x0).attr("height", 46).attr("width", 0);
    // La etiqueta va a la derecha del tramo del gold, en oscuro: dentro no cabe
    const etGold = fila2.append("text").attr("class", "g-dentro g-dentro-osc").attr("x", x0 + xb(D.completas) + 14)
      .attr("y", 31).text(`${miles(D.completas)} con PM2,5 y PM10: el gold (${D.estaciones_gold} estaciones)`);

    function mostrar(n, animar) {
      const t = (sel) => (animar ? sel.transition().duration(900).ease(d3.easeCubicOut) : sel);
      t(barras1).attr("width", (d) => (n >= 1 || d.clase === "s-valida" ? xa(d.filas) : 0));
      if (n < 1) {
        // Antes del primer clic, la barra entera se ve como «leídas», sin partir
        t(barras1.filter((d) => d.clase === "s-valida")).attr("width", xa(D.leidas));
      }
      fila1.selectAll(".g-dentro").attr("opacity", n >= 1 ? 1 : 0);
      ley.attr("opacity", n >= 1 ? 1 : 0);
      fila2.attr("opacity", n >= 2 ? 1 : 0);
      t(total2).attr("width", n >= 2 ? xb(D.anchas) : 0);
      t(gold).attr("width", n >= 2 ? xb(D.completas) : 0);
      etGold.attr("opacity", n >= 2 ? 1 : 0);
    }
    return {
      entrar() { barras1.attr("width", 0); mostrar(0, true); },
      paso(n) { mostrar(n, true); },
      final() { mostrar(2, false); },
    };
  }

  // ------------------------------------------------------------------ speedup con su intervalo
  // data-series="cargador" o "kmeans_1000,kmeans_10000". La diagonal ideal, en la misma escala:
  // el gráfico muestra cuánto falta, no lo esconde.
  function speedup(el) {
    const S = window.DATOS.speedup;
    const nombres = (el.dataset.series || "cargador").split(",");
    const etiqueta = {
      cargador: "cargador, CSV completo",
      kmeans_1000: `K-Means, chunks de 1 000 filas (${S.chunks_1000} chunks)`,
      kmeans_10000: `K-Means, chunks de 10 000 filas (${S.chunks_10000} chunks)`,
    };
    const ancho = 980, alto = 470, m = { t: 20, r: 330, b: 64, l: 70 };
    const svg = lienzo(el, ancho, alto, "g-speedup");
    const x = d3.scaleLinear([1, 12], [m.l, ancho - m.r]);
    const y = d3.scaleLinear([0, 12], [alto - m.b, m.t]);
    svg.append("g").attr("class", "g-eje").attr("transform", `translate(0,${alto - m.b})`)
      .call(d3.axisBottom(x).tickValues([1, 2, 4, 8, 12]).tickSizeOuter(0));
    svg.append("g").attr("class", "g-eje").attr("transform", `translate(${m.l},0)`)
      .call(d3.axisLeft(y).tickValues([2, 4, 6, 8, 10, 12]).tickFormat((v) => `${v}×`).tickSizeOuter(0));
    svg.append("text").attr("class", "g-et").attr("x", (m.l + ancho - m.r) / 2).attr("y", alto - 12)
      .attr("text-anchor", "middle").text("workers (W)");
    svg.append("line").attr("class", "g-ideal").attr("x1", x(1)).attr("y1", y(1)).attr("x2", x(12)).attr("y2", y(12));
    svg.append("text").attr("class", "g-et g-ideal-t").attr("x", x(6) - 10).attr("y", y(6) - 10)
      .attr("text-anchor", "end").text("ideal: W×");

    const area = d3.area().x((d) => x(d.w)).y0((d) => y(d.inf)).y1((d) => y(d.sup));
    const linea = d3.line().x((d) => x(d.w)).y((d) => y(d.punto));
    const capas = nombres.map((n, i) => {
      const g = svg.append("g").attr("class", `g-serie g-serie-${n}`).attr("opacity", 0);
      g.append("path").attr("class", "g-ic").attr("d", area(S[n]));
      const p = g.append("path").attr("class", "g-linea").attr("d", linea(S[n]));
      g.selectAll("circle").data(S[n]).join("circle").attr("r", 6).attr("cx", (d) => x(d.w)).attr("cy", (d) => y(d.punto));
      const u = S[n][S[n].length - 1];
      g.append("text").attr("class", "g-valor").attr("x", x(u.w) + 14).attr("y", y(u.punto) + 8)
        .text(veces(u.punto));
      g.append("text").attr("class", "g-et").attr("x", x(u.w) + 14).attr("y", y(u.punto) + 34)
        .text(`IC 95 %: ${dec(u.inf, 2)}–${dec(u.sup, 2)}`);
      // Leyenda arriba a la derecha, una línea por serie
      g.append("text").attr("class", "g-ley-t").attr("x", ancho - m.r + 20).attr("y", 40 + i * 64)
        .call((t) => etiqueta[n].split(", ").forEach((parte, j) =>
          t.append("tspan").attr("x", ancho - m.r + 20).attr("dy", j ? 26 : 0).text(parte + (j === 0 && etiqueta[n].includes(", ") ? "," : ""))));
      return { g, p };
    });

    function mostrar(n, animar) {
      capas.forEach(({ g, p }, i) => {
        const visible = i <= n;
        g.attr("opacity", visible ? 1 : 0);
        if (visible && animar) {
          const L = p.node().getTotalLength();
          p.attr("stroke-dasharray", `${L} ${L}`).attr("stroke-dashoffset", L)
            .transition().duration(1100).ease(d3.easeCubicOut).attr("stroke-dashoffset", 0)
            .on("end", () => p.attr("stroke-dasharray", null));
        }
      });
    }
    let actual = -1;
    return {
      entrar() { actual = 0; mostrar(0, true); },
      paso(n) { if (n !== actual) { actual = n; mostrar(n, true); } },
      final() { mostrar(capas.length - 1, false); },
    };
  }

  // ------------------------------------------------------------------ el worker pool, animado
  // Ilustrativo (los tiempos no son mediciones): C chunks van por el canal «trabajos» a W workers
  // que viven toda la corrida; cada worker escribe solo P[b], el parcial de su chunk; la barrera
  // espera los C; Fusionar suma en orden de chunk aunque hayan llegado en otro orden; Actualizar
  // mueve los centroides y empieza la iteración siguiente.
  function pool(el) {
    const C = 12;
    let W = +(el.dataset.w || 4);
    const ancho = 1180, alto = 486, x0 = 30, ancho_util = ancho - 2 * x0;
    const paso = ancho_util / C, lado = Math.min(58, paso - 12);
    const yCola = 66, yWorkers = 160, ySlots = 326, yBarrera = 414;
    const svg = lienzo(el, ancho, alto, "g-pool");
    const xChunk = (b) => x0 + b * paso + (paso - lado) / 2;

    svg.append("text").attr("class", "g-titulo").attr("x", x0).attr("y", yCola - 20)
      .text("canal trabajos: índices de chunk");
    svg.append("text").attr("class", "g-titulo").attr("x", x0).attr("y", ySlots - 20)
      .text("parciales P[b]: cada worker escribe solo el de su chunk");

    // Las ranuras de la cola y de los parciales, alineadas: P[b] queda debajo del chunk b
    svg.append("g").selectAll("rect").data(d3.range(C)).join("rect").attr("class", "p-ranura")
      .attr("x", xChunk).attr("y", yCola).attr("width", lado).attr("height", lado).attr("rx", 8);
    const slots = svg.append("g").selectAll("rect").data(d3.range(C)).join("rect").attr("class", "p-slot")
      .attr("x", xChunk).attr("y", ySlots).attr("width", lado).attr("height", lado).attr("rx", 8);
    svg.append("g").selectAll("text").data(d3.range(C)).join("text").attr("class", "p-slot-t")
      .attr("x", (b) => xChunk(b) + lado / 2).attr("y", ySlots + lado + 24).attr("text-anchor", "middle")
      .text((b) => `P[${b}]`);

    const gWorkers = svg.append("g");
    const tiles = svg.append("g").selectAll("g").data(d3.range(C)).join("g").attr("class", "p-tile");
    tiles.append("rect").attr("width", lado).attr("height", lado).attr("rx", 8);
    tiles.append("text").attr("x", lado / 2).attr("y", lado / 2 + 9).attr("text-anchor", "middle").text((b) => b);

    // Barrera y fusión
    const barrera = svg.append("g").attr("transform", `translate(${x0},${yBarrera})`);
    barrera.append("rect").attr("class", "p-barrera-fondo").attr("width", ancho_util).attr("height", 14).attr("rx", 7);
    const barra = barrera.append("rect").attr("class", "p-barrera").attr("height", 14).attr("rx", 7).attr("width", 0);
    const barreraT = barrera.append("text").attr("class", "g-et").attr("y", 44);
    const cursor = svg.append("rect").attr("class", "p-cursor").attr("y", ySlots - 6).attr("width", lado + 12)
      .attr("height", lado + 12).attr("rx", 11).attr("opacity", 0);
    const llegadaT = svg.append("text").attr("class", "g-et p-orden").attr("x", x0).attr("y", alto - 6);
    const iterT = svg.append("text").attr("class", "g-valor p-iter").attr("x", ancho - x0).attr("y", yCola - 20)
      .attr("text-anchor", "end");

    let workers = [];
    function dibujarWorkers() {
      const ancho_w = Math.min(220, (ancho_util - (W - 1) * 14) / W);
      const total = W * ancho_w + (W - 1) * 14, xi = x0 + (ancho_util - total) / 2;
      workers = d3.range(W).map((w) => ({ w, x: xi + w * (ancho_w + 14), ancho: ancho_w }));
      gWorkers.selectAll("*").remove();
      const g = gWorkers.selectAll("g").data(workers).join("g").attr("transform", (d) => `translate(${d.x},${yWorkers})`);
      g.append("rect").attr("class", "p-worker").attr("width", (d) => d.ancho).attr("height", 118).attr("rx", 12);
      g.append("text").attr("class", "p-worker-t").attr("x", 12).attr("y", 26).text((d) => (W > 6 ? `w${d.w + 1}` : `worker ${d.w + 1}`));
      g.append("rect").attr("class", "p-prog-fondo").attr("x", 12).attr("y", 102).attr("width", (d) => d.ancho - 24).attr("height", 8).attr("rx", 4);
      g.append("rect").attr("class", "p-prog").attr("x", 12).attr("y", 102).attr("width", 0).attr("height", 8).attr("rx", 4);
    }

    let token = { vivo: false };
    const esperar = (ms, tk) => new Promise((ok) => setTimeout(ok, ms)).then(() => { if (!tk.vivo) throw tk; });
    const fin = (tr, tk) => tr.end().catch(() => {}).then(() => { if (!tk.vivo) throw tk; });

    function reiniciar() {
      tiles.interrupt().attr("transform", (b) => `translate(${xChunk(b)},${yCola})`).attr("class", "p-tile");
      slots.attr("class", "p-slot");
      barra.interrupt().attr("width", 0).attr("class", "p-barrera");
      barreraT.text(`barrera.Wait(): 0 de ${C} parciales`);
      cursor.interrupt().attr("opacity", 0);
      gWorkers.selectAll(".p-prog").interrupt().attr("width", 0);
    }

    async function iteracion(t, tk) {
      reiniciar();
      iterT.text(`iteración ${t} · W = ${W}`);
      llegadaT.text("");
      const azar = d3.randomLcg(t * 7919 + W);
      const llegada = [];
      let siguiente = 0, listos = 0;
      const t0 = performance.now();
      await Promise.all(workers.map(async (wk) => {
        const prog = gWorkers.selectAll(".p-prog").filter((d) => d.w === wk.w);
        while (siguiente < C) {
          const b = siguiente++;
          const tile = tiles.filter((d) => d === b).raise();
          await fin(tile.transition().duration(380).ease(d3.easeCubicInOut)
            .attr("transform", `translate(${wk.x + (wk.ancho - lado) / 2},${yWorkers + 36})`), tk);
          tile.attr("class", "p-tile p-tile-calcula");
          const dur = 650 + 900 * azar();
          prog.attr("width", 0);
          await fin(prog.transition().duration(dur).ease(d3.easeLinear).attr("width", wk.ancho - 24), tk);
          await fin(tile.transition().duration(380).ease(d3.easeCubicInOut)
            .attr("transform", `translate(${xChunk(b)},${ySlots})`), tk);
          tile.attr("class", "p-tile p-tile-listo");
          slots.filter((d) => d === b).attr("class", "p-slot p-slot-listo");
          llegada.push(b);
          listos++;
          barra.transition().duration(200).attr("width", (ancho_util * listos) / C);
          barreraT.text(`barrera.Wait(): ${listos} de ${C} parciales`);
          llegadaT.text(`orden de llegada: ${llegada.join(" · ")}`);
        }
      }));
      const seg = (performance.now() - t0) / 1000;
      barra.attr("class", "p-barrera p-barrera-ok");
      barreraT.text(`barrera superada en ${dec(seg, 1)} s (ilustrativo): ahora sí se escriben los centroides`);
      await esperar(700, tk);
      // Fusionar: en orden de chunk, sin importar el de llegada
      cursor.attr("opacity", 1).attr("x", xChunk(0) - 6);
      llegadaT.text(`orden de llegada: ${llegada.join(" · ")}   →   Fusionar en orden de chunk: 0 · 1 · 2 · … · ${C - 1}`);
      for (let b = 0; b < C; b++) {
        await fin(cursor.transition().duration(170).attr("x", xChunk(b) - 6), tk);
        tiles.filter((d) => d === b).attr("class", "p-tile p-tile-fusionado");
      }
      await fin(cursor.transition().duration(250).attr("opacity", 0), tk);
      barreraT.text("Actualizar(c, F): centroides nuevos, y empieza la iteración siguiente");
      await esperar(1600, tk);
    }

    async function correr(tk) {
      let t = 1;
      try {
        for (;;) { await iteracion(t++, tk); }
      } catch (e) {
        if (e !== tk) throw e;
      }
    }

    // Botones W = 1, 2, 4, 8 (HTML, debajo del gráfico)
    const botones = d3.select(el).append("div").attr("class", "p-botones");
    botones.append("span").attr("class", "p-botones-t").text("workers:");
    botones.selectAll("button").data([1, 2, 4, 8]).join("button")
      .attr("class", (d) => (d === W ? "activo" : null)).text((d) => `W = ${d}`)
      .on("click", (ev, d) => {
        ev.stopPropagation();
        W = d;
        botones.selectAll("button").attr("class", (v) => (v === W ? "activo" : null));
        api.salir();
        dibujarWorkers();
        api.entrar();
      });

    dibujarWorkers();
    reiniciar();
    const api = {
      entrar() { token.vivo = false; token = { vivo: true }; correr(token); },
      salir() { token.vivo = false; svg.selectAll("*").interrupt(); },
      final() {
        // En PDF: la iteración ya cerrada, con los parciales listos y la barrera superada
        tiles.attr("transform", (b) => `translate(${xChunk(b)},${ySlots})`).attr("class", "p-tile p-tile-fusionado");
        slots.attr("class", "p-slot p-slot-listo");
        barra.attr("width", ancho_util).attr("class", "p-barrera p-barrera-ok");
        barreraT.text(`barrera superada: los ${C} parciales, fusionados en orden de chunk`);
        iterT.text(`W = ${W}`);
      },
    };
    return api;
  }

  // ------------------------------------------------------------------ los tres niveles frente al ECA
  function niveles(el) {
    const N = window.DATOS.niveles, E = window.DATOS.eca;
    const ancho = 1180, alto = 450, m = { t: 56, b: 92, l: 64 };
    const svg = lienzo(el, ancho, alto, "g-niveles");
    const paneles = [
      { clave: "pm2_5", titulo: "PM2,5 medio (µg/m³)", eca: E.pm2_5, x0: 0 },
      { clave: "pm10", titulo: "PM10 medio (µg/m³)", eca: E.pm10, x0: ancho / 2 + 20 },
    ];
    const anchoPanel = ancho / 2 - 40;
    const barras = [];
    paneles.forEach((p) => {
      const g = svg.append("g").attr("transform", `translate(${p.x0},0)`);
      const max = Math.max(p.eca, d3.max(N, (d) => d[p.clave])) * 1.12;
      const y = d3.scaleLinear([0, max], [alto - m.b, m.t]);
      const x = d3.scaleBand(N.map((d) => d.nivel), [m.l, anchoPanel]).padding(0.28);
      g.append("text").attr("class", "g-titulo").attr("x", m.l).attr("y", 24).text(p.titulo);
      g.append("g").attr("class", "g-eje").attr("transform", `translate(${m.l},0)`)
        .call(d3.axisLeft(y).ticks(5).tickSizeOuter(0));
      const b = g.selectAll("rect.n").data(N).join("rect").attr("class", (d) => `n n-${d.nivel}`)
        .attr("x", (d) => x(d.nivel)).attr("width", x.bandwidth())
        .attr("y", y(0)).attr("height", 0)
        .attr("data-y", (d) => y(d[p.clave])).attr("data-h", (d) => y(0) - y(d[p.clave]));
      barras.push(b);
      g.selectAll("text.v").data(N).join("text").attr("class", "g-valor v")
        .attr("x", (d) => x(d.nivel) + x.bandwidth() / 2).attr("y", (d) => y(d[p.clave]) - 10)
        .attr("text-anchor", "middle").text((d) => dec(d[p.clave], 1));
      g.selectAll("text.c").data(N).join("text").attr("class", "g-et c")
        .attr("x", (d) => x(d.nivel) + x.bandwidth() / 2).attr("y", alto - m.b + 30).attr("text-anchor", "middle")
        .text((d) => NIVEL[d.nivel]);
      g.selectAll("text.h").data(N).join("text").attr("class", "g-et g-suave h")
        .attr("x", (d) => x(d.nivel) + x.bandwidth() / 2).attr("y", alto - m.b + 58).attr("text-anchor", "middle")
        .text((d) => pct(d.fraccion));
      g.append("text").attr("class", "g-et g-suave").attr("x", m.l - 12).attr("y", alto - m.b + 58)
        .attr("text-anchor", "end").text("horas");
      g.append("line").attr("class", "g-eca").attr("x1", m.l).attr("x2", anchoPanel).attr("y1", y(p.eca)).attr("y2", y(p.eca));
      // A la izquierda, sobre las barras bajas: a la derecha la taparía la del nivel alto
      g.append("text").attr("class", "g-eca-t").attr("x", m.l + 8).attr("y", y(p.eca) - 10)
        .text(`ECA de 24 h: ${p.eca}`);
    });
    // La altura final de cada barra quedó en data-y y data-h al crearla
    const arriba = function () { return +this.dataset.y; };
    const alto_ = function () { return +this.dataset.h; };
    const base = function () { return +this.dataset.y + +this.dataset.h; };
    function mostrar(animar) {
      barras.forEach((b) => (animar ? b.transition().duration(900).delay((d, i) => 150 * i) : b)
        .attr("y", arriba).attr("height", alto_));
    }
    return {
      entrar() { barras.forEach((b) => b.interrupt().attr("y", base).attr("height", 0)); mostrar(true); },
      final() { mostrar(false); },
    };
  }

  // ------------------------------------------------------------------ cuándo: el nivel alto por hora
  function horas(el) {
    const alto_ = window.DATOS.niveles.find((d) => d.nivel === "alto");
    const datos = alto_.por_hora.map((n, h) => ({ h, n }));
    const pico = datos.reduce((a, b) => (b.n > a.n ? b : a));
    const ancho = 700, alto = 430, m = { t: 40, r: 10, b: 70, l: 58 };
    const svg = lienzo(el, ancho, alto, "g-horas");
    const x = d3.scaleBand(d3.range(24), [m.l, ancho - m.r]).padding(0.18);
    const y = d3.scaleLinear([0, pico.n * 1.15], [alto - m.b, m.t]);
    svg.append("g").attr("class", "g-eje").attr("transform", `translate(0,${alto - m.b})`)
      .call(d3.axisBottom(x).tickValues([0, 3, 6, 9, 12, 15, 18, 21]).tickFormat((h) => `${String(h).padStart(2, "0")}h`).tickSizeOuter(0));
    svg.append("g").attr("class", "g-eje").attr("transform", `translate(${m.l},0)`)
      .call(d3.axisLeft(y).ticks(4).tickSizeOuter(0));
    svg.append("text").attr("class", "g-et").attr("x", m.l).attr("y", 18).text("horas en nivel alto, por hora del día");
    const b = svg.selectAll("rect").data(datos).join("rect")
      .attr("class", (d) => (d.h === pico.h ? "n n-alto" : "n n-alto-suave"))
      .attr("x", (d) => x(d.h)).attr("width", x.bandwidth()).attr("y", y(0)).attr("height", 0);
    svg.append("text").attr("class", "g-valor").attr("x", x(pico.h) + x.bandwidth() / 2).attr("y", y(pico.n) - 12)
      .attr("text-anchor", "middle").text(`${String(pico.h).padStart(2, "0")}:00`);
    svg.append("text").attr("class", "g-et g-suave").attr("x", m.l).attr("y", alto - 10)
      .text("la hora es la del fin del promedio: 08h = de 07:00 a 08:00");
    const mostrar = (animar) => (animar ? b.transition().duration(800).delay((d) => 25 * d.h) : b)
      .attr("y", (d) => y(d.n)).attr("height", (d) => y(0) - y(d.n));
    return {
      entrar() { b.attr("y", y(0)).attr("height", 0); mostrar(true); },
      final() { mostrar(false); },
    };
  }

  // ------------------------------------------------------------------ dónde: estaciones del nivel alto
  function estaciones(el) {
    const alto_ = window.DATOS.niveles.find((d) => d.nivel === "alto");
    const E = alto_.estaciones;
    const ancho = 520, alto = 430, m = { t: 40, l: 10 };
    const svg = lienzo(el, ancho, alto, "g-estaciones");
    svg.append("text").attr("class", "g-et").attr("x", m.l).attr("y", 18).text("de dónde salen las horas del nivel alto");
    const x = d3.scaleLinear([0, d3.max(E, (d) => d.fraccion)], [0, ancho - 130]);
    const fila = 74;
    const g = svg.selectAll("g.f").data(E).join("g").attr("class", "f")
      .attr("transform", (d, i) => `translate(${m.l},${m.t + 30 + i * fila})`);
    g.append("text").attr("class", "g-titulo").attr("y", 0).text((d) => d.estacion);
    g.append("text").attr("class", "g-et g-suave").attr("x", 150).attr("y", 0)
      .text((d) => `${d.departamento}${d.altitud ? ` · ${miles(+d.altitud)} m` : ""}`);
    const b = g.append("rect").attr("class", (d, i) => (i === 0 ? "n n-alto" : "n n-alto-suave")).attr("y", 12).attr("height", 24).attr("width", 0);
    g.append("text").attr("class", "g-valor").attr("y", 32).attr("x", (d) => x(d.fraccion) + 10).text((d) => pct(d.fraccion, 0));
    const mostrar = (animar) => (animar ? b.transition().duration(800).delay((d, i) => 120 * i) : b).attr("width", (d) => x(d.fraccion));
    return {
      entrar() { b.attr("width", 0); mostrar(true); },
      final() { mostrar(false); },
    };
  }

  // ------------------------------------------------------------------ enlace con reveal.js
  const GRAFICOS = { embudo, speedup, pool, niveles, horas, estaciones };
  const instancias = [];

  function pasoVisible(lamina) {
    return d3.max(Array.from(lamina.querySelectorAll(".fragment.visible[data-paso]")), (f) => +f.dataset.paso) || 0;
  }

  function alCambiar(actual, anterior) {
    instancias.forEach(({ el, g }) => {
      if (anterior && anterior.contains(el) && g.salir) g.salir();
    });
    instancias.forEach(({ el, g }) => {
      if (actual && actual.contains(el)) {
        if (g.entrar) g.entrar();
        if (g.paso) g.paso(pasoVisible(actual));
      }
    });
  }

  function alFragmento(ev) {
    const lamina = ev.fragment && ev.fragment.closest("section");
    if (!lamina) return;
    instancias.forEach(({ el, g }) => {
      if (lamina.contains(el) && g.paso) g.paso(pasoVisible(lamina));
    });
  }

  function iniciar() {
    document.querySelectorAll("[data-grafico]").forEach((el) => {
      const f = GRAFICOS[el.dataset.grafico];
      if (!f) { console.warn("gráfico desconocido:", el.dataset.grafico); return; }
      instancias.push({ el, g: f(el) });
    });
    if (/print-pdf/.test(window.location.search)) {
      instancias.forEach(({ g }) => g.final && g.final());
      return;
    }
    Reveal.on("slidechanged", (ev) => alCambiar(ev.currentSlide, ev.previousSlide));
    Reveal.on("fragmentshown", alFragmento);
    Reveal.on("fragmenthidden", alFragmento);
    alCambiar(Reveal.getCurrentSlide(), null);
  }

  function cuandoReveal() {
    if (!window.Reveal) { setTimeout(cuandoReveal, 50); return; }
    if (Reveal.isReady && Reveal.isReady()) iniciar();
    else Reveal.on("ready", iniciar);
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", cuandoReveal);
  else cuandoReveal();
})();
