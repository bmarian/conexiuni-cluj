import {computed, ref, watch} from 'vue'
import {defineStore} from 'pinia'
import {currentCity} from '@/utils/city.ts'
import {urlLocale} from '@/utils/site.ts'
import {REGION_SETTINGS, regionSettingStorageKey, type RegionSettingKey} from '@/utils/regionSettings.ts'

type Theme = 'light' | 'dark' | 'system'
type AppLocale = 'ro' | 'en'

export type ToastIcon = 'info' | 'bell' | 'bell-off'

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
    const asked = urlLocale()
    if (asked) return asked
    const saved = localStorage.getItem('settings.locale') as AppLocale | null
    // Not navigator.language: bare URLs are the Romanian canonical, and Google renders them as en-US.
    return saved === 'en' ? 'en' : 'ro'
  }

  const locale = ref<AppLocale>(detectInitialLocale())

  const isDark = computed(() => {
    if (theme.value === 'dark') return true
    if (theme.value === 'light') return false
    return systemDark.value
  })

  watch(
    isDark,
    (dark) => {
      document.documentElement.classList.toggle('dark', dark)
      document.documentElement.style.colorScheme = dark ? 'dark' : 'light'
    },
    {immediate: true},
  )

  function setTheme(newTheme: Theme) {
    theme.value = newTheme
    localStorage.setItem('settings.theme', newTheme)
  }

  watch(
    locale,
    (lang) => {
      document.documentElement.lang = lang
    },
    {immediate: true},
  )

  function setLocale(newLocale: AppLocale) {
    locale.value = newLocale
    localStorage.setItem('settings.locale', newLocale)
  }

  const autoCenterOnMe = ref(localStorage.getItem('settings.autoCenterOnMe') !== 'false')
  const autoFitMap = ref(localStorage.getItem('settings.autoFitMap') !== 'false')
  const colorfulLines = ref(localStorage.getItem('settings.colorfulLines') === 'true')

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

  function setColorfulLines(val: boolean) {
    colorfulLines.value = val
    localStorage.setItem('settings.colorfulLines', val ? 'true' : 'false')
  }

  // Another tab changed what is saved.
  function reload() {
    theme.value = (localStorage.getItem('settings.theme') as Theme) ?? 'system'
    locale.value = detectInitialLocale()
    autoCenterOnMe.value = localStorage.getItem('settings.autoCenterOnMe') !== 'false'
    autoFitMap.value = localStorage.getItem('settings.autoFitMap') !== 'false'
    colorfulLines.value = localStorage.getItem('settings.colorfulLines') === 'true'
    regionValues.value = {}
  }

  const toast = ref<Toast | null>(null)
  let toastTimer: ReturnType<typeof setTimeout> | null = null
  let toastSeq = 0

  function showToast(title: string, options: {body?: string; icon?: ToastIcon} = {}) {
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
    theme,
    locale,
    isDark,
    setTheme,
    setLocale,
    autoCenterOnMe,
    autoFitMap,
    setAutoCenterOnMe,
    setAutoFitMap,
    colorfulLines,
    setColorfulLines,
    showGreenFriday,
    regionSetting,
    setRegionSetting,
    reload,
    toast,
    showToast,
    dismissToast,
  }
})
