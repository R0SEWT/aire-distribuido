/// <reference types="vitest/config" />
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// La API local, para `pnpm dev`. En Docker Compose no hay proxy de Vite: Caddy
// sirve la UI y pasa estas mismas rutas a la API (web/Caddyfile).
const API = process.env.API_URL ?? 'http://localhost:8080'

// Sin `changeOrigin`: la API acepta el WebSocket si el Origin es el mismo host
// que el pedido (api/cors.go). Con changeOrigin el proxy reescribe Host, dejan
// de coincidir y el apretón de manos responde 403.
const proxy = {
  '/api': { target: API },
  '/ws': { target: API, ws: true },
  '/docs': { target: API },
  '/openapi.yaml': { target: API },
}

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: { proxy },
  preview: { proxy },
  build: {
    // La CSP no admite scripts ni estilos en línea: nada de recursos embebidos
    // como data: en el CSS (las fuentes salen como archivos del propio origen).
    assetsInlineLimit: 0,
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/pruebas/preparar.ts'],
    css: false,
    restoreMocks: true,
  },
})
