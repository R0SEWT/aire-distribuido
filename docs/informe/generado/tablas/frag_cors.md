<!-- Generado por tablas.py desde api/cors.go. No editar a mano. -->

::: {#lst-cors}
```go
origen := r.Header.Get("Origin")
preflight := r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
if origen == "" {
    return false
}
// La respuesta depende de Origin: una caché no puede servirle a un origen
// la respuesta de otro.
w.Header().Add("Vary", "Origin")
if !c.permite(origen) {
    if preflight {
        escribirError(w, http.StatusForbidden, "CORS: el origen "+recortar(origen, 128)+" no está permitido")
        return true
    }
    return false // sin encabezados: el servidor responde y el navegador no deja leer
}
if c.todos {
    w.Header().Set("Access-Control-Allow-Origin", "*")
} else {
    w.Header().Set("Access-Control-Allow-Origin", origen)
}
if !preflight {
    return false
}
w.Header().Add("Vary", "Access-Control-Request-Method")
w.Header().Add("Vary", "Access-Control-Request-Headers")
w.Header().Set("Access-Control-Allow-Methods", corsMetodos)
w.Header().Set("Access-Control-Allow-Headers", corsEncabezados)
w.Header().Set("Access-Control-Max-Age", corsMaxAge)
w.WriteHeader(http.StatusNoContent)
```

CORS en la API: un origen que no está en la lista no recibe encabezados, y su preflight, un 403 (`api/cors.go`, líneas 78–106)
:::
