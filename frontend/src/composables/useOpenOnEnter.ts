import {watch, type Ref} from 'vue'
import {useKeyboardNav} from '@/composables/useKeyboardNav.ts'

export function useOpenOnEnter(openTop: () => boolean, loading: Ref<boolean>, query: Ref<string>, blur: () => void) {
  const {keyboardMode} = useKeyboardNav()
  let waiting = false

  function onEnter() {
    if (!keyboardMode.value && openTop()) return
    blur()
    waiting = !keyboardMode.value && loading.value
  }

  watch(query, () => (waiting = false))
  watch(loading, (isLoading) => {
    if (isLoading || !waiting) return
    waiting = false
    openTop()
  })

  return onEnter
}
