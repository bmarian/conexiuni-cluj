<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRemindersStore, reminderTimes, type ReminderTarget} from '@/stores/reminders.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import {useUserStore} from '@/stores/user.ts'
import {useKbdEscape} from '@/composables/useKeyboardNav.ts'
import {network, type DayType} from '@/utils/network.ts'
import {addDays, dayTypeOf, formatSlot, isoWeekday, nextDeparture, nextOf, resolveSlots, type Reminder} from '@/utils/reminders.ts'
import {cityDate} from '@/utils/cityClock.ts'
import {cityMinuteOfDay, walkFrom} from '@/utils/follow.ts'
import {dayLetter, reminderWhen} from '@/utils/reminderText.ts'
import {askNotifications} from '@/utils/notify.ts'
import {pushSupported} from '@/utils/pushSubscription.ts'

const props = defineProps<{ target: ReminderTarget }>()
const emit = defineEmits<{ close: [] }>()

const {t, locale} = useI18n()
const store = useRemindersStore()
const settings = useSettingsStore()
const userStore = useUserStore()

const existing = store.forBus(props.target.stopId, props.target.routeId, props.target.tripId)
const repeat = ref(existing ? !existing.date : false)
const days = ref<number[]>(existing?.days.length ? [...existing.days] : [1, 2, 3, 4, 5, 6, 7])
const last = ref(existing ? existing.time === null : false)
// A new reminder starts on the next bus: the one the user is likely after.
const soonest = ref(!existing)
const time = ref<number | null>(existing?.time ?? null)
const now = ref(Date.now())
const permission = ref(typeof Notification === 'undefined' ? 'denied' : Notification.permission)
const saving = ref(false)
let ticker: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  ticker = setInterval(() => { now.value = Date.now() }, 30_000)
})
onUnmounted(() => {
  if (ticker) clearInterval(ticker)
})
useKbdEscape(() => true, () => emit('close'))

const times = computed(() => {
  void network.value
  return reminderTimes(props.target.stopId, props.target.routeId, props.target.tripId) ?? {}
})
const today = computed(() => cityDate(now.value))
const upcoming = computed(() => nextDeparture(times.value, now.value))

const dayTimes = computed<number[]>(() => {
  const all = times.value as Partial<Record<DayType, number[]>>
  if (repeat.value) {
    const day = Array.from({length: 7}, (_, i) => addDays(today.value, i)).find((d) => days.value.includes(isoWeekday(d))) ?? today.value
    return all[dayTypeOf(day)] ?? []
  }
  const nowMinutes = cityMinuteOfDay(now.value)
  const left = (all[dayTypeOf(today.value)] ?? []).filter((m) => m > nowMinutes)
  return left.length ? left : all[dayTypeOf(addDays(today.value, 1))] ?? []
})

const groups = computed(() => {
  const byHour = new Map<number, number[]>()
  for (const m of dayTimes.value) {
    const hour = Math.floor(m / 60)
    byHour.set(hour, [...(byHour.get(hour) ?? []), m])
  }
  return [...byHour.entries()].map(([hour, minutes]) => ({hour, minutes}))
})

const lastToday = computed(() => (dayTimes.value.length ? Math.max(...dayTimes.value) : null))

const draft = computed<Reminder | null>(() => {
  const next = soonest.value && !repeat.value ? upcoming.value : null
  const chosen = last.value ? null : soonest.value && !repeat.value ? next?.minute ?? null : time.value
  if (!last.value && chosen === null) return null
  const walk = walkFrom(userStore.userLocation, props.target.stopLat, props.target.stopLon)
  const base: Reminder = {
    id: existing?.id ?? crypto.randomUUID(),
    ...props.target,
    time: chosen,
    days: repeat.value ? [...days.value].sort((a, b) => a - b) : [],
    date: null,
    slots: resolveSlots(times.value, chosen),
    walk: walk ?? existing?.walk ?? null,
    walkAt: walk === null ? existing?.walkAt : now.value,
  }
  if (repeat.value) return base.days.length ? base : null
  if (next && !last.value) return {...base, date: next.date}
  return [today.value, addDays(today.value, 1)].map((date) => ({...base, date})).find((r) => nextOf(r, now.value) !== null) ?? null
})

const next = computed(() => (draft.value ? nextOf(draft.value, now.value) : null))

