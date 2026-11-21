/** El nombre del sistema, como va en la barra y en el ingreso. */
export function Marca() {
  return (
    <span className="inline-flex items-center gap-2.5 whitespace-nowrap">
      <svg viewBox="0 0 32 32" aria-hidden="true" className="size-7">
        <circle cx="16" cy="16" r="9" fill="none" stroke="var(--color-acento)" strokeWidth="3.5" />
        <circle cx="16" cy="16" r="3.5" fill="var(--color-inca-buena)" />
      </svg>
      <span className="text-texto text-base font-semibold">Aire distribuido</span>
      <span className="text-tenue hidden text-sm sm:inline">Calidad del aire · OEFA</span>
    </span>
  )
}
