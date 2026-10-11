import {ref, shallowRef, watch} from 'vue'
import {defineStore} from 'pinia'
import {apiPost} from '@/utils/api.ts'
import {OUTGOING_SUFFIX} from '@/types/tranzy.ts'
import {buildHourlyOffsets, buildStopInfo, network, type DayType} from '@/utils/network.ts'
import {getShapeStopTimes, getTimeOffsetToStop} from '@/utils/trips.ts'
import {DAY_TYPES, MAX_REMINDERS, nextOf, resolveSlots, stopTimesByDayType, type Reminder} from '@/utils/reminders.ts'
import {pushRegistration, pushSubscription, pushSupported} from '@/utils/pushSubscription.ts'
import {translate as t} from '@/i18n.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import {usePushStore} from '@/stores/push.ts'

const STORAGE_KEY = 'reminders.v1'
const SYNCED_KEY = 'reminders.synced'

function load(): Reminder[] {
  try {
    const list = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]')
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

export type ReminderTarget = Pick<Reminder, 'stopId' | 'stopName' | 'routeId' | 'tripId' | 'line' | 'color' | 'destination'>

export const useRemindersStore = defineStore('reminders', () => {
  const settings = useSettingsStore()
  const reminders = ref<Reminder[]>(load())
  const editing = shallowRef<ReminderTarget | null>(null)
  const syncing = ref(false)
  let again = false

  function persist() {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(reminders.value))
    } catch {}
  }

  function forBus(stopId: number, routeId: number, tripId: string): Reminder | undefined {
    return reminders.value.find((r) => r.stopId === stopId && r.routeId === routeId && r.tripId === tripId)
  }

  function payload() {
    return reminders.value.map((r) => ({
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
    }))
  }

  async function sync(): Promise<boolean> {
    if (!pushSupported()) return false
    const hadServer = localStorage.getItem(SYNCED_KEY) === 'true'
    if (!reminders.value.length && !hadServer) return true
    if (Notification.permission !== 'granted') return false
    if (syncing.value) {
      again = true
      return true
    }
    syncing.value = true
    try {
      const reg = await pushRegistration()
      if (!reg) return false
      const sub = await pushSubscription(reg)
      await apiPost('push/reminders', {...sub.toJSON(), locale: settings.locale, reminders: payload()})
      localStorage.setItem(SYNCED_KEY, reminders.value.length ? 'true' : 'false')
      if (!reminders.value.length && !usePushStore().enabled) await usePushStore().disable()
      return true
    } catch (err) {
      console.warn('Reminder sync failed:', err)
      return false
    } finally {
      syncing.value = false
      if (again) {
        again = false
        void sync()
      }
    }
  }

  const full = (id?: string) => reminders.value.length >= MAX_REMINDERS && !reminders.value.some((r) => r.id === id)

  async function save(reminder: Reminder) {
    if (full(reminder.id)) return false
    reminders.value = [...reminders.value.filter((r) => r.id !== reminder.id), reminder]
    persist()
    return sync()
  }

  function remove(id: string) {
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
  watch(() => settings.locale, () => void sync())

  function openSheet(target: ReminderTarget) {
    if (full(forBus(target.stopId, target.routeId, target.tripId)?.id)) {
      settings.showToast(t('limitReached', {n: MAX_REMINDERS}), {icon: 'bell-off'})
      return
    }
    editing.value = target
  }

  function closeSheet() {
    editing.value = null
  }

  return {reminders, syncing, editing, forBus, save, remove, refresh, sync, openSheet, closeSheet}
})
