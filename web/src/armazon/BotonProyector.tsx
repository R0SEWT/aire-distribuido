import { useModoProyector } from './modo'

/** Pone y quita el modo proyector. Va en la barra y en el ingreso. */
export function BotonProyector() {
  const [proyector, poner] = useModoProyector()
  return (
    <button
      type="button"
      className="pildora"
      aria-pressed={proyector}
      title="Letra más grande y más contraste, para proyectar"
      onClick={() => poner(!proyector)}
    >
      <svg
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
        className="size-4"
      >
        <rect x="3" y="4" width="18" height="12" rx="2" />
        <path d="M8 20h8M12 16v4" />
      </svg>
      Modo proyector
    </button>
  )
}
