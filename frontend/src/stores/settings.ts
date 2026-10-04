import {computed, ref, watch} from 'vue'
import {defineStore} from 'pinia'

type Theme = 'light' | 'dark' | 'system'
type AppLocale = 'ro' | 'en'

export type ToastIcon = 'info' | 'bell' | 'bell-off' | 'joystick' | 'ghost'

export interface Toast {
  id: number
  title: string
  body?: string
  icon: ToastIcon
  duration: number
}

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

  // No phone is this small and square, so this alone picks out a watch. The simplified
  // layout is only offered there, and only used once the user has said yes to it.
  const watchScreenQuery = window.matchMedia('(max-width: 520px) and (max-height: 520px) and (aspect-ratio: 1/1)')
  const watchScreen = ref(watchScreenQuery.matches)
  watchScreenQuery.addEventListener('change', (e) => {
    watchScreen.value = e.matches
  })
  const savedSimplified = localStorage.getItem('settings.simplifiedLayout')
  const simplifiedChoice = ref<boolean | null>(savedSimplified === 'on' ? true : savedSimplified === 'off' ? false : null)
  const watchActive = computed(() => watchScreen.value && simplifiedChoice.value === true)
  const askSimplified = computed(() => watchScreen.value && simplifiedChoice.value === null)

  watch(watchActive, (active) => {
    document.documentElement.toggleAttribute('data-watch', active)
  }, {immediate: true})

  function setSimplified(on: boolean) {
    simplifiedChoice.value = on
    localStorage.setItem('settings.simplifiedLayout', on ? 'on' : 'off')
  }

  const isDark = computed(() => {
    if (watchActive.value) return true
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

  watch([arcadeActive, watchActive], ([active, onWatch]) => {
    if (active && !onWatch) {
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
    deactivatePaper()
    arcadeActive.value = true
    localStorage.setItem('settings.arcadeActive', 'true')
  }

  function deactivateArcade() {
    arcadeActive.value = false
    localStorage.setItem('settings.arcadeActive', 'false')
  }

  const legacyBlueUnlocked = ref(persistedBool('settings.legacyBlueUnlocked'))
  const legacyBlueActive = ref(persistedBool('settings.legacyBlueActive'))

  watch([legacyBlueActive, watchActive], ([active, onWatch]) => {
    if (active && !onWatch) {
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
    deactivatePaper()
    legacyBlueActive.value = true
    localStorage.setItem('settings.legacyBlueActive', 'true')
  }

  function deactivateLegacyBlue() {
    legacyBlueActive.value = false
    localStorage.setItem('settings.legacyBlueActive', 'false')
  }

  const paperActive = ref(persistedBool('settings.paperActive'))

  watch([paperActive, watchActive], ([active, onWatch]) => {
    if (active && !onWatch) {
      document.documentElement.setAttribute('data-paper', '')
    } else {
      document.documentElement.removeAttribute('data-paper')
    }
  }, {immediate: true})

  function activatePaper() {
    deactivateArcade()
    deactivateLegacyBlue()
    paperActive.value = true
    localStorage.setItem('settings.paperActive', 'true')
  }

  function deactivatePaper() {
    paperActive.value = false
    localStorage.setItem('settings.paperActive', 'false')
  }

  const showWeather = ref(localStorage.getItem('settings.showWeather') !== 'false')
  const showNews = ref(localStorage.getItem('settings.showNews') !== 'false')
  const autoCenterOnMe = ref(localStorage.getItem('settings.autoCenterOnMe') !== 'false')
  const autoFitMap = ref(localStorage.getItem('settings.autoFitMap') !== 'false')
  const showVehicleExtras = ref(localStorage.getItem('settings.showVehicleExtras') !== 'false')
  const showGreenFriday = ref(localStorage.getItem('settings.showGreenFriday') !== 'false')
  const showTimetableChanges = ref(persistedBool('settings.showTimetableChanges'))

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

  function setShowVehicleExtras(val: boolean) {
    showVehicleExtras.value = val
    localStorage.setItem('settings.showVehicleExtras', val ? 'true' : 'false')
  }

  function setShowGreenFriday(val: boolean) {
    showGreenFriday.value = val
    localStorage.setItem('settings.showGreenFriday', val ? 'true' : 'false')
  }

  function setShowTimetableChanges(val: boolean) {
    showTimetableChanges.value = val
    localStorage.setItem('settings.showTimetableChanges', val ? 'true' : 'false')
  }

  const toast = ref<Toast | null>(null)
  let toastTimer: ReturnType<typeof setTimeout> | null = null
  let toastSeq = 0

  function showToast(title: string, options: { body?: string; icon?: ToastIcon } = {}) {
    const duration = options.body ? 4500 : 3000
    toast.value = {id: ++toastSeq, title, body: options.body, icon: options.icon ?? 'info', duration}
    if (toastTimer) clearTimeout(toastTimer)
    toastTimer = setTimeout(dismissToast, duration)
  }

  function dismissToast() {
    if (toastTimer) clearTimeout(toastTimer)
    toastTimer = null
    toast.value = null
  }

  return {
    theme, locale, isDark, setTheme, setLocale,
    watchScreen, watchActive, askSimplified, setSimplified,
    arcadeUnlocked, arcadeActive,
    unlockArcade, activateArcade, deactivateArcade,
    legacyBlueUnlocked, legacyBlueActive,
    unlockLegacyBlue, activateLegacyBlue, deactivateLegacyBlue,
    paperActive, activatePaper, deactivatePaper,
    showWeather, showNews, setShowWeather, setShowNews,
    autoCenterOnMe, autoFitMap, setAutoCenterOnMe, setAutoFitMap,
    showVehicleExtras, setShowVehicleExtras,
    showGreenFriday, setShowGreenFriday,
    showTimetableChanges, setShowTimetableChanges,
    toast, showToast, dismissToast,
  }
})
