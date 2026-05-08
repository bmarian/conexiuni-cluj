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

  function detectInitialLocale(): AppLocale {
    const saved = localStorage.getItem('settings.locale') as AppLocale | null
    if (saved === 'ro' || saved === 'en') return saved
    return navigator.language.toLowerCase().startsWith('en') ? 'en' : 'ro'
  }

  const locale = ref<AppLocale>(detectInitialLocale())

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

  watch(locale, (lang) => {
    document.documentElement.lang = lang
  }, {immediate: true})

  function setLocale(newLocale: AppLocale) {
    locale.value = newLocale
    localStorage.setItem('settings.locale', newLocale)
  }

  function persistedBool(key: string) {
    return localStorage.getItem(key) === 'true'
  }

  const arcadeUnlocked = ref(persistedBool('settings.arcadeUnlocked'))
  const arcadeActive = ref(persistedBool('settings.arcadeActive'))

  watch(arcadeActive, (active) => {
    if (active) {
      document.documentElement.setAttribute('data-arcade', '')
    } else {
      document.documentElement.removeAttribute('data-arcade')
    }
  }, {immediate: true})

  function unlockArcade() {
    arcadeUnlocked.value = true
    localStorage.setItem('settings.arcadeUnlocked', 'true')
  }

  function activateArcade() {
    deactivateLegacyBlue()
    arcadeActive.value = true
    localStorage.setItem('settings.arcadeActive', 'true')
  }

  function deactivateArcade() {
    arcadeActive.value = false
    localStorage.setItem('settings.arcadeActive', 'false')
  }

  const legacyBlueUnlocked = ref(persistedBool('settings.legacyBlueUnlocked'))
  const legacyBlueActive = ref(persistedBool('settings.legacyBlueActive'))

  watch(legacyBlueActive, (active) => {
    if (active) {
      document.documentElement.setAttribute('data-legacy-blue', '')
    } else {
      document.documentElement.removeAttribute('data-legacy-blue')
    }
  }, {immediate: true})

  function unlockLegacyBlue() {
    if (legacyBlueUnlocked.value) return
    legacyBlueUnlocked.value = true
    localStorage.setItem('settings.legacyBlueUnlocked', 'true')
  }

  function activateLegacyBlue() {
    deactivateArcade()
    legacyBlueActive.value = true
    localStorage.setItem('settings.legacyBlueActive', 'true')
  }

  function deactivateLegacyBlue() {
    legacyBlueActive.value = false
    localStorage.setItem('settings.legacyBlueActive', 'false')
  }

  const showWeather = ref(localStorage.getItem('settings.showWeather') !== 'false')
  const showNews = ref(localStorage.getItem('settings.showNews') !== 'false')
  const autoCenterOnMe = ref(localStorage.getItem('settings.autoCenterOnMe') !== 'false')
  const autoFitMap = ref(localStorage.getItem('settings.autoFitMap') !== 'false')

  function setShowWeather(val: boolean) {
    showWeather.value = val
    localStorage.setItem('settings.showWeather', val ? 'true' : 'false')
  }

  function setShowNews(val: boolean) {
    showNews.value = val
    localStorage.setItem('settings.showNews', val ? 'true' : 'false')
  }

  function setAutoCenterOnMe(val: boolean) {
    autoCenterOnMe.value = val
    localStorage.setItem('settings.autoCenterOnMe', val ? 'true' : 'false')
  }

  function setAutoFitMap(val: boolean) {
    autoFitMap.value = val
    localStorage.setItem('settings.autoFitMap', val ? 'true' : 'false')
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
    arcadeUnlocked, arcadeActive,
    unlockArcade, activateArcade, deactivateArcade,
    legacyBlueUnlocked, legacyBlueActive,
    unlockLegacyBlue, activateLegacyBlue, deactivateLegacyBlue,
    showWeather, showNews, setShowWeather, setShowNews,
    autoCenterOnMe, autoFitMap, setAutoCenterOnMe, setAutoFitMap,
    toastMessage, showToast,
  }
})
