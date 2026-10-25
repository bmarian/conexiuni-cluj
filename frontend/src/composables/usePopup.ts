import {onBeforeUnmount, ref} from 'vue'

export function usePopup(inside: () => (HTMLElement | null | undefined)[]) {
  const open = ref(false)

  function onOutside(e: Event) {
    if (!inside().some((el) => el?.contains(e.target as Node))) close()
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') close()
  }

  function close() {
    open.value = false
    document.removeEventListener('pointerdown', onOutside, true)
    document.removeEventListener('keydown', onKey)
  }

  function show() {
    open.value = true
    document.addEventListener('pointerdown', onOutside, true)
    document.addEventListener('keydown', onKey)
  }

  onBeforeUnmount(close)
  return {open, show, close}
}
