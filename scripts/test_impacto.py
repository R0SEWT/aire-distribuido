"""Pruebas de los datos del frontend: población, impacto, series y mapa.

Ejecutar desde la raíz:

    python3 -m unittest discover -s scripts -p 'test_impacto.py'

Casi todo corre sin los datos sin versionar: se comprueba lo que está en el repositorio (las formas,
las sumas internas, que cuadre con la ficha y con reports/alerta_24h.json, y que lo esperado de cada
serie salga de sus propias lecturas con una segunda implementación de la regla). Con el silver y el
gold presentes (en data/, o donde diga la variable AIRE_DATOS) además se recalcula todo y se compara.
"""

import csv
import datetime as dt
import json
import os
import statistics
import sys
import unittest
from pathlib import Path

RAIZ = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(RAIZ / "scripts"))
import impacto  # noqa: E402
import mapa_peru  # noqa: E402

DATOS = Path(os.environ.get("AIRE_DATOS", RAIZ / "data"))
HAY_DATOS = all((DATOS / r).is_file() for r in impacto.RUTAS.values())
BANDAS = ["buena", "moderada", "mala", "umbral_de_cuidado"]
COLUMNAS_POBLACION = ["ubigeo", "departamento", "provincia", "distrito", "poblacion", "anio", "fuente"]


def leer_json(ruta):
    return json.loads((RAIZ / ruta).read_text(encoding="utf-8"))


def leer_csv(ruta):
    with open(RAIZ / ruta, newline="", encoding="utf-8") as f:
        return list(csv.DictReader(f))


def numeros(x):
    """Todos los números de un documento JSON."""
    if isinstance(x, dict):
        for v in x.values():
            yield from numeros(v)
    elif isinstance(x, list):
        for v in x:
            yield from numeros(v)
    elif isinstance(x, (int, float)) and not isinstance(x, bool):
        yield x


def instante(fecha_hora):
    return dt.datetime.fromisoformat(fecha_hora)


def entero(texto):
    """Una población del CSV: vacía si no se pudo confirmar en una fuente oficial."""
    return int(texto) if texto else None


# --- Una segunda implementación de la regla, sin el código de impacto.py ---------------------------

def media_ingenua(lecturas, estacion, fecha_hora):
    """La media de 24 h de una lectura mirando todas las de su estación: (t − 24 h, t]."""
    t = instante(fecha_hora)
    ventana = sorted((instante(l["fecha_hora"]), l["pm2_5"], l["pm10"]) for l in lecturas
                     if l["estacion"] == estacion and t - dt.timedelta(hours=24) < instante(l["fecha_hora"]) <= t)
    if len(ventana) < 18:
        return {"pm2_5": None, "pm10": None, "lecturas": len(ventana)}
    s25 = s10 = 0.0
    for _, a, b in ventana:
        s25 += a
        s10 += b
    return {"pm2_5": s25 / len(ventana), "pm10": s10 / len(ventana), "lecturas": len(ventana)}


def alerta_ingenua(media):
    return media["pm2_5"] is not None and (media["pm2_5"] > 50 or media["pm10"] > 100)


def inca_ingenuo(media):
    if media["pm2_5"] is None:
        return None
    por_pm10 = 0 if media["pm10"] <= 75 else 1 if media["pm10"] <= 150 else 2 if media["pm10"] <= 250 else 3
    por_pm25 = 0 if media["pm2_5"] <= 12.5 else 1 if media["pm2_5"] <= 25 else 2 if media["pm2_5"] <= 125 else 3
    return BANDAS[max(por_pm10, por_pm25)]


class Modelo:
    """El modelo sembrado, de los reportes versionados: z-score del gold y centroide más cercano."""

    def __init__(self):
        kmeans, est = leer_json("reports/kmeans.json"), leer_json("reports/estandarizacion.json")
        assert kmeans["features"] == est["features"] == ["pm2_5", "pm10"] and est["log1p"] == [] and est["nivel"] == "absoluto"
        self.media, self.desv, self.centroides = est["media"], est["desv"], kmeans["centroides_estandarizados"]
        self.excede = {r["regimen"]: r["eca"] == "excede" for r in leer_json("reports/regimenes.json")["regimenes"]}

    def regimen(self, pm25, pm10):
        z = [(pm25 - self.media[0]) / self.desv[0], (pm10 - self.media[1]) / self.desv[1]]
        distancias = [(z[0] - c[0]) ** 2 + (z[1] - c[1]) ** 2 for c in self.centroides]
        return distancias.index(min(distancias))


