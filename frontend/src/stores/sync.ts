import {defineStore} from 'pinia'
import {ref, watch} from 'vue'
import {useFavoritesStore} from '@/stores/favorites'
import {useRouteUpdatesStore} from '@/stores/routeUpdates'
import {useSettingsStore} from '@/stores/settings'
import {i18n, translate} from '@/i18n'
import {currentCity} from '@/utils/city.ts'
import {canSync} from '@/utils/sync/crypto.ts'
import {deviceLabel} from '@/utils/sync/local.ts'
import * as engine from '@/utils/sync/engine.ts'
import type {DeviceView, Problem} from '@/utils/sync/engine.ts'

const SETTINGS_KEY = 'sync.settings'
// Collects a burst of edits (a reorder, a few toggles) into one write.
const AFTER_EDIT_MS = 2000

function readFlag(key: string, fallback: boolean): boolean {
  try {
    const saved = localStorage.getItem(key)
    return saved === null ? fallback : saved === 'true'
  } catch {
    return fallback
  }
}

// Only one tab syncs at a time; the others pick its changes up from storage events.
async function exclusive<T>(fn: () => Promise<T>): Promise<T> {
  return navigator.locks ? await navigator.locks.request('cai-sync', fn) as T : fn()
}

export const useSyncStore = defineStore('sync', () => {
  const status = ref<'unknown' | 'off' | 'on'>('unknown')
  const busy = ref(false)
  const devices = ref<DeviceView[]>([])
  const problems = ref<Problem[]>([])
  const lastSynced = ref<number | null>(null)
  const removed = ref(false)
  const offline = ref(false)
  const withSettings = ref(readFlag(SETTINGS_KEY, true))
  let timer: ReturnType<typeof setTimeout> | undefined
  let running: Promise<void> | null = null
  let again = false
  let started = false

  async function syncNow(): Promise<void> {
    if (status.value !== 'on') return
    if (running) {
      again = true
      return running
    }
    running = (async () => {
      busy.value = true
      try {
        do {
          again = false
          const report = await exclusive(() => engine.syncOnce(withSettings.value, deviceLabel()))
          devices.value = report.devices
          problems.value = report.problems
          lastSynced.value = Date.now()
          offline.value = false
          if (report.dropped) useSettingsStore().showToast(translate('syncDropped', {n: report.dropped}))
        } while (again)
      } catch (err) {
        if (err instanceof engine.Gone) {
          status.value = 'off'
          removed.value = true
          devices.value = []
        } else if (err instanceof TypeError) {
          offline.value = true
        } else {
          problems.value = [...new Set([...problems.value, 'broken' as Problem])]
        }
      } finally {
        busy.value = false
        running = null
      }
    })()
    return running
  }

  function schedule() {
    if (status.value !== 'on') return
    if (timer) clearTimeout(timer)
    timer = setTimeout(() => void syncNow(), AFTER_EDIT_MS)
  }

  function onStorage(e: StorageEvent) {
    if (!e.key) return
    if (e.key.startsWith(`${currentCity.value}:favorites:`) || e.key.startsWith(`${currentCity.value}:recents:`)) void useFavoritesStore().hydrate()
    else if (e.key === `${currentCity.value}:follows:routes`) useRouteUpdatesStore().reload()
    else if (e.key.startsWith('settings.') || e.key.includes(':settings.')) {
      const settings = useSettingsStore()
      settings.reload()
      i18n.global.locale.value = settings.locale
    }
  }

  async function start() {
    if (started || !canSync()) {
      if (!canSync()) status.value = 'off'
      return
    }
    started = true
    status.value = (await engine.membership()) ? 'on' : 'off'
    const favorites = useFavoritesStore()
    const routeUpdates = useRouteUpdatesStore()
    const settings = useSettingsStore()
    watch(() => [favorites.favoriteStopIds, favorites.favoriteRoutes, favorites.favoritePlans, routeUpdates.followed], schedule, {deep: true})
    watch(() => [settings.theme, settings.locale, settings.autoCenterOnMe, settings.autoFitMap, settings.regionSetting('cluj', 'showGreenFriday')], () => {
      if (withSettings.value) schedule()
    })
    window.addEventListener('storage', onStorage)
    window.addEventListener('online', () => void syncNow())
    // A desktop window coming back to the front stays "visible" the whole time; focus is what changes.
    window.addEventListener('focus', () => void syncNow())
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'visible') void syncNow()
    })
    void syncNow()
  }

  // A pairing just made this device a member.
  async function joined() {
    removed.value = false
    problems.value = []
    status.value = 'on'
    await syncNow()
  }

  function setWithSettings(value: boolean) {
    withSettings.value = value
    try {
      localStorage.setItem(SETTINGS_KEY, String(value))
    } catch { /* noop */ }
    if (value) void syncNow()
  }

  async function removeDevice(id: string) {
    busy.value = true
    try {
      await exclusive(() => engine.removeDevice(id, withSettings.value, deviceLabel()))
    } finally {
      busy.value = false
    }
    await syncNow()
  }

  async function stop(everywhere: boolean) {
    busy.value = true
    try {
      await exclusive(() => everywhere ? engine.deleteEverything() : engine.leave())
      status.value = 'off'
      devices.value = []
      problems.value = []
    } finally {
      busy.value = false
    }
  }

  return {
    status, busy, devices, problems, lastSynced, removed, offline, withSettings,
    start, syncNow, joined, setWithSettings, removeDevice, stop,
  }
})
