import {ref, watch} from 'vue'
import {defineStore} from 'pinia'
import {apiPost} from '@/utils/api'
import {FOLLOWS_KEY, useRouteUpdatesStore} from '@/stores/routeUpdates'
import {useSettingsStore} from '@/stores/settings'
import {REMINDERS_KEY} from '@/stores/reminders'
import {citiesSaving, currentCity, knownCity} from '@/utils/city.ts'
import {pushRegistration, pushSubscription, pushSupported} from '@/utils/pushSubscription'
import {translate as t} from '@/i18n.ts'

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

  const alertCities = () => [...new Set([currentCity.value, ...citiesSaving(FOLLOWS_KEY).filter(knownCity)])]

  async function register(sub: PushSubscription, city: string) {
    await apiPost('push/subscribe', {
      ...sub.toJSON(),
      routes: enabled.value ? routeUpdates.followedIn(city) : [],
      locale: settings.locale,
    }, city)
  }

  async function settleServer(sub: PushSubscription, city: string) {
    const remindersHere = citiesSaving(REMINDERS_KEY).includes(city)
    if (remindersHere || (enabled.value && routeUpdates.followedIn(city).length)) return register(sub, city)
    await apiPost('push/unsubscribe', {endpoint: sub.endpoint}, city)
  }

  async function releaseUnused(sub: PushSubscription) {
    if (!citiesSaving(REMINDERS_KEY).length && !(enabled.value && citiesSaving(FOLLOWS_KEY).length)) await sub.unsubscribe()
  }

  async function settle(sub: PushSubscription, city: string) {
    await settleServer(sub, city)
    await releaseUnused(sub)
  }

  async function enable(): Promise<PushResult> {
    if (!supported) return 'unsupported'
    busy.value = true
    try {
      if (await Notification.requestPermission() !== 'granted') return 'denied'
      const reg = await pushRegistration()
      if (!reg) return 'failed'
      setEnabled(true)
      const sub = await pushSubscription(reg)
      for (const city of alertCities()) await register(sub, city)
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

  async function ask() {
    if (enabled.value || busy.value) return
    const result = await enable()
    if (result === 'enabled') return
    const key = {
      unsupported: 'pushUnsupportedToast',
      denied: 'pushDeniedToast',
      brave: 'pushBraveToast',
      failed: 'pushFailedToast',
    }[result]
    const isIOS = /iPhone|iPad/.test(navigator.userAgent)
    const body = result === 'unsupported' && !isIOS ? undefined : t(`${key}Body`)
    settings.showToast(t(key), {body, icon: 'bell-off'})
  }

  async function currentSub() {
    return (await pushRegistration())?.pushManager.getSubscription() ?? null
  }

  async function disable() {
    setEnabled(false)
    if (!supported) return
    try {
      const sub = await currentSub()
      if (!sub) return
      for (const city of alertCities()) await settleServer(sub, city)
      await releaseUnused(sub)
    } catch (err) {
      console.warn('Push unsubscribe failed:', err)
    }
  }

  async function settleCity(city: string) {
    if (!supported) return
    try {
      const sub = await currentSub()
      if (sub) await settle(sub, city)
    } catch (err) {
      console.warn('Push settle failed:', err)
    }
  }

  async function sync(city = currentCity.value) {
    if (!enabled.value) return
    if (!citiesSaving(FOLLOWS_KEY).length || Notification.permission !== 'granted') return disable()
    try {
      const reg = await pushRegistration()
      if (reg) await settle(await pushSubscription(reg), city)
    } catch (err) {
      console.warn('Push sync failed:', err)
    }
  }

  const pending = new Set<string>()
  function scheduleSync(city = currentCity.value) {
    if (!enabled.value) return
    pending.add(city)
    if (syncTimer) clearTimeout(syncTimer)
    syncTimer = setTimeout(async () => {
      const cities = [...pending]
      pending.clear()
      for (const c of cities) await sync(c)
    }, SYNC_DELAY_MS)
  }

  // A city switch changes the list, not what to send.
  watch(() => [currentCity.value, [...routeUpdates.followed]] as const, ([city], [previous]) => {
    if (city === previous) scheduleSync(city)
  })
  watch(() => settings.locale, () => citiesSaving(FOLLOWS_KEY).filter(knownCity).forEach((city) => scheduleSync(city)))

  return {supported, enabled, busy, ask, disable, sync, settleCity}
})