const missing = computed(() => {
  const r = draft.value
  if (!r || !repeat.value) return []
  const labels: Record<DayType, string> = {weekday: t('weekdays'), saturday: t('saturday'), sunday: t('sunday')}
  const picked = new Set(r.days.map((d) => (d === 6 ? 'saturday' : d === 7 ? 'sunday' : 'weekday') as DayType))
  return [...picked].filter((d) => r.slots[d] === undefined).map((d) => `${labels[d]}: ${t('remindNoBus')}`)
})

function toggleDay(day: number) {
  days.value = days.value.includes(day) ? days.value.filter((d) => d !== day) : [...days.value, day]
}

function pick(minute: number) {
  last.value = false
  soonest.value = false
  time.value = minute
}

function pickLast() {
  last.value = true
  soonest.value = false
}

function pickNext() {
  last.value = false
  soonest.value = true
}

function setRepeat(on: boolean) {
  // The next bus is a one-off; repeating keeps its time.
  if (on && soonest.value) {
    soonest.value = false
    time.value = upcoming.value?.minute ?? time.value
  }
  repeat.value = on
}

async function allow() {
  permission.value = (await askNotifications()) === 'granted' ? 'granted' : 'denied'
}

async function save() {
  const reminder = draft.value
  if (!reminder || next.value === null || saving.value) return
  saving.value = true
  try {
    if (!pushSupported()) {
      settings.showToast(t('pushUnsupportedToast'), {icon: 'bell-off'})
      return
    }
    if ((await askNotifications()) !== 'granted') {
      permission.value = 'denied'
      settings.showToast(t('notificationsBlocked'), {icon: 'bell-off'})
      return
    }
    const ok = await store.save(reminder)
    if (ok) settings.showToast(t('remindSaved'), {body: reminderWhen(next.value, locale.value), icon: 'bell'})
    else settings.showToast(t('remindFailed'), {icon: 'bell-off'})
    emit('close')
  } finally {
    saving.value = false
  }
}

function remove() {
  if (existing) void store.remove(existing.id)
  emit('close')
}
</script>

