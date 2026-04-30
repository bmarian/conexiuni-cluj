import {computed, ref, watch} from 'vue'
import {defineStore} from 'pinia'

type Theme = 'light' | 'dark' | 'system'
type AppLocale = 'ro' | 'en'

export const useSettingsStore = defineStore('settings', () => {
  const systemDark = ref(window.matchMedia('(prefers-color-scheme: dark)').matches)
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
    systemDark.value = e.matches
  })

  const theme = ref<Theme>((localStorage.getItem('settings.theme') as Theme) ?? 'system')
  const locale = ref<AppLocale>((localStorage.getItem('settings.locale') as AppLocale) ?? 'ro')

  const isDark = computed(() => {
    if (theme.value === 'dark') return true
    if (theme.value === 'light') return false
    return systemDark.value
  })

  watch(isDark, (dark) => {
    document.documentElement.classList.toggle('dark', dark)
    document.documentElement.style.colorScheme = dark ? 'dark' : 'light'
  }, {immediate: true})

  function setTheme(newTheme: Theme) {
    theme.value = newTheme
    localStorage.setItem('settings.theme', newTheme)
  }

  function setLocale(newLocale: AppLocale) {
    locale.value = newLocale
    localStorage.setItem('settings.locale', newLocale)
  }

  const easterEggUnlocked = ref(localStorage.getItem('settings.easterEggUnlocked') === 'true')
  const easterEggActive = ref(localStorage.getItem('settings.easterEggActive') === 'true')

  watch(easterEggActive, (active) => {
    if (active) {
      document.documentElement.setAttribute('data-hungry', '')
    } else {
      document.documentElement.removeAttribute('data-hungry')
    }
  }, {immediate: true})

  function unlockEasterEgg() {
    easterEggUnlocked.value = true
    localStorage.setItem('settings.easterEggUnlocked', 'true')
  }

  function activateEasterEgg() {
    deactivateTraditional()
    easterEggActive.value = true
    localStorage.setItem('settings.easterEggActive', 'true')
  }

  function deactivateEasterEgg() {
    easterEggActive.value = false
    localStorage.setItem('settings.easterEggActive', 'false')
  }

  const traditionalUnlocked = ref(localStorage.getItem('settings.traditionalUnlocked') === 'true')
  const traditionalActive = ref(localStorage.getItem('settings.traditionalActive') === 'true')
  const traditionalLowPerf = ref(localStorage.getItem('settings.traditionalLowPerf') === 'true')

  watch(traditionalActive, (active) => {
    if (active) {
      document.documentElement.setAttribute('data-traditional', '')
      if (traditionalLowPerf.value) {
        document.documentElement.setAttribute('data-traditional-lowperf', '')
      }
    } else {
      document.documentElement.removeAttribute('data-traditional')
      document.documentElement.removeAttribute('data-traditional-lowperf')
    }
  }, {immediate: true})

  watch(traditionalLowPerf, (lowPerf) => {
    if (lowPerf) {
      document.documentElement.setAttribute('data-traditional-lowperf', '')
    } else {
      document.documentElement.removeAttribute('data-traditional-lowperf')
    }
  }, {immediate: true})

  function unlockTraditional() {
    if (traditionalUnlocked.value) return
    traditionalUnlocked.value = true
    localStorage.setItem('settings.traditionalUnlocked', 'true')
  }

  function activateTraditional() {
    deactivateEasterEgg()
    traditionalActive.value = true
    localStorage.setItem('settings.traditionalActive', 'true')
  }

  function deactivateTraditional() {
    traditionalActive.value = false
    localStorage.setItem('settings.traditionalActive', 'false')
  }

  function setTraditionalLowPerf(val: boolean) {
    traditionalLowPerf.value = val
    localStorage.setItem('settings.traditionalLowPerf', val ? 'true' : 'false')
  }

  const toastMessage = ref<string | null>(null)
  let toastTimer: ReturnType<typeof setTimeout> | null = null

  function showToast(message: string) {
    toastMessage.value = message
    if (toastTimer) clearTimeout(toastTimer)
    toastTimer = setTimeout(() => {
      toastMessage.value = null
    }, 3000)
  }

  return {
    theme, locale, isDark, setTheme, setLocale,
    easterEggUnlocked, easterEggActive,
    unlockEasterEgg, activateEasterEgg, deactivateEasterEgg,
    traditionalUnlocked, traditionalActive, traditionalLowPerf,
    unlockTraditional, activateTraditional, deactivateTraditional, setTraditionalLowPerf,
    toastMessage, showToast,
  }
})