def resumen_aparte(valores):
    if not valores:
        return None
    v = sorted(valores)
    q1, mediana, q3 = statistics.quantiles(v, n=4, method="inclusive") if len(v) > 1 else (v[0], v[0], v[0])
    return {"n": len(v), "minimo": v[0], "q1": q1, "mediana": mediana, "q3": q3, "maximo": v[-1]}


# --- Población ---------------------------------------------------------------------------------------

class Poblacion(unittest.TestCase):
    def setUp(self):
        self.filas = leer_csv("reports/poblacion_distritos.csv")
        self.ficha = leer_csv("reports/estaciones_ficha.csv")

    def test_columnas_y_cifras(self):
        self.assertEqual(list(self.filas[0]), COLUMNAS_POBLACION)
        for f in self.filas:
            with self.subTest(ubigeo=f["ubigeo"]):
                self.assertRegex(f["ubigeo"], r"^\d{6}$")
                self.assertEqual(f["anio"], "2017")
                # Una cifra sin confirmar va vacía; si está, es un entero positivo con su fuente oficial.
                if f["poblacion"]:
                    self.assertGreater(int(f["poblacion"]), 0)
                    self.assertRegex(f["fuente"], r"^https://(www\.)?(inei\.gob\.pe|datosabiertos\.gob\.pe)/")

    def test_son_los_distritos_de_la_ficha_por_ubigeo(self):
        de_la_ficha = {e["ubigeo"]: (e["departamento"], e["provincia"], e["distrito"])
                       for e in self.ficha if e["ubigeo"] != "NA"}
        del_csv = {f["ubigeo"]: (f["departamento"], f["provincia"], f["distrito"]) for f in self.filas}
        self.assertEqual(len(self.filas), 16)
        self.assertEqual(del_csv, de_la_ficha)
        # LURIGANCHO (150118) es Lurigancho-Chosica, no San Juan de Lurigancho (150132).
        self.assertEqual(del_csv["150118"][2], "LURIGANCHO")
        self.assertNotIn("150132", del_csv)


# --- impacto.json ------------------------------------------------------------------------------------

