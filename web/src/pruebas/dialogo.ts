// jsdom no implementa showModal() ni close() de <dialog>. Este doble pone lo
// que la app usa del diálogo modal del navegador:
//
// - showModal() lo abre (el atributo `open`) y recuerda quién tenía el foco;
// - Escape lo cancela: el evento `cancel`, que se puede impedir, y si no, se cierra;
// - close() lo cierra, devuelve el foco a quien lo tenía y avisa con `close`,
//   después y no en el acto, como el navegador.
//
// No pone lo demás: la capa superior, que lo de atrás quede inerte y que el
// foco no salga del diálogo son del navegador y se comprueban en uno de verdad.

interface Abierto {
  conFoco: Element | null
  alTeclear: (e: KeyboardEvent) => void
}

const abiertos = new WeakMap<HTMLDialogElement, Abierto>()

function soltar(dialogo: HTMLDialogElement): Abierto | undefined {
  const abierto = abiertos.get(dialogo)
  if (abierto) document.removeEventListener('keydown', abierto.alTeclear)
  abiertos.delete(dialogo)
  return abierto
}

/** Pone showModal() y close() en HTMLDialogElement si el entorno no los trae. */
export function instalarDialogoModal() {
  const prototipo = HTMLDialogElement.prototype
  if (typeof prototipo.showModal === 'function') return

  prototipo.showModal = function (this: HTMLDialogElement) {
    if (this.open) return
    const alTeclear = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      // Se desmontó sin cerrarse: ya no hay diálogo que cancelar.
      if (!this.isConnected || !this.open) {
        soltar(this)
        return
      }
      if (this.dispatchEvent(new Event('cancel', { cancelable: true }))) this.close()
    }
    abiertos.set(this, { conFoco: document.activeElement, alTeclear })
    document.addEventListener('keydown', alTeclear)
    this.setAttribute('open', '')
  }

  prototipo.close = function (this: HTMLDialogElement) {
    if (!this.open) return
    this.removeAttribute('open')
    const abierto = soltar(this)
    if (abierto?.conFoco instanceof HTMLElement && abierto.conFoco.isConnected) abierto.conFoco.focus()
    queueMicrotask(() => this.dispatchEvent(new Event('close')))
  }
}
