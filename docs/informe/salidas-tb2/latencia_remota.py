"""Latencia de dos GET contra el despliegue público, desde la laptop, por una conexión ya abierta."""
import http.client, json, statistics, sys, time
host, n = sys.argv[1], int(sys.argv[2])
c = http.client.HTTPSConnection(host, timeout=10)
t0 = time.perf_counter(); c.connect(); conexion = (time.perf_counter() - t0) * 1000
def pedir(metodo, ruta, token=None):
    h = {"Authorization": "Bearer " + token} if token else {}
    t = time.perf_counter(); c.request(metodo, ruta, headers=h); r = c.getresponse(); cuerpo = r.read()
    return (time.perf_counter() - t) * 1000, r.status, cuerpo
_, st, cuerpo = pedir("POST", "/api/auth/invitado"); token = json.loads(cuerpo)["token"]
print(f"conexión TLS abierta en {conexion:.1f} ms; {n} pedidos por ruta sobre esa misma conexión")
print(f"{'ruta':<22}{'p50 ms':>8}{'p95 ms':>8}{'máx ms':>8}  códigos")
for ruta, tok in (("/api/salud", None), ("/api/regimenes", token)):
    for _ in range(3): pedir("GET", ruta, tok)   # calentamiento
    ms, cods = [], set()
    for _ in range(n):
        m, st, _ = pedir("GET", ruta, tok); ms.append(m); cods.add(st)
    ms.sort()
    p95 = ms[int(round(0.95 * len(ms))) - 1]
    print(f"GET {ruta:<18}{statistics.median(ms):>8.1f}{p95:>8.1f}{ms[-1]:>8.1f}  {sorted(cods)}")