<template>
  <Teleport to="body">
    <div class="rs-scrim" @click.self="emit('close')">
      <section class="rs-sheet" role="dialog" aria-modal="true" :aria-label="t('remindSet')">
        <div class="rs-scroll">
          <header class="rs-head">
            <span class="rs-badge" :style="{ backgroundColor: target.color }">{{ target.line }}</span>
            <div class="min-w-0 flex-1">
              <p class="rs-dest">→ {{ target.destination }}</p>
              <p class="rs-stop">{{ target.stopName }}</p>
            </div>
            <button type="button" class="rs-close" :aria-label="t('close')" @click="emit('close')">
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
              </svg>
            </button>
          </header>

          <div class="rs-choice" role="radiogroup">
            <button type="button" role="radio" :aria-checked="!repeat" :class="{ 'is-on': !repeat }" @click="setRepeat(false)">{{ t('remindOnce') }}</button>
            <button type="button" role="radio" :aria-checked="repeat" :class="{ 'is-on': repeat }" @click="setRepeat(true)">{{ t('remindRepeat') }}</button>
          </div>

          <div v-if="repeat" class="rs-days">
            <button v-for="day in 7" :key="day" type="button" class="rs-day" :class="{ 'is-on': days.includes(day) }"
                    :aria-pressed="days.includes(day)" @click="toggleDay(day)">{{ dayLetter(locale, day) }}</button>
          </div>

          <template v-if="dayTimes.length">
            <div class="rs-pair">
              <button v-if="!repeat && upcoming" type="button" class="rs-last" :class="{ 'is-on': soonest && !last }"
                      :aria-pressed="soonest && !last" @click="pickNext">
                <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 22a2.5 2.5 0 0 0 2.4-2h-4.8a2.5 2.5 0 0 0 2.4 2zm7-6v-5a7 7 0 0 0-5.5-6.8V3.5a1.5 1.5 0 0 0-3 0v.7A7 7 0 0 0 5 11v5l-2 2v1h18v-1l-2-2zM3.6 4.4l1.4-1.4A11 11 0 0 0 1.1 11h2a9 9 0 0 1 .5-6.6zM19 3l1.4 1.4A9 9 0 0 1 20.9 11h2A11 11 0 0 0 19 3z"/>
                </svg>
                {{ t('remindNext') }} · {{ formatSlot(upcoming.minute) }}
              </button>
              <button type="button" class="rs-last" :class="{ 'is-on': last }" :aria-pressed="last" @click="pickLast">
                <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor"><path d="M12.3 2a10 10 0 1 0 9.7 12.4A8 8 0 0 1 12.3 2z"/></svg>
                {{ t(!repeat && upcoming ? 'remindLast' : 'remindLastBus') }}<template v-if="lastToday !== null"> · {{ formatSlot(lastToday) }}</template>
              </button>
            </div>
            <div class="rs-hours">
              <div v-for="group in groups" :key="group.hour" class="rs-hour-row">
                <span class="rs-hour" :class="{ 'is-next-day': group.hour >= 24 }">{{ String(group.hour % 24).padStart(2, '0') }}</span>
                <div class="rs-minutes">
                  <button v-for="minute in group.minutes" :key="minute" type="button" class="rs-time"
                          :class="{ 'is-on': !last && !soonest && time === minute }" :aria-pressed="!last && !soonest && time === minute"
                          @click="pick(minute)">{{ formatSlot(minute).slice(3) }}</button>
                </div>
              </div>
            </div>
          </template>
          <p v-else class="rs-empty">{{ t('noSchedule') }}</p>
        </div>

        <footer class="rs-foot">
          <div v-if="permission !== 'granted'" class="rs-permission">
            <span>{{ t('remindNeedsNotifications') }}</span>
            <button type="button" class="rs-text-btn" @click="allow">{{ t('remindAllow') }}</button>
          </div>
          <div class="rs-actions">
            <div class="min-w-0 flex-1">
              <p v-if="draft" class="rs-next">
                <svg class="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <circle cx="12" cy="13" r="8"/><path stroke-linecap="round" d="M12 9v4l2 2M5 3L2 6m17-3l3 3"/>
                </svg>
                {{ next !== null ? reminderWhen(next, locale, now) : t('remindNoBus') }}
              </p>
              <p v-for="line in missing" :key="line" class="rs-missing">{{ line }}</p>
            </div>
            <button v-if="existing" type="button" class="rs-text-btn" @click="remove">{{ t('remindDelete') }}</button>
            <button type="button" class="rs-save" :disabled="!draft || next === null || saving" @click="save">
              {{ existing ? t('save') : t('remindSet') }}
            </button>
          </div>
        </footer>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.rs-scrim {
  position: fixed;
  inset: 0;
  z-index: 10000;
  background: rgba(15, 23, 42, 0.45);
  display: flex;
  align-items: flex-end;
  justify-content: center;
}

