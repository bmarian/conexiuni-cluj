import {computed, ref, watch} from 'vue'
import {defineStore} from 'pinia'
import {currentCity} from '@/utils/city.ts'
import {REGION_SETTINGS, regionSettingStorageKey, type RegionSettingKey} from '@/utils/regionSettings.ts'

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
    deactivatePaper()
    legacyBlueActive.value = true
    localStorage.setItem('settings.legacyBlueActive', 'true')
  }

  function deactivateLegacyBlue() {
    legacyBlueActive.value = false
    localStorage.setItem('settings.legacyBlueActive', 'false')
  }

  const paperActive = ref(persistedBool('settings.paperActive'))

  watch(paperActive, (active) => {
    if (active) {
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

  const autoCenterOnMe = ref(localStorage.getItem('settings.autoCenterOnMe') !== 'false')
  const autoFitMap = ref(localStorage.getItem('settings.autoFitMap') !== 'false')
  const showVehicleExtras = ref(localStorage.getItem('settings.showVehicleExtras') !== 'false')

  const regionValues = ref<Record<string, boolean>>({})
  function regionSetting(slug: string, key: RegionSettingKey): boolean {
    const setting = REGION_SETTINGS.find((s) => s.key === key)
    if (!setting || !setting.cities.includes(slug)) return false
    const storageKey = regionSettingStorageKey(slug, key)
    if (!(storageKey in regionValues.value)) {
      const saved = localStorage.getItem(storageKey)
      regionValues.value[storageKey] = saved === null ? setting.defaultOn : saved === 'true'
    }
    return regionValues.value[storageKey]!
  }
  function setRegionSetting(slug: string, key: RegionSettingKey, val: boolean) {
    const storageKey = regionSettingStorageKey(slug, key)
    regionValues.value[storageKey] = val
    localStorage.setItem(storageKey, val ? 'true' : 'false')
  }
  const showGreenFriday = computed(() => regionSetting(currentCity.value, 'showGreenFriday'))

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

  const busAlerts = ref(persistedBool('settings.busAlerts'))

  function setBusAlerts(val: boolean) {
    busAlerts.value = val
    localStorage.setItem('settings.busAlerts', val ? 'true' : 'false')
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
    arcadeUnlocked, arcadeActive,
    unlockArcade, activateArcade, deactivateArcade,
    legacyBlueUnlocked, legacyBlueActive,
    unlockLegacyBlue, activateLegacyBlue, deactivateLegacyBlue,
    paperActive, activatePaper, deactivatePaper,
    autoCenterOnMe, autoFitMap, setAutoCenterOnMe, setAutoFitMap,
    showVehicleExtras, setShowVehicleExtras,
    showGreenFriday, regionSetting, setRegionSetting,
    busAlerts, setBusAlerts,
    toast, showToast, dismissToast,
  }
})
