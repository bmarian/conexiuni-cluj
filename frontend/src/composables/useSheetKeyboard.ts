import {nextTick, onMounted, onUnmounted, watch, type Ref} from 'vue'
import {isTypingTarget} from '@/utils/keyboardFocus.ts'
import {isIOS} from '@/utils/ios.ts'

export function useSheetKeyboard(
  sheet: Ref<HTMLElement | null>,
  options: {
    enabled: Ref<boolean>
    keyboardPx: Ref<number>
    measureViewport: boolean
    open: () => void
  },
) {
  const {enabled, keyboardPx, measureViewport, open} = options
  if (isIOS) return

  const typingInSheet = () => isTypingTarget(document.activeElement) && !!sheet.value?.contains(document.activeElement)
  let tapped = false
  const onPointerDown = (e: PointerEvent) => {
    tapped = e.pointerType === 'touch'
  }
  const onKeyDown = () => {
    tapped = false
  }

  function onFocusIn() {
    if (tapped && enabled.value && typingInSheet()) open()
    if (measureViewport) sync()
  }

  function sync() {
    const vv = window.visualViewport
    if (!vv || !enabled.value || !typingInSheet()) {
      keyboardPx.value = 0
      return
    }
    // Undo the browser panning to the field.
    if (vv.offsetTop > 0) document.documentElement.scrollIntoView({block: 'start'})
    keyboardPx.value = Math.max(0, Math.round(window.innerHeight - vv.height))
  }

  // scrollIntoView would also scroll the page behind.
  function reveal() {
    const field = document.activeElement
    const list = field?.closest<HTMLElement>('.drawer-view')
    if (!(field instanceof HTMLElement) || !list) return
    const box = field.getBoundingClientRect()
    const view = list.getBoundingClientRect()
    if (box.bottom > view.bottom) list.scrollTop += box.bottom - view.bottom
    else if (box.top < view.top) list.scrollTop -= view.top - box.top
  }

  watch(keyboardPx, (px) => {
    if (px <= 0 || !enabled.value || !typingInSheet()) return
    open()
    void nextTick(reveal)
  })

  onMounted(() => {
    window.addEventListener('pointerdown', onPointerDown, {capture: true, passive: true})
    window.addEventListener('keydown', onKeyDown, {capture: true, passive: true})
    sheet.value?.addEventListener('focusin', onFocusIn)
    if (!measureViewport) return
    window.visualViewport?.addEventListener('resize', sync, {passive: true})
    window.visualViewport?.addEventListener('scroll', sync, {passive: true})
    sheet.value?.addEventListener('focusout', sync)
  })

  onUnmounted(() => {
    window.removeEventListener('pointerdown', onPointerDown, {capture: true})
    window.removeEventListener('keydown', onKeyDown, {capture: true})
    sheet.value?.removeEventListener('focusin', onFocusIn)
    window.visualViewport?.removeEventListener('resize', sync)
    window.visualViewport?.removeEventListener('scroll', sync)
    sheet.value?.removeEventListener('focusout', sync)
  })
}
