import {nextTick, onMounted, onUnmounted, watch, type Ref} from 'vue'
import {isTypingTarget} from '@/utils/keyboardFocus.ts'
import {isIOS} from '@/utils/ios.ts'

// Android covers the page with the keyboard instead of resizing it. As on the phone, typing in the sheet opens it fully
// and its list ends above the keyboard. iOS has its own handling in useIOSKeyboard.
export function useSheetKeyboard(sheet: Ref<HTMLElement | null>, options: {
  enabled: Ref<boolean>
  keyboardPx: Ref<number>
  // Browser tabs only shrink the visual viewport; an installed app's keyboard is measured by the VirtualKeyboard API.
  measure: boolean
  open: () => void
}) {
  const {enabled, keyboardPx, measure, open} = options
  if (isIOS) return

  const typingInSheet = () => isTypingTarget(document.activeElement) && !!sheet.value?.contains(document.activeElement)

  function sync() {
    const vv = window.visualViewport
    keyboardPx.value = vv && enabled.value && typingInSheet() ? Math.max(0, Math.round(window.innerHeight - vv.height - vv.offsetTop)) : 0
  }

  // Scrolls only the sheet's list: scrollIntoView would also scroll the clipped page behind it.
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
    if (!measure) return
    window.visualViewport?.addEventListener('resize', sync, {passive: true})
    window.visualViewport?.addEventListener('scroll', sync, {passive: true})
    sheet.value?.addEventListener('focusin', sync)
    sheet.value?.addEventListener('focusout', sync)
  })

  onUnmounted(() => {
    window.visualViewport?.removeEventListener('resize', sync)
    window.visualViewport?.removeEventListener('scroll', sync)
    sheet.value?.removeEventListener('focusin', sync)
    sheet.value?.removeEventListener('focusout', sync)
  })
}
