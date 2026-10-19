import {ref, shallowRef, watch} from 'vue'
import {defineStore} from 'pinia'
import {apiPost} from '@/utils/api.ts'
import {OUTGOING_SUFFIX} from '@/types/tranzy.ts'
import {buildHourlyOffsets, buildStopInfo, network, type DayType} from '@/utils/network.ts'
import {getShapeStopTimes, getTimeOffsetToStop} from '@/utils/trips.ts'
import {DAY_TYPES, MAX_REMINDERS, nextDeparture, nextOf, resolveSlots, stopTimesByDayType, type Reminder} from '@/utils/reminders.ts'
import {pushRegistration, pushSubscription, pushSupported, retractReminder} from '@/utils/pushSubscription.ts'
import {translate as t} from '@/i18n.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import {usePushStore} from '@/stores/push.ts'
import {cacheKey, citiesSaving, currentCity} from '@/utils/city.ts'

export const REMINDERS_KEY = 'reminders.v1'
const storageKey = (city?: string) => cacheKey(REMINDERS_KEY, city)
const syncedKey = (city?: string) => cacheKey('reminders.synced', city)

function load(city?: string): Reminder[] {
  try {
    const list = JSON.parse(localStorage.getItem(storageKey(city)) ?? '[]')
    return Array.isArray(list) ? list.filter((r) => r && typeof r.id === 'string' && r.slots) : []
  } catch {
    return []
  }
}

export function reminderTimes(stopId: number, routeId: number, tripId: string): Partial<Record<DayType, number[]>> | null {
  const index = network.value
  const route = index?.routeById.get(routeId)
  if (!index || !route?.timetable) return null
  const info = buildStopInfo(index, stopId)
  const shape = info?.shapes_info.find((s) => s.route_id === routeId)
  const fallback = shape ? getTimeOffsetToStop(getShapeStopTimes(shape), tripId, stopId) : 0
  const hourly = Object.fromEntries(DAY_TYPES.map((day) => [day, buildHourlyOffsets(index, route, day)[tripId]]))
  return stopTimesByDayType(route.timetable, tripId.endsWith(OUTGOING_SUFFIX), stopId, hourly, fallback)
}

export type ReminderTarget = Pick<Reminder, 'stopId' | 'stopName' | 'stopLat' | 'stopLon' | 'routeId' | 'tripId' | 'line' | 'color' | 'destination'>

/** A one-off reminder for the next bus of [target]; null when the timetable has none. */
export function nextBusReminder(target: ReminderTarget, walk: number | null, nowMs = Date.now()): Reminder | null {
  const times = reminderTimes(target.stopId, target.routeId, target.tripId)
  const next = times && nextDeparture(times, nowMs)
  if (!times || !next) return null
  return {
    id: crypto.randomUUID(), ...target, time: next.minute, days: [], date: next.date, slots: resolveSlots(times, next.minute),
    walk, walkAt: walk === null ? undefined : nowMs,
  }
}

export const useRemindersStore = defineStore('reminders', () => {
  const settings = useSettingsStore()
  const reminders = ref<Reminder[]>(load())
  const editing = shallowRef<ReminderTarget | null>(null)
  const syncing = new Set<string>()
  const again = new Set<string>()

  watch(currentCity, () => {
    reminders.value = load()
    editing.value = null
  }, {flush: 'sync'})

  function persist() {
    try {
      localStorage.setItem(storageKey(), JSON.stringify(reminders.value))
    } catch {}
  }

  function forBus(stopId: number, routeId: number, tripId: string): Reminder | undefined {
    return reminders.value.find((r) => r.stopId === stopId && r.routeId === routeId && r.tripId === tripId)
  }

  // Off: the server gets none; the list stays here.
  function payload(list: Reminder[]) {
    if (!settings.busAlerts) return []
    return list.map((r) => ({
      id: r.id,
      stop_id: r.stopId,
      stop_name: r.stopName,
      route_id: r.routeId,
      trip_id: r.tripId,
      line: r.line,
      destination: r.destination,
      days: r.date ? [] : r.days,
      date: r.date ?? '',
      slots: r.slots,
      walk: r.walk ?? undefined,
      walk_at: r.walk == null ? undefined : r.walkAt,
    }))
  }

  async function sync(city = currentCity.value): Promise<boolean> {
    if (!pushSupported()) return false
    const hadServer = localStorage.getItem(syncedKey(city)) === 'true'
    const list = payload(city === currentCity.value ? reminders.value : load(city))
    if (!list.length && !hadServer) return true
    if (Notification.permission !== 'granted') return false
    if (syncing.has(city)) {
      again.add(city)
      return true
    }
    syncing.add(city)
    try {
      const reg = await pushRegistration()
      if (!reg) return false
      const sub = await pushSubscription(reg)
      await apiPost('push/reminders', {...sub.toJSON(), locale: settings.locale, reminders: list}, city)
      localStorage.setItem(syncedKey(city), list.length ? 'true' : 'false')
      if (!list.length) await usePushStore().settleCity(city)
      return true
    } catch (err) {
      console.warn('Reminder sync failed:', err)
      return false
    } finally {
      syncing.delete(city)
      if (again.delete(city)) void sync(city)
    }
  }

  async function syncAll() {
    for (const city of new Set([currentCity.value, ...citiesSaving(REMINDERS_KEY)])) await sync(city)
  }

  watch(() => settings.busAlerts, () => void syncAll())

  const full = (id?: string) => reminders.value.length >= MAX_REMINDERS && !reminders.value.some((r) => r.id === id)

  async function save(reminder: Reminder) {
    if (full(reminder.id)) return false
    reminders.value = [...reminders.value.filter((r) => r.id !== reminder.id), reminder]
    persist()
    return sync()
  }

  function remove(id: string) {
    const gone = reminders.value.find((r) => r.id === id)
    if (gone && pushSupported()) void retractReminder(id, nextOf(gone, Date.now()) ?? Date.now())
    reminders.value = reminders.value.filter((r) => r.id !== id)
    persist()
    return sync()
  }

  function refresh(nowMs = Date.now()) {
    let changed = false
    const kept: Reminder[] = []
    for (const reminder of reminders.value) {
      const times = reminderTimes(reminder.stopId, reminder.routeId, reminder.tripId)
      let next = reminder
      if (times) {
        const slots = resolveSlots(times, reminder.time)
        if (Object.keys(slots).length && JSON.stringify(slots) !== JSON.stringify(reminder.slots)) {
          next = {...reminder, slots}
          changed = true
        }
      }
      if (next.date && nextOf(next, nowMs) === null) {
        changed = true
        continue
      }
      kept.push(next)
    }
    if (changed) {
      reminders.value = kept
      persist()
    }
    void sync()
  }

  watch(network, (index) => {
    if (index) refresh()
  }, {immediate: true})
  watch(() => settings.locale, () => void syncAll())

  function openSheet(target: ReminderTarget) {
    if (!settings.busAlerts) return
    if (full(forBus(target.stopId, target.routeId, target.tripId)?.id)) {
      settings.showToast(t('limitReached', {n: MAX_REMINDERS}), {icon: 'bell-off'})
      return
    }
    editing.value = target
  }

  function closeSheet() {
    editing.value = null
  }

  return {reminders, editing, forBus, full, save, remove, refresh, sync, openSheet, closeSheet}
})
