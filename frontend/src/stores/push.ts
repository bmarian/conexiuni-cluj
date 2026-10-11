import {ref, watch} from 'vue'
import {defineStore} from 'pinia'
import {apiPost} from '@/utils/api'
import {useRouteUpdatesStore} from '@/stores/routeUpdates'
import {useSettingsStore} from '@/stores/settings'
import {useRemindersStore} from '@/stores/reminders'
import {pushRegistration, pushSubscription, pushSupported} from '@/utils/pushSubscription'

const ENABLED_KEY = 'settings.pushNotifications'
const SYNC_DELAY_MS = 1000

export type PushResult = 'enabled' | 'unsupported' | 'denied' | 'brave' | 'failed'

export const usePushStore = defineStore('push', () => {
  const settings = useSettingsStore()
  const routeUpdates = useRouteUpdatesStore()

  const supported = pushSupported()
  const enabled = ref(supported && localStorage.getItem(ENABLED_KEY) === 'true')
  const busy = ref(false)
  let syncTimer: ReturnType<typeof setTimeout> | null = null

  function setEnabled(value: boolean) {
    enabled.value = value
    localStorage.setItem(ENABLED_KEY, value ? 'true' : 'false')
  }

  async function register(sub: PushSubscription) {
    await apiPost('push/subscribe', {
      ...sub.toJSON(),
      routes: enabled.value ? routeUpdates.followed : [],
      locale: settings.locale,
    })
  }

  async function enable(): Promise<PushResult> {
    if (!supported) return 'unsupported'
    busy.value = true
    try {
      if (await Notification.requestPermission() !== 'granted') return 'denied'
      const reg = await pushRegistration()
      if (!reg) return 'failed'
      setEnabled(true)
      await register(await pushSubscription(reg))
      return 'enabled'
    } catch (err) {
      setEnabled(false)
      console.warn('Push subscribe failed:', err)
      // Brave keeps Google's push service off until the user opts in.
      if ('brave' in navigator && err instanceof DOMException && err.name === 'AbortError') return 'brave'
      return 'failed'
    } finally {
      busy.value = false
    }
  }

  async function disable() {
    setEnabled(false)
    if (!supported) return
    try {
      const sub = await (await pushRegistration())?.pushManager.getSubscription()
      if (!sub) return
      if (useRemindersStore().reminders.length) {
        await register(sub)
        return
      }
      const {endpoint} = sub
      await sub.unsubscribe()
      await apiPost('push/unsubscribe', {endpoint})
    } catch (err) {
      console.warn('Push unsubscribe failed:', err)
    }
  }

  async function sync() {
    if (!enabled.value) return
    if (!settings.showTimetableChanges || Notification.permission !== 'granted') return disable()
    try {
      const reg = await pushRegistration()
      if (reg) await register(await pushSubscription(reg))
    } catch (err) {
      console.warn('Push sync failed:', err)
    }
  }

  function scheduleSync() {
    if (!enabled.value) return
    if (syncTimer) clearTimeout(syncTimer)
    syncTimer = setTimeout(() => void sync(), SYNC_DELAY_MS)
  }

  watch(() => [...routeUpdates.followed], scheduleSync)
  watch(() => settings.locale, scheduleSync)
  watch(() => settings.showTimetableChanges, scheduleSync)

  return {supported, enabled, busy, enable, disable, sync}
})