class Impacto(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.d = leer_json("reports/impacto.json")
        cls.ficha = {e["estacion"]: e for e in leer_csv("reports/estaciones_ficha.csv")}
        cls.poblacion = {f["ubigeo"]: f for f in leer_csv("reports/poblacion_distritos.csv")}
        cls.alerta = leer_json("reports/alerta_24h.json")

    def test_la_copia_de_la_web_es_identica(self):
        self.assertEqual((RAIZ / "reports/impacto.json").read_bytes(),
                         (RAIZ / "web/public/datos/impacto.json").read_bytes())

    def test_los_totales_son_los_de_alerta_24h(self):
        t, a, s = self.d["totales"], self.alerta, self.d["senales"]
        self.assertEqual(t["lecturas"], a["gold"]["filas"])
        self.assertEqual(t["horas_calculables"], a["horas_calculables"])
        self.assertEqual(t["horas_sobre_eca"], a["horas_sobre_eca"])
        self.assertEqual(t["inca"], {b: a["inca"][b] for b in BANDAS})
        self.assertEqual((t["horas_calculables"], t["horas_sobre_eca"]), (64748, 1184))
        self.assertEqual(self.d["metadatos"]["fuentes"]["gold"]["sha256"], a["gold"]["sha256"])
        for nombre in ("regimen_alto", "lectura_horaria"):
            self.assertEqual(s[nombre]["horas"], a[nombre]["horas"])
            self.assertEqual(s[nombre]["aciertos"], a[nombre]["aciertos"])
            self.assertEqual(s[nombre]["precision"], a[nombre]["aciertos"] / a[nombre]["horas"])
            self.assertEqual(s[nombre]["cobertura"], a[nombre]["aciertos"] / a["horas_sobre_eca"])
        self.assertEqual(s["regimen_alto"]["regimen"], a["regimen_alto"]["regimen"])

    def test_las_reglas_son_las_de_la_ventana(self):
        r = self.d["metadatos"]["reglas"]
        self.assertEqual((r["ventana_horas"], r["minimo_lecturas"], r["eca_pm2_5"], r["eca_pm10"]), (24, 18, 50, 100))
        self.assertEqual((r["inca"]["bandas"], r["inca"]["cortes_pm10"], r["inca"]["cortes_pm2_5"]),
                         (BANDAS, [75, 150, 250], [12.5, 25, 125]))
        self.assertEqual((r["ventana_horas"], r["minimo_lecturas"], r["eca_pm2_5"], r["eca_pm10"]),
                         (self.alerta["ventana_horas"], self.alerta["minimo_lecturas"],
                          self.alerta["eca_pm2_5"], self.alerta["eca_pm10"]))
        for nombre in ("gold", "asignaciones", "silver", "ficha", "poblacion"):
            self.assertRegex(self.d["metadatos"]["fuentes"][nombre]["sha256"], r"^[0-9a-f]{64}$")
        dt.date.fromisoformat(self.d["metadatos"]["generado"])

    def test_cada_fila_es_coherente(self):
        for grupo in ("estaciones", "tipos_actividad", "distritos"):
            for fila in self.d[grupo]:
                with self.subTest(grupo=grupo, fila=fila.get("estacion") or fila.get("tipo_actividad") or fila["ubigeo"]):
                    self.assertEqual(list(fila["inca"]), BANDAS)
                    self.assertEqual(sum(fila["inca"].values()), fila["horas_calculables"])
                    self.assertLessEqual(fila["horas_sobre_eca"], fila["horas_calculables"])
                    self.assertLessEqual(fila["horas_calculables"], fila["lecturas"])
                    self.assertLessEqual(fila["primera_fecha"], fila["ultima_fecha"])
                    mas = max(BANDAS, key=lambda b: (fila["inca"][b], BANDAS.index(b)))
                    self.assertEqual(fila["banda_mas_frecuente"], mas)

    def test_las_tres_vistas_suman_los_totales(self):
        for grupo in ("estaciones", "tipos_actividad", "distritos"):
            with self.subTest(grupo=grupo):
                for campo in ("lecturas", "horas_calculables", "horas_sobre_eca"):
                    self.assertEqual(sum(f[campo] for f in self.d[grupo]), self.d["totales"][campo])
                for b in BANDAS:
                    self.assertEqual(sum(f["inca"][b] for f in self.d[grupo]), self.d["totales"]["inca"][b])
        codigos = sorted(e["estacion"] for e in self.d["estaciones"])
        self.assertEqual(sorted(x for t in self.d["tipos_actividad"] for x in t["estaciones"]), codigos)
        self.assertEqual(sorted(x for t in self.d["distritos"] for x in t["estaciones"]), codigos)

    def test_las_estaciones_son_las_17_del_gold_y_estan_en_la_ficha(self):
        con_gold = {c for c, e in self.ficha.items() if int(e["filas_gold"]) > 0}
        self.assertEqual({e["estacion"] for e in self.d["estaciones"]}, con_gold)
        self.assertEqual(len(con_gold), 17)
        self.assertEqual(sorted(self.d["alcance"]["estaciones_sin_pm_horario"]), sorted(set(self.ficha) - con_gold))
        self.assertEqual(self.d["alcance"]["estaciones_en_la_ficha"], 33)
        self.assertEqual(self.d["alcance"]["distritos_con_pm_horario"], 10)
        for e in self.d["estaciones"]:
            f = self.ficha[e["estacion"]]
            with self.subTest(estacion=e["estacion"]):
                self.assertEqual(e["lecturas"], int(f["filas_gold"]))
                self.assertEqual(e["objeto_fiscalizado"], f["objeto_fiscalizado"])
                self.assertEqual(e["tipo_actividad"], f["tipo_actividad"])
                self.assertEqual(e["ubigeo"], None if f["ubigeo"] == "NA" else f["ubigeo"])
                self.assertEqual(e["distrito"], None if f["distrito"] == "NA" else f["distrito"])
                self.assertGreaterEqual(e["primera_fecha"], f["fecha_inicio"])
                self.assertLessEqual(e["ultima_fecha"], f["fecha_fin"])
                self.assertLessEqual(e["dias_con_datos"], int(f["dias_con_datos"]))
                self.assertLessEqual(e["lecturas"], 24 * e["dias_con_datos"])

    def test_la_poblacion_es_la_del_csv_por_ubigeo(self):
        for e in self.d["estaciones"]:
            with self.subTest(estacion=e["estacion"]):
                if e["ubigeo"] is None:
                    self.assertIsNone(e["poblacion_distrito"])
                    continue
                p = self.poblacion[e["ubigeo"]]
                self.assertEqual(e["poblacion_distrito"],
                                 {"poblacion": entero(p["poblacion"]), "anio": int(p["anio"]), "fuente": p["fuente"]})
        for fila in self.d["distritos"] + [x for t in self.d["tipos_actividad"] for x in t["distritos"]]:
            if fila["ubigeo"] is not None:
                self.assertEqual(fila["poblacion"], entero(self.poblacion[fila["ubigeo"]]["poblacion"]))
        self.assertEqual([(p["ubigeo"], p["poblacion"]) for p in self.d["poblacion_distritos"]],
                         [(u, entero(p["poblacion"])) for u, p in sorted(self.poblacion.items())])

    def test_no_hay_ninguna_suma_de_poblacion(self):
        # §7.2: la población va fila por fila. Ningún número del archivo es la suma de dos o más distritos
        # de un mismo grupo (todos, los que tienen PM horario, o los de un tipo de actividad).
        todos = [entero(p["poblacion"]) or 0 for p in self.poblacion.values()]
        con_pm = [d["poblacion"] or 0 for d in self.d["distritos"]]
        prohibidos = {sum(todos), sum(con_pm)}
        prohibidos |= {sum(x["poblacion"] or 0 for x in t["distritos"])
                       for t in self.d["tipos_actividad"] if len(t["distritos"]) > 1}
        self.assertFalse(prohibidos & set(numeros(self.d)))

    def test_anticipacion(self):
        a = self.d["anticipacion"]
        episodios = a["episodios"]
        self.assertIn("partición temporal", a["lo_que_no_es"])
        self.assertIn("reentrenamiento", a["lo_que_no_es"])
        self.assertTrue(a["definicion"]["episodio"])
        self.assertEqual(a["regimen_que_excede"], self.alerta["regimen_alto"]["regimen"])
        # Toda hora sobre el ECA es de un episodio y solo de uno.
        self.assertEqual(sum(e["horas_sobre_eca"] for e in episodios), self.d["totales"]["horas_sobre_eca"])
        self.assertGreaterEqual(a["cruces_hacia_arriba"], len(episodios))
        por_estacion = {}
        for e in episodios:
            with self.subTest(episodio=(e["estacion"], e["inicio"])):
                self.assertLessEqual(e["inicio"], e["fin"])
                horas = (instante(e["fin"]) - instante(e["inicio"])) / dt.timedelta(hours=1) + 1
                self.assertLessEqual(e["horas_sobre_eca"], horas)
                self.assertTrue(0 <= e["horas_antes_lectura_horaria"] <= 23)
                self.assertTrue(e["horas_antes_regimen"] is None or 0 <= e["horas_antes_regimen"] <= 23)
                # La racha seguida no puede empezar antes de la primera vez que la señal se enciende.
                self.assertLessEqual(e["horas_seguidas_lectura_horaria"], e["horas_antes_lectura_horaria"])
                self.assertLessEqual(e["horas_seguidas_regimen"], e["horas_antes_regimen"] or 0)
                # Una lectura del régimen que excede no siempre supera el umbral horario, y al revés:
                # no se exige orden entre las dos señales.
                anterior = por_estacion.get(e["estacion"])
                if anterior:  # un episodio nuevo exige 24 horas sin ninguna hora sobre el ECA
                    self.assertGreater(instante(e["inicio"]) - instante(anterior["fin"]), dt.timedelta(hours=24))
                por_estacion[e["estacion"]] = e

        def resumen(grupo):
            def senal(nombre):
                v = [e["horas_antes_" + nombre] for e in grupo if e["horas_antes_" + nombre] is not None]
                return {"con_senal": len(v), "sin_senal": len(grupo) - len(v),
                        "con_senal_previa": sum(1 for x in v if x > 0), "en_el_tope": v.count(23),
                        "horas_antes": resumen_aparte(v),
                        "horas_seguidas": resumen_aparte([e["horas_seguidas_" + nombre] for e in grupo])}
            return {"episodios": len(grupo),
                    "con_media_previa": sum(e["media_previa_calculable"] for e in grupo),
                    "sin_media_previa": sum(not e["media_previa_calculable"] for e in grupo),
                    "sin_senal_previa": sum(1 for e in grupo if not e["horas_antes_lectura_horaria"]
                                            and not e["horas_antes_regimen"]),
                    "horas_sobre_eca": sum(e["horas_sobre_eca"] for e in grupo),
                    "lectura_horaria": senal("lectura_horaria"), "regimen": senal("regimen")}

        self.assertEqual(a["total"], resumen(episodios))
        self.assertEqual(a["solo_con_media_previa"], resumen([e for e in episodios if e["media_previa_calculable"]]))
        self.assertEqual([f["estacion"] for f in a["por_estacion"]], [e["estacion"] for e in self.d["estaciones"]])
        for fila in a["por_estacion"]:
            suyos = [e for e in episodios if e["estacion"] == fila["estacion"]]
            esperado = {"estacion": fila["estacion"], "cruces_hacia_arriba": fila["cruces_hacia_arriba"], **resumen(suyos)}
            self.assertEqual(fila, esperado)
            estacion = next(e for e in self.d["estaciones"] if e["estacion"] == fila["estacion"])
            self.assertEqual(fila["horas_sobre_eca"], estacion["horas_sobre_eca"])
        self.assertEqual(sum(f["cruces_hacia_arriba"] for f in a["por_estacion"]), a["cruces_hacia_arriba"])


# --- series.json -------------------------------------------------------------------------------------

class Series(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.s = leer_json("web/public/datos/series.json")
        cls.ficha = {e["estacion"]: e for e in leer_csv("reports/estaciones_ficha.csv")}
        cls.con_gold = {c for c, e in cls.ficha.items() if int(e["filas_gold"]) > 0}
        cls.modelo = Modelo()

    def tramos(self):
        e = self.s["escenario"]
        yield "escenario", e, e["lecturas"]
        for d in self.s["dias"]:
            yield d["id"], d, d["lecturas"]

    def test_el_modelo_de_la_prueba_es_el_sembrado(self):
        # El centroide más cercano de aquí da el régimen de los casos fijos de modelo/ (Go).
        casos = leer_json("modelo/testdata/casos_gold.json")["casos"]
        self.assertGreater(len(casos), 100)
        for c in casos:
            self.assertEqual(self.modelo.regimen(c["pm2_5"], c["pm10"]), c["regimen"], c)

    def test_la_ventana_portada_da_lo_mismo_que_la_de_go(self):
        # ventana/testdata/tramo_gold.json: 72 lecturas reales con lo que responde ventana.go.
        tramo = leer_json("ventana/testdata/tramo_gold.json")
        ventanas = impacto.Ventanas()
        for l in tramo["lecturas"]:
            m = ventanas.agregar(tramo["estacion"], impacto.hora_de(l["fecha_hora"]), l["pm2_5"], l["pm10"])
            visto = {"lecturas": m["lecturas"], "pm2_5": m["pm2_5"], "pm10": m["pm10"],
                     "alerta": impacto.alerta(m), "inca": impacto.inca(m)}
            self.assertEqual(visto, l["esperado"], l["fecha_hora"])

    def test_la_ventana_portada_guarda_72_y_acepta_desorden(self):
        ventanas = impacto.Ventanas()
        for h in range(100):
            ventanas.agregar("X", h, 1.0, 2.0)
        self.assertEqual([p[0] for p in ventanas.por_estacion["X"]], list(range(28, 100)))
        m = ventanas.agregar("X", 99, 9.0, 9.0)  # la misma hora reemplaza
        self.assertEqual((m["lecturas"], len(ventanas.por_estacion["X"])), (24, 72))
        self.assertEqual(m["pm2_5"], (23 * 1.0 + 9.0) / 24)
        m = ventanas.agregar("X", 10, 5.0, 5.0)  # una fecha lejana desplaza una lectura y queda sola
        self.assertEqual((m["lecturas"], m["pm2_5"], len(ventanas.por_estacion["X"])), (1, None, 72))

    def test_las_lecturas_se_pueden_mandar_a_la_api(self):
        for nombre, tramo, lecturas in self.tramos():
            with self.subTest(tramo=nombre):
                desde, hasta = instante(tramo["desde"]), instante(tramo["hasta"])
                self.assertEqual(hasta - desde, dt.timedelta(hours=47))
                self.assertEqual(tramo["horas"], 48)
                claves = [(l["fecha_hora"], l["estacion"]) for l in lecturas]
                self.assertEqual(claves, sorted(claves))  # por hora y, dentro de la hora, por estación
                self.assertEqual(len(set(claves)), len(claves))
                for l in lecturas:
                    self.assertEqual(list(l), ["estacion", "fecha_hora", "pm2_5", "pm10"])
                    self.assertIn(l["estacion"], self.con_gold)
                    self.assertRegex(l["fecha_hora"], r"^\d{4}-\d\d-\d\dT\d\d:00:00-05:00$")
                    self.assertTrue(desde <= instante(l["fecha_hora"]) <= hasta)
                    for campo in ("pm2_5", "pm10"):
                        self.assertIsInstance(l[campo], float)
                        self.assertTrue(0 <= l[campo] <= 10000)

    def test_el_escenario(self):
        e = self.s["escenario"]
        self.assertGreaterEqual(e["desde"], "2021-12-13T01:00:00-05:00")
        self.assertLessEqual(e["hasta"], "2022-01-01T00:00:00-05:00")
        self.assertEqual(e["dias"], [(instante(e["desde"]) - dt.timedelta(hours=1)).date().isoformat(),
                                     (instante(e["hasta"]) - dt.timedelta(hours=1)).date().isoformat()])
        cuenta = {}
        for l in e["lecturas"]:
            cuenta[l["estacion"]] = cuenta.get(l["estacion"], 0) + 1
        self.assertEqual({x["estacion"]: x["lecturas"] for x in e["estaciones"]}, cuenta)
        self.assertEqual(set(e["esperado"]), set(cuenta))
        self.assertEqual(sorted(e["estaciones_sin_lecturas"]), sorted(self.con_gold - set(cuenta)))
        # Intercaladas: en la primera hora ya hay más de una estación.
        self.assertGreater(len({l["estacion"] for l in e["lecturas"] if l["fecha_hora"] == e["desde"]}), 1)
        for x in e["estaciones"]:
            self.assertEqual(x["con_media_al_final"], e["esperado"][x["estacion"]]["media_24h"]["pm2_5"] is not None)
        # La spec (§5.4) dice que entre el 13 y el 31 hay de 10 a 14 estaciones a la vez.
        self.assertGreaterEqual(sum(x["con_media_al_final"] for x in e["estaciones"]), 10)

    def test_lo_esperado_sale_de_las_lecturas(self):
        for nombre, tramo, lecturas in self.tramos():
            esperados = tramo["esperado"] if nombre == "escenario" else {tramo["estacion"]: tramo["esperado"]}
            for estacion, esp in esperados.items():
                with self.subTest(tramo=nombre, estacion=estacion):
                    suyas = [l for l in lecturas if l["estacion"] == estacion]
                    ultima = suyas[-1]
                    self.assertEqual(esp["fecha_hora"], ultima["fecha_hora"])
                    self.assertEqual(esp["lecturas_enviadas"], len(suyas))
                    media = media_ingenua(lecturas, estacion, ultima["fecha_hora"])
                    self.assertEqual(esp["media_24h"], media)
                    self.assertEqual(list(esp["media_24h"]), ["pm2_5", "pm10", "lecturas"])
                    self.assertEqual(esp["alerta"], alerta_ingenua(media))
                    self.assertEqual(esp["inca"], inca_ingenuo(media))
                    regimen = self.modelo.regimen(ultima["pm2_5"], ultima["pm10"])
                    self.assertEqual(esp["regimen"], regimen)
                    self.assertEqual(esp["excede_eca"], self.modelo.excede[regimen])
                    medias = [(l["fecha_hora"], media_ingenua(lecturas, estacion, l["fecha_hora"])) for l in suyas]
                    self.assertEqual(esp["primera_media"], next((f for f, m in medias if m["pm2_5"] is not None), None))
                    self.assertEqual(esp["primera_alerta"], next((f for f, m in medias if alerta_ingenua(m)), None))

    def test_los_dias_sueltos(self):
        dias = self.s["dias"]
        self.assertTrue(7 <= len(dias) <= 10)
        self.assertEqual(len({d["id"] for d in dias}), len(dias))
        self.assertEqual(len({d["estacion"] for d in dias}), len(dias))  # de estaciones distintas
        for d in dias:
            with self.subTest(dia=d["id"]):
                self.assertEqual(d["id"], f"{d['estacion']}-{d['dia']}")
                self.assertEqual(d["dia"], (instante(d["hasta"]) - dt.timedelta(hours=1)).date().isoformat())
                self.assertEqual({l["estacion"] for l in d["lecturas"]}, {d["estacion"]})
                self.assertEqual(d["huecos"], 48 - len(d["lecturas"]))
                esp = d["esperado"]
                if d["caso"] == "limpio":
                    self.assertEqual((d["huecos"], esp["alerta"], esp["inca"], esp["primera_alerta"]), (0, False, "buena", None))
                elif d["caso"] == "mala_sin_alerta":
                    self.assertEqual((esp["alerta"], esp["inca"], esp["primera_alerta"]), (False, "mala", None))
                elif d["caso"] == "cruza_eca":
                    self.assertTrue(esp["alerta"] and esp["primera_media"] < esp["primera_alerta"])
                elif d["caso"] == "en_alerta":
                    self.assertTrue(esp["alerta"] and esp["primera_media"] == esp["primera_alerta"])
                elif d["caso"] == "umbral_de_cuidado":
                    self.assertEqual((esp["alerta"], esp["inca"]), (True, "umbral_de_cuidado"))
                elif d["caso"] == "huecos_con_media":
                    self.assertTrue(d["huecos"] > 0 and 18 <= esp["media_24h"]["lecturas"] < 24)
                elif d["caso"] == "huecos_sin_media":
                    self.assertTrue(d["huecos"] > 0 and esp["media_24h"]["lecturas"] < 18)
                    self.assertEqual((esp["media_24h"]["pm2_5"], esp["alerta"], esp["inca"]), (None, False, None))
                else:
                    self.fail(f"caso desconocido: {d['caso']}")
        casos = {d["caso"] for d in dias}
        self.assertTrue({"limpio", "cruza_eca", "huecos_con_media", "huecos_sin_media"} <= casos)
        self.assertGreaterEqual(sum(d["esperado"]["alerta"] for d in dias), 2)

    def test_los_dias_son_posteriores_al_escenario_y_estan_separados(self):
        e = self.s["escenario"]
        tramos = {}  # estación → [(primera lectura, última lectura, nombre)]
        for l in e["lecturas"]:
            a, b, _ = tramos.setdefault(l["estacion"], [(instante(l["fecha_hora"]), instante(l["fecha_hora"]), "escenario")])[0]
            tramos[l["estacion"]][0] = (min(a, instante(l["fecha_hora"])), max(b, instante(l["fecha_hora"])), "escenario")
        for d in self.s["dias"]:
            with self.subTest(dia=d["id"]):
                desde, hasta = instante(d["desde"]), instante(d["hasta"])
                # Ninguna fecha de un día suelto es anterior al escenario, sea o no de una estación suya.
                self.assertGreater(desde, instante(e["hasta"]))
                for a, b, nombre in tramos.get(d["estacion"], []):
                    separados = desde - b > dt.timedelta(hours=48) or a - hasta > dt.timedelta(hours=48)
                    self.assertTrue(separados, f"a 48 horas o menos de {nombre}")
                tramos.setdefault(d["estacion"], []).append((desde, hasta, d["id"]))


# --- peru.geojson y la carpeta -----------------------------------------------------------------------

class Mapa(unittest.TestCase):
    def test_los_departamentos_y_las_estaciones(self):
        ruta = RAIZ / "web/public/datos/peru.geojson"
        geo = json.loads(ruta.read_text(encoding="utf-8"))
        self.assertLess(ruta.stat().st_size, 300_000)
        self.assertEqual(mapa_peru.problemas(geo, mapa_peru.leer_ficha()), [])
        departamentos = {r["properties"]["departamento"] for r in geo["features"]}
        de_la_ficha = {e["departamento"] for e in mapa_peru.leer_ficha()} - {"NA"}
        self.assertLessEqual(de_la_ficha, departamentos)

    def test_una_coordenada_invertida_se_nota(self):
        geo = json.loads((RAIZ / "web/public/datos/peru.geojson").read_text(encoding="utf-8"))
        ficha = mapa_peru.leer_ficha()
        e = dict(ficha[0])
        e["lat"], e["lon"] = e["lon"], e["lat"]
        self.assertTrue(mapa_peru.problemas(geo, [e]))
        otro = dict(ficha[0], departamento="PUNO")
        self.assertTrue(mapa_peru.problemas(geo, [otro]))

    def test_el_readme_nombra_cada_archivo(self):
        carpeta = RAIZ / "web/public/datos"
        texto = (carpeta / "README.md").read_text(encoding="utf-8")
        for archivo in carpeta.iterdir():
            self.assertIn(f"`{archivo.name}`", texto)


# --- Con los datos sin versionar ---------------------------------------------------------------------

@unittest.skipUnless(HAY_DATOS, f"sin el silver y el gold en {DATOS} (o en AIRE_DATOS)")
class ConLosDatos(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.impacto, cls.series = impacto.generar(DATOS)

    def test_lo_versionado_es_lo_que_se_calcula_hoy(self):
        for ruta, documento in (("reports/impacto.json", self.impacto), ("web/public/datos/impacto.json", self.impacto),
                                ("web/public/datos/series.json", self.series)):
            with self.subTest(archivo=ruta):
                self.assertEqual(impacto.sin_fecha(leer_json(ruta)), impacto.sin_fecha(documento))

    def test_el_formato_de_series_no_pierde_nada(self):
        self.assertEqual(json.loads(impacto.texto_series(self.series)), self.series)

    def test_el_regimen_de_las_asignaciones_es_el_del_modelo(self):
        lecturas, _ = impacto.leer_lecturas(DATOS)
        modelo = Modelo()
        distintas = sum(1 for serie in lecturas.values() for _, a, b, g in serie if modelo.regimen(a, b) != g)
        self.assertEqual(distintas, 0)

    def test_los_totales_con_una_ventana_ingenua(self):
        # Las horas calculables y sobre el ECA de dos estaciones chicas, recontadas sin la clase Ventanas.
        lecturas, _ = impacto.leer_lecturas(DATOS)
        por_estacion = {e["estacion"]: e for e in self.impacto["estaciones"]}
        for estacion in ("CA-HU-04", "CA-RZC-02", "CA-SOC-01"):
            serie = [{"estacion": estacion, "fecha_hora": impacto.sello(h), "pm2_5": a, "pm10": b}
                     for h, a, b, _ in lecturas[estacion]]
            medias = [media_ingenua(serie, estacion, l["fecha_hora"]) for l in serie]
            calculables = [m for m in medias if m["pm2_5"] is not None]
            self.assertEqual(len(calculables), por_estacion[estacion]["horas_calculables"])
            self.assertEqual(sum(alerta_ingenua(m) for m in calculables), por_estacion[estacion]["horas_sobre_eca"])
            for b in BANDAS:
                self.assertEqual(sum(inca_ingenuo(m) == b for m in calculables), por_estacion[estacion]["inca"][b])


if __name__ == "__main__":
    unittest.main()
