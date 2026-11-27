import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'
import { instalarDialogoModal } from './dialogo'

// Las pruebas que corren en Node (pruebas/caddy.test.ts) no tienen navegador.
if (typeof document !== 'undefined') instalarDialogoModal()

afterEach(() => {
  // Las pruebas que corren en Node (pruebas/caddy.test.ts) no tienen navegador.
  if (typeof document === 'undefined') return
  cleanup()
  sessionStorage.clear()
  localStorage.clear()
  delete document.documentElement.dataset.modo
})
