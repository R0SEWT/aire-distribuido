<!-- Generado por tablas.py desde web/src/api/cliente.ts. No editar a mano. -->

::: {#lst-cliente-token}
```typescript
const autenticacion: Middleware = {
  onRequest({ request }) {
    const sesion = sesionActual()
    if (sesion) request.headers.set(ENCABEZADO_TOKEN, `Bearer ${sesion.token}`)
    return request
  },
  onResponse({ request, response }) {
    // Un 401 a un pedido que llevaba el token de la sesión actual la cierra
    // (spec §9.5): la UI vuelve al ingreso. Si el token era de una sesión
    // anterior (la respuesta llegó tarde), la de ahora no se toca.
    const enviado = request.headers.get(ENCABEZADO_TOKEN)
    const sesion = sesionActual()
    if (response.status === 401 && sesion && enviado === `Bearer ${sesion.token}`) cerrarSesion('vencida')
    return response
  },
}
```

La interfaz pone el token en cada pedido y cierra la sesión con un 401 (`web/src/api/cliente.ts`, líneas 19–34)
:::
