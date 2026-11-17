# /// script
# requires-python = ">=3.11"
# dependencies = ["openapi-spec-validator>=0.7", "openapi-schema-validator>=0.6", "pyyaml>=6"]
# ///
"""Valida OpenAPI, la cobertura de la PC4 y sus ejemplos sin levantar la API.

    uv run scripts/validar-openapi.py
"""

import argparse
import copy
from pathlib import Path

import yaml
from openapi_schema_validator import OAS30Validator
from openapi_spec_validator import validate


RAIZ = Path(__file__).resolve().parents[1]
OPERACIONES = {
    "/api/auth/token": {"post"},
    "/api/salud": {"get"},
    "/api/predicciones": {"post"},
    "/api/estaciones": {"get"},
    "/api/estaciones/{id}/regimen": {"get"},
    "/api/modelos/activo": {"get"},
    "/api/cluster/nodos": {"get"},
    "/api/entrenamientos": {"get", "post"},
    "/api/entrenamientos/{id}": {"get"},
    "/api/metricas": {"get"},
    "/openapi.yaml": {"get"},
    "/docs": {"get"},
    "/ws": {"get"},
}
PUBLICAS = {"/api/auth/token", "/api/salud", "/openapi.yaml", "/docs"}
ADMIN = {"/api/cluster/nodos", "/api/entrenamientos", "/api/entrenamientos/{id}", "/api/metricas"}


class SinDuplicados(yaml.SafeLoader):
    """YAML no debe ocultar un endpoint o un campo sobrescrito por una clave repetida."""


def mapping_unico(loader, node, deep=False):
    resultado = {}
    for clave_node, valor_node in node.value:
        clave = loader.construct_object(clave_node, deep=deep)
        if clave in resultado:
            raise ValueError(f"clave YAML duplicada: {clave}, línea {clave_node.start_mark.line + 1}")
        resultado[clave] = loader.construct_object(valor_node, deep=deep)
    return resultado


SinDuplicados.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, mapping_unico)


def resolver(objeto, documento, pila=()):
    """Expande referencias locales para validar también ejemplos fuera del esquema OpenAPI."""
    if isinstance(objeto, list):
        return [resolver(v, documento, pila) for v in objeto]
    if not isinstance(objeto, dict):
        return objeto
    if "$ref" in objeto:
        ref = objeto["$ref"]
        if not ref.startswith("#/") or ref in pila:
            raise ValueError(f"referencia no local o cíclica: {ref}")
        valor = documento
        for parte in ref[2:].split("/"):
            valor = valor[parte.replace("~1", "/").replace("~0", "~")]
        return resolver(valor, documento, (*pila, ref))
    return {k: resolver(v, documento, pila) for k, v in objeto.items()}


def ejemplos_media(media):
    if "example" in media:
        yield media["example"]
    for ejemplo in media.get("examples", {}).values():
        yield ejemplo["value"]


def comprobar(documento):
    validate(documento)
    paths = documento["paths"]
    if set(paths) != set(OPERACIONES):
        raise ValueError("las rutas no coinciden con la spec de la PC4 §6.1")
    total_ejemplos = 0
    for ruta, metodos in OPERACIONES.items():
        item = paths[ruta]
        actuales = set(item) & {"get", "post", "put", "delete", "patch", "options", "head", "trace"}
        if actuales != metodos:
            raise ValueError(f"métodos distintos a la spec en {ruta}")
        for metodo in sorted(metodos):
            operacion = item[metodo]
            if ruta in PUBLICAS:
                if operacion.get("security") != []:
                    raise ValueError(f"{ruta} debe ser público")
            else:
                seguridad = operacion.get("security", documento.get("security"))
                esperada = [{"wsToken": []}] if ruta == "/ws" else [{"bearerAuth": []}]
                if seguridad != esperada:
                    raise ValueError(f"autenticación incorrecta en {ruta}")
                roles = ["admin"] if ruta in ADMIN else ["usuario", "admin"]
                if operacion.get("x-roles") != roles:
                    raise ValueError(f"roles incorrectos en {ruta}")
            contenidos = []
            if "requestBody" in operacion:
                contenidos.append(operacion["requestBody"]["content"])
            contenidos.extend(resolver(r, documento).get("content", {}) for r in operacion["responses"].values())
            for contenido in contenidos:
                for media in contenido.values():
                    schema = resolver(media["schema"], documento)
                    validador = OAS30Validator(schema, format_checker=OAS30Validator.FORMAT_CHECKER)
                    for ejemplo in ejemplos_media(media):
                        try:
                            validador.validate(ejemplo)
                        except Exception as error:
                            raise ValueError(f"ejemplo inválido en {metodo.upper()} {ruta}: {error}") from error
                        total_ejemplos += 1
    eventos = paths["/ws"]["get"]["x-websocket-events"]
    if set(eventos) != {"entrenamiento.ronda", "entrenamiento.fin"}:
        raise ValueError("los eventos no coinciden con la spec §6.4")
    for evento in eventos.values():
        schema = resolver(evento["schema"], documento)
        ejemplos = [evento["example"]] if "example" in evento else list(evento["examples"].values())
        for ejemplo in ejemplos:
            OAS30Validator(schema).validate(ejemplo)
            total_ejemplos += 1
    return total_ejemplos


def comprobar_rechazos(documento):
    """Comprueba que el contrato rechaza lecturas y pedidos físicamente inválidos."""
    lectura = documento["paths"]["/api/predicciones"]["post"]["requestBody"]["content"]["application/json"]
    schema = resolver(lectura["schema"], documento)
    validador = OAS30Validator(schema, format_checker=OAS30Validator.FORMAT_CHECKER)
    for campo, valor in (("pm2_5", -1), ("pm10", -1), ("fecha_hora", "ayer"), ("estacion", "")):
        invalido = copy.deepcopy(lectura["example"])
        invalido[campo] = valor
        if validador.is_valid(invalido):
            raise ValueError(f"el esquema no rechaza {campo}={valor!r}")
    pedido = documento["components"]["schemas"]["PedidoEntrenamiento"]
    for campo in ("nodos", "replicas"):
        invalido = {"nodos": 4, "replicas": 1, "semilla": 42, campo: 0}
        if OAS30Validator(pedido).is_valid(invalido):
            raise ValueError(f"el esquema no rechaza {campo}=0")


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("archivo", nargs="?", type=Path, default=RAIZ / "docs/api/openapi.yaml")
    args = parser.parse_args()
    documento = yaml.load(args.archivo.read_text(encoding="utf-8"), Loader=SinDuplicados)
    ejemplos = comprobar(documento)
    comprobar_rechazos(documento)
    print(f"OpenAPI válido: {len(OPERACIONES)} rutas, {sum(map(len, OPERACIONES.values()))} operaciones, "
          f"{ejemplos} ejemplos y 6 rechazos comprobados")


if __name__ == "__main__":
    main()
