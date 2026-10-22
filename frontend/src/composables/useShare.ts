import {useI18n} from 'vue-i18n'
import {useSettingsStore} from '@/stores/settings.ts'
import {useKbdShortcuts} from '@/composables/useKeyboardNav.ts'

export function useShare() {
  const {t} = useI18n()
  const settings = useSettingsStore()

  async function share() {
    const url = window.location.href
    if (navigator.share) {
      try {
        await navigator.share({url})
      } catch {
        /* noop */
      }
      return
    }
    try {
      await navigator.clipboard.writeText(url)
      settings.showToast(t('urlCopied'))
    } catch {
      /* noop */
    }
  }

  useKbdShortcuts({c: () => void share()})

  return {share}
}
