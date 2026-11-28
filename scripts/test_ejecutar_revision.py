# /// script
# requires-python = ">=3.12"
# dependencies = ["nbclient", "ipykernel", "pandas", "matplotlib", "scikit-learn", "scipy", "joblib"]
# ///
import importlib.util
import json
import tempfile
import unittest
from pathlib import Path
from unittest import mock


RUTA = Path(__file__).with_name("ejecutar_revision.py")
SPEC = importlib.util.spec_from_file_location("ejecutar_revision", RUTA)
MODULO = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULO)

RUTA_AIRE_LAB = RUTA.parents[1] / "notebooks/aire_lab.py"
SPEC_AIRE_LAB = importlib.util.spec_from_file_location("aire_lab", RUTA_AIRE_LAB)
AIRE_LAB = importlib.util.module_from_spec(SPEC_AIRE_LAB)
SPEC_AIRE_LAB.loader.exec_module(AIRE_LAB)


class EjecutarRevisionTest(unittest.TestCase):
    def test_texto_seguro_csv_neutraliza_formulas(self):
        for peligroso in ("=SUM(A1:A2)", "+1", "-1", "@cmd", "\t=1", "\r=1"):
            self.assertEqual(AIRE_LAB.texto_seguro_csv(peligroso), "'" + peligroso)
        for seguro in ("CA-AYAS-01", "texto", "", 42, None):
            self.assertEqual(AIRE_LAB.texto_seguro_csv(seguro), seguro)

    def test_diferencias_ignora_metadatos_y_ordena_las_claves(self):
        referencia = {"entrada": "linux.csv", "duracion_s": 1, "filas": 10, "estaciones": {"A": 1}}
        actual = {"entrada": "mac.csv", "duracion_s": 2, "filas": 11, "estaciones": {"A": 2}}
        self.assertEqual(MODULO.diferencias_limpieza(actual, referencia), ["estaciones", "filas"])

    def test_escribir_ejecucion_incluye_las_diferencias(self):
        with tempfile.TemporaryDirectory() as temporal:
            base = Path(temporal)
            repo, raiz = base / "repo", base / "revision"
            (repo / "reports").mkdir(parents=True)
            (raiz / "reports").mkdir(parents=True)
            (raiz / "data/silver").mkdir(parents=True)
            (raiz / "data/gold").mkdir(parents=True)
            bronze = base / "bronze.csv"
            bronze.write_text("bronze", encoding="utf-8")
            (raiz / "data/silver/oefa_ancho.csv").write_text("silver", encoding="utf-8")
            (raiz / "data/gold/oefa_features.csv").write_text("gold", encoding="utf-8")
            (raiz / "reports/auditoria.json").write_text(
                json.dumps({"fecha": "2026-11-07", "chequeos": [{"estado": "OK"}]}), encoding="utf-8")
            (raiz / "reports/limpieza.json").write_text(
                json.dumps({"entrada": "mac.csv", "filas": 11, "estaciones": {"A": 1}}), encoding="utf-8")
            (repo / "reports/limpieza.json").write_text(
                json.dumps({"entrada": "linux.csv", "filas": 10, "estaciones": {"A": 1}}), encoding="utf-8")
            fuentes = [
                "configs/limpieza.json", "configs/kmeans.json", "notebooks/aire_lab.py",
                "notebooks/01_eda_criba.ipynb", "notebooks/03_auditoria_figuras.ipynb",
                "scripts/ejecutar_revision.py",
            ]
            for fuente in fuentes:
                ruta = repo / fuente
                ruta.parent.mkdir(parents=True, exist_ok=True)
                ruta.write_text(fuente, encoding="utf-8")

            respuestas = [mock.Mock(stdout="go version go1.27.1 darwin/arm64\n"),
                          mock.Mock(stdout="abc123\n"), mock.Mock(stdout="feature/eda-limpieza\n")]
            with mock.patch.object(MODULO.importlib.metadata, "version", return_value="1.0"), \
                    mock.patch.object(MODULO.subprocess, "run", side_effect=respuestas):
                MODULO.escribir_ejecucion(repo, bronze, raiz)

            evidencia = json.loads((raiz / "reports/ejecucion.json").read_text(encoding="utf-8"))
            self.assertEqual(evidencia["diferencias_con_referencia"], ["filas"])

    def test_candado_impide_dos_ejecuciones_en_la_misma_raiz(self):
        with tempfile.TemporaryDirectory() as temporal:
            raiz = Path(temporal) / "revision"
            with MODULO.ejecucion_exclusiva(raiz):
                with self.assertRaisesRegex(RuntimeError, "Ya hay una revisión"):
                    with MODULO.ejecucion_exclusiva(raiz):
                        pass
            with MODULO.ejecucion_exclusiva(raiz):
                pass


if __name__ == "__main__":
    unittest.main()
