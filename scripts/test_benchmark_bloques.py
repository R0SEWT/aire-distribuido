"""Regresiones de procedencia, orden y metadatos del benchmark de Windows.

Ejecutar desde la raíz: python -m unittest discover -s scripts -p test_benchmark_bloques.py
"""

import copy
from contextlib import redirect_stdout
import importlib.util
import io
import json
import tempfile
import unittest
from unittest.mock import patch
from pathlib import Path


RAIZ = Path(__file__).resolve().parents[1]


def cargar_modulo(nombre, ruta):
    spec = importlib.util.spec_from_file_location(nombre, ruta)
    modulo = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(modulo)
    return modulo


benchmark = cargar_modulo("benchmark_bloques", RAIZ / "scripts/benchmark-bloques.py")
tablas = cargar_modulo("tablas", RAIZ / "docs/informe/tablas.py")


class BenchmarkWindows(unittest.TestCase):
    def setUp(self):
        self.temporal = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporal.cleanup)
        self.ruta = Path(self.temporal.name)
        self.datos = json.loads((RAIZ / "reports/benchmark_cargador_windows.json").read_text(encoding="utf-8"))
        self.referencia = json.loads((RAIZ / "reports/benchmark_cargador.json").read_text(encoding="utf-8"))

    def informe(self, datos):
        (self.ruta / "benchmark_cargador_windows.json").write_text(json.dumps(datos), encoding="utf-8")
        informe = tablas.Informe(self.ruta, self.ruta, RAIZ / "reports")
        informe.datos["benchmark_cargador.json"] = self.referencia
        informe.benchmark_cargador_windows()
        return informe

    def test_orden_de_json_no_cambia_cifras_ni_tablas(self):
        original = self.informe(self.datos)
        tabla = (self.ruta / "benchmark_cargador_windows_workers.md").read_text(encoding="utf-8")
        desordenado = copy.deepcopy(self.datos)
        for corrida in [*desordenado["tamanos"], desordenado["workers"]]:
            corrida["mediciones"].reverse()
        nuevo = self.informe(desordenado)
        self.assertEqual(original.cifras, nuevo.cifras)
        self.assertEqual(tabla, (self.ruta / "benchmark_cargador_windows_workers.md").read_text(encoding="utf-8"))
        self.assertEqual(nuevo.problemas, [])

    def test_metadatos_inconsistentes_se_rechazan(self):
        for campo in ("go", "cpus", "chunk", "plataforma", "commit", "arbol_sucio"):
            with self.subTest(campo=campo):
                datos = copy.deepcopy(self.datos)
                registro = datos["tamanos"][0]
                if campo in ("commit", "arbol_sucio"):
                    registro["metadatos"][campo] = "alterado"
                else:
                    registro[campo] = "alterado"
                self.assertTrue(any(campo in p for p in self.informe(datos).problemas))

    def test_muestra_activaria_rechazo_final(self):
        self.datos["tamanos"][0]["entrada"] = "data/sample/oefa_real_muestra.csv"
        self.assertTrue(self.informe(self.datos).sinteticos)

    def test_dispersion_y_redondeo_desde_tiempos(self):
        cifras = self.informe(self.datos).cifras["benchmark_cargador_windows"]
        self.assertEqual(cifras["diferencia_ms"], "0,7")
        self.assertEqual(cifras["dispersion_max"], "2,0")
        self.assertEqual(cifras["speedup_medianas"], "5,36")
        self.assertEqual(cifras["commit"], self.datos["workers"]["metadatos"]["commit"][:7])

    def test_agrupacion_conserva_mediciones_y_orden(self):
        registros = [*self.datos["tamanos"], self.datos["workers"]]
        rutas = []
        for i, registro in enumerate(registros):
            registro = copy.deepcopy(registro)
            registro["host"] = "equipo-privado"
            registro["metadatos"]["maquina"] = "equipo-privado"
            registro["entrada"] = "C:/datos/oefa.csv"
            ruta = self.ruta / f"{i}.json"
            ruta.write_text(json.dumps(registro), encoding="utf-8")
            rutas.append(ruta)
        self.assertEqual(benchmark.agrupar(rutas), self.datos)

    def test_resumen_reordena_workers_sin_cambiar_valores(self):
        ruta = self.ruta / "agrupado.json"
        ruta.write_text(json.dumps(self.datos), encoding="utf-8")
        resumen = benchmark.resumir(ruta, "workers")
        self.assertIn("5.36", resumen)
        self.assertIn("4.81", resumen)
        self.datos["workers"]["mediciones"].reverse()
        ruta.write_text(json.dumps(self.datos), encoding="utf-8")
        cambiado = benchmark.resumir(ruta, "workers")
        # Solo cambia el hash del archivo, porque sus bytes se reordenaron.
        self.assertEqual(resumen.splitlines()[3:], cambiado.splitlines()[3:])

    def test_rechaza_tamanos_duplicados(self):
        with self.assertRaises(ValueError):
            datos = copy.deepcopy(self.datos)
            datos["tamanos"][1] = datos["tamanos"][0]
            self.informe(datos)

    def test_orden_elegido_se_conserva(self):
        self.assertEqual(benchmark.tamanos_en_orden("128,16,64"), [128, 16, 64])

    def preparar_cli(self):
        cargador = self.ruta / "cargar.exe"
        cargador.write_bytes(b"solo prueba; no se ejecuta")
        entrada = self.ruta / "muestra.csv"
        entrada.write_bytes(b"muestra para verificar el control del proceso")
        salida = self.ruta / "evidencias"
        argv = ["benchmark-bloques.py", str(cargador), str(entrada), str(salida),
                "--piloto", "--tamanos", "128,16,64"]
        return entrada, salida, argv

    def test_cli_ejecuta_orden_solicitado_y_lee_workers_por_valor(self):
        entrada, salida, argv = self.preparar_cli()
        bloques = []

        def cargador_simulado(comando, **opciones):
            bloques.append(comando[comando.index("-bloque") + 1])
            self.assertIs(opciones["shell"], False)
            self.assertEqual(opciones["timeout"], 600)
            corrida = Path(comando[comando.index("-reports") + 1])
            (corrida / "silver.csv").write_bytes(b"silver")
            (corrida / "gold.csv").write_bytes(b"gold")
            informe = {"sha256": benchmark.sha256(entrada), "identicos": True, "bloques": 10,
                       "mediciones": [{"workers": 4, "mediana_s": 0.5, "speedup": 2},
                                      {"workers": 1, "mediana_s": 1, "speedup": 1}]}
            (corrida / "benchmark_cargador.json").write_text(json.dumps(informe), encoding="utf-8")
            return benchmark.subprocess.CompletedProcess(comando, 0)

        with patch.object(benchmark.sys, "argv", argv), \
                patch.object(benchmark.subprocess, "run", side_effect=cargador_simulado), \
                redirect_stdout(io.StringIO()):
            benchmark.main()
        self.assertEqual(bloques, ["128KB", "16KB", "64KB"])
        resumen = (salida / "resumen.md").read_text(encoding="utf-8")
        self.assertIn("Orden de tamaños: 128, 16, 64 KiB", resumen)
        self.assertIn("1000.000 ms | 500.000 ms | 2.00", resumen)

    def test_cli_timeout_no_publica_resumen_incompleto(self):
        _, salida, argv = self.preparar_cli()
        with patch.object(benchmark.sys, "argv", argv), \
                patch.object(benchmark.subprocess, "run", side_effect=benchmark.subprocess.TimeoutExpired("cargar", 600)), \
                redirect_stdout(io.StringIO()):
            with self.assertRaisesRegex(SystemExit, "superó 600 s.*terminal.txt"):
                benchmark.main()
        self.assertFalse((salida / "resumen.md").exists())
        self.assertTrue((salida / "128KiB/terminal.txt").exists())


if __name__ == "__main__":
    unittest.main()