.rs-sheet {
  width: 100%;
  max-width: 32rem;
  max-height: 88vh;
  display: flex;
  flex-direction: column;
  background: var(--color-white, #fff);
  color: var(--color-slate-900, #0f172a);
  border-radius: 1.25rem 1.25rem 0 0;
  box-shadow: 0 -8px 32px rgba(0, 0, 0, 0.2);
}

@media (min-width: 768px) {
  .rs-scrim {
    align-items: center;
  }

  .rs-sheet {
    border-radius: 1.25rem;
  }
}

.rs-scroll {
  overflow-y: auto;
  padding: 1.25rem 1.25rem 0.5rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.rs-head {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.rs-badge {
  min-width: 3rem;
  height: 2.5rem;
  padding: 0 0.5rem;
  border-radius: 0.75rem;
  color: var(--color-white, #fff);
  font-weight: 900;
  display: flex;
  align-items: center;
  justify-content: center;
}

.rs-dest {
  font-weight: 700;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.rs-stop {
  font-size: 0.8125rem;
  color: var(--color-slate-500, #64748b);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.rs-close {
  width: 2rem;
  height: 2rem;
  border-radius: 9999px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-slate-400, #94a3b8);
}

.rs-close:hover {
  background: var(--color-slate-100, #f1f5f9);
}

.rs-choice {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.25rem;
  padding: 0.25rem;
  border-radius: 9999px;
  background: var(--color-slate-100, #f1f5f9);
}

.rs-choice button {
  padding: 0.5rem;
  border-radius: 9999px;
  font-weight: 600;
  font-size: 0.875rem;
  color: var(--color-slate-600, #475569);
}

.rs-choice button.is-on {
  background: var(--color-emerald-500, #10b981);
  color: var(--color-white, #fff);
}

.rs-days {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 0.375rem;
}

.rs-day {
  aspect-ratio: 1;
  border-radius: 0.75rem;
  font-weight: 700;
  background: var(--color-slate-100, #f1f5f9);
  color: var(--color-slate-600, #475569);
}

.rs-day.is-on {
  background: var(--color-emerald-100, #d1fae5);
  color: var(--color-emerald-700, #047857);
}

.rs-pair {
  display: flex;
  gap: 0.5rem;
}

.rs-pair .rs-last {
  flex: 1;
  min-width: 0;
}

.rs-last {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  padding: 0.6rem;
  border-radius: 9999px;
  font-weight: 600;
  background: var(--color-slate-100, #f1f5f9);
  color: var(--color-slate-700, #334155);
}

.rs-last.is-on,
.rs-time.is-on {
  background: var(--color-emerald-500, #10b981);
  color: var(--color-white, #fff);
}

.rs-hours {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.rs-hour-row {
  display: flex;
  align-items: baseline;
  gap: 0.75rem;
}

.rs-hour {
  width: 1.75rem;
  font-weight: 800;
  color: var(--color-emerald-600, #059669);
  font-variant-numeric: tabular-nums;
}

.rs-hour.is-next-day {
  color: var(--color-slate-400, #94a3b8);
}

.rs-minutes {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
}

.rs-time {
  min-width: 2.75rem;
  padding: 0.35rem 0.5rem;
  border-radius: 0.6rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  background: var(--color-slate-100, #f1f5f9);
  color: var(--color-slate-700, #334155);
}

.rs-empty {
  font-size: 0.875rem;
  color: var(--color-slate-400, #94a3b8);
}

.rs-foot {
  padding: 0.75rem 1.25rem calc(1rem + env(safe-area-inset-bottom));
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  border-top: 1px solid var(--color-slate-100, #f1f5f9);
}

.rs-permission {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  font-size: 0.8125rem;
  color: var(--color-slate-600, #475569);
}

.rs-permission span {
  flex: 1;
}

.rs-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.rs-next {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  font-weight: 700;
  color: var(--color-emerald-600, #059669);
}

.rs-missing {
  font-size: 0.75rem;
  color: var(--color-red-600, #dc2626);
}

.rs-text-btn {
  padding: 0.5rem 0.75rem;
  border-radius: 9999px;
  font-weight: 600;
  color: var(--color-emerald-600, #059669);
}

.rs-text-btn:hover {
  background: var(--color-emerald-50, #ecfdf5);
}

.rs-save {
  padding: 0.6rem 1.1rem;
  border-radius: 9999px;
  font-weight: 700;
  background: var(--color-emerald-500, #10b981);
  color: var(--color-white, #fff);
}

.rs-save:disabled {
  opacity: 0.45;
}

:root.dark .rs-sheet {
  background: var(--color-slate-900, #0f172a);
  color: var(--color-slate-100, #f1f5f9);
}

:root.dark .rs-choice,
:root.dark .rs-day,
:root.dark .rs-last,
:root.dark .rs-time {
  background: var(--color-slate-800, #1e293b);
  color: var(--color-slate-300, #cbd5e1);
}

:root.dark .rs-choice {
  color: var(--color-slate-300, #cbd5e1);
}

:root.dark .rs-choice button {
  color: var(--color-slate-300, #cbd5e1);
}

:root.dark .rs-choice button.is-on,
:root.dark .rs-last.is-on,
:root.dark .rs-time.is-on {
  background: var(--color-emerald-500, #10b981);
  color: var(--color-white, #fff);
}

:root.dark .rs-day.is-on {
  background: var(--color-emerald-900, #064e3b);
  color: var(--color-emerald-300, #6ee7b7);
}

:root.dark .rs-stop,
:root.dark .rs-permission {
  color: var(--color-slate-400, #94a3b8);
}

:root.dark .rs-foot {
  border-color: var(--color-slate-800, #1e293b);
}

:root.dark .rs-close:hover,
:root.dark .rs-text-btn:hover {
  background: var(--color-slate-800, #1e293b);
}

:root.dark .rs-next,
:root.dark .rs-text-btn,
:root.dark .rs-hour {
  color: var(--color-emerald-400, #34d399);
}
</style>
