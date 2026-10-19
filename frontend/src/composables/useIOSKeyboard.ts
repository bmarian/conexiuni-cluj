import {onMounted, onUnmounted, ref, type Ref} from 'vue'
import {isTypingTarget} from '@/utils/keyboardFocus.ts'
import {isIOS} from '@/utils/ios.ts'

const TOUCH_TO_FOCUS_MS = 1000

function opensKeyboard(el: EventTarget | null): boolean {
  return el instanceof HTMLElement && isTypingTarget(el) && !(el as HTMLInputElement).readOnly
}

// iOS scrolls the whole page to reveal a focused field and picks that scroll at the moment of focus,
// so the sheet is lifted before it measures and the slide is replayed a frame later.
export function useIOSKeyboard(sheet: Ref<HTMLElement | null>, options: {
  enabled: Ref<boolean>
  sheetHeight: Ref<number>
  liftTo: Ref<number>
  keyboardPx: Ref<number>
}) {
  const {enabled, sheetHeight, liftTo, keyboardPx} = options
  const fieldFocused = ref(false)
  let lastTouch = -Infinity

  function onPointerDown() {
    lastTouch = performance.now()
  }

  function onFocusIn(e: FocusEvent) {
    if (!enabled.value || !opensKeyboard(e.target) || performance.now() - lastTouch > TOUCH_TO_FOCUS_MS) return
    fieldFocused.value = true
    const el = sheet.value
    if (!el) return
    const from = el.style.transform
    const to = `translateY(${liftTo.value}px)`
    el.style.transition = 'none'
    el.style.transform = to
    void el.offsetHeight
    requestAnimationFrame(() => {
      el.style.transform = from
      void el.offsetHeight
      el.style.transition = ''
      el.style.transform = to
    })
  }

  function onFocusOut(e: FocusEvent) {
    if (!opensKeyboard(e.relatedTarget)) fieldFocused.value = false
  }

  function sync() {
    const vv = window.visualViewport
    if (!vv) return
    if (!enabled.value || !sheet.value?.contains(document.activeElement)) {
      keyboardPx.value = 0
      return
    }
    if (window.scrollY !== 0) window.scrollTo(0, 0)
    keyboardPx.value = Math.max(0, Math.round(sheetHeight.value - vv.height - vv.offsetTop))
  }

  onMounted(() => {
    const el = sheet.value
    if (!isIOS || !el) return
    el.addEventListener('pointerdown', onPointerDown, {capture: true, passive: true})
    el.addEventListener('focusin', onFocusIn)
    el.addEventListener('focusout', onFocusOut)
    window.visualViewport?.addEventListener('resize', sync, {passive: true})
    window.visualViewport?.addEventListener('scroll', sync, {passive: true})
  })

  onUnmounted(() => {
    const el = sheet.value
    el?.removeEventListener('pointerdown', onPointerDown, {capture: true})
    el?.removeEventListener('focusin', onFocusIn)
    el?.removeEventListener('focusout', onFocusOut)
    window.visualViewport?.removeEventListener('resize', sync)
    window.visualViewport?.removeEventListener('scroll', sync)
  })

  return {fieldFocused}
}
