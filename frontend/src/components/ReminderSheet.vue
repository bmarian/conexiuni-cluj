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
import {AlarmIcon, BedtimeIcon, CloseIcon, NotificationsActiveIcon} from '@/utils/icons.ts'
import LineBadge from '@/components/ui/LineBadge.vue'
import ChoiceRow from '@/components/ui/ChoiceRow.vue'
import PopIcon from '@/components/ui/PopIcon.vue'
import Ticker from '@/components/ui/Ticker.vue'

const props = defineProps<{ target: ReminderTarget }>()
const emit = defineEmits<{ close: [] }>()

const {t, locale} = useI18n()
const repeatChoices = computed(() => [{value: 'once' as const, label: t('remindOnce')}, {value: 'repeat' as const, label: t('remindRepeat')}])
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
    <div class="scrim" @click.self="emit('close')">
      <section class="sheet" role="dialog" aria-modal="true" :aria-label="t('remindSet')">
        <span class="grip" aria-hidden="true"></span>
        <div class="scroll">
          <header class="flex items-center gap-3">
            <LineBadge :name="target.line" :color="target.color" :height="40" :min-width="48" :corner="12"/>
            <div class="min-w-0 flex-1">
              <p class="text-title-medium font-bold truncate">→ {{ target.destination }}</p>
              <p class="text-body-medium text-on-surface-variant truncate">{{ target.stopName }}</p>
            </div>
            <button type="button" class="icon-btn" :aria-label="t('close')" :title="t('close')" @click="emit('close')">
              <CloseIcon class="m-icon"/>
            </button>
          </header>

          <ChoiceRow :options="repeatChoices" :model-value="repeat ? 'repeat' : 'once'" @update:model-value="setRepeat($event === 'repeat')"/>

          <Transition name="days">
            <div v-if="repeat" class="days">
              <button
                v-for="day in 7"
                :key="day"
                type="button"
                class="day squish"
                :class="{ 'is-on': days.includes(day) }"
                :aria-pressed="days.includes(day)"
                @click="toggleDay(day)"
              >{{ dayLetter(locale, day) }}</button>
            </div>
          </Transition>

          <template v-if="dayTimes.length">
            <div class="flex gap-2">
              <button
                v-if="!repeat && upcoming"
                type="button"
                class="choice squish flex-1"
                :class="{ 'is-on': soonest && !last }"
                :aria-pressed="soonest && !last"
                @click="pickNext"
              >
                <PopIcon :icon="NotificationsActiveIcon" :on="soonest && !last" class="size-[18px]"/>
                {{ t('remindNext') }} · {{ formatSlot(upcoming.minute) }}
              </button>
              <button type="button" class="choice squish flex-1" :class="{ 'is-on': last }" :aria-pressed="last" @click="pickLast">
                <PopIcon :icon="BedtimeIcon" :on="last" class="size-[18px]"/>
                {{ t(!repeat && upcoming ? 'remindLast' : 'remindLastBus') }}<template v-if="lastToday !== null"> · {{ formatSlot(lastToday) }}</template>
              </button>
            </div>
            <div>
              <div v-for="group in groups" :key="group.hour" class="hour-row">
                <span class="hour" :class="{ 'is-next-day': group.hour >= 24 }">{{ String(group.hour % 24).padStart(2, '0') }}</span>
                <div class="minutes">
                  <button
                    v-for="minute in group.minutes"
                    :key="minute"
                    type="button"
                    class="choice squish"
                    :class="{ 'is-on': !last && !soonest && time === minute }"
                    :aria-pressed="!last && !soonest && time === minute"
                    @click="pick(minute)"
                  >{{ formatSlot(minute).slice(3) }}</button>
                </div>
              </div>
            </div>
          </template>
          <p v-else class="text-body-medium text-on-surface-variant">{{ t('noSchedule') }}</p>
        </div>

        <footer class="foot">
          <div v-if="permission !== 'granted'" class="permission">
            <span class="flex-1 text-body-small">{{ t('remindNeedsNotifications') }}</span>
            <button type="button" class="btn btn-text" @click="allow">{{ t('remindAllow') }}</button>
          </div>
          <div class="flex items-center gap-2">
            <div class="min-w-0 flex-1">
              <p v-if="draft" class="flex items-center gap-1.5 text-title-small">
                <AlarmIcon class="m-icon size-[18px] text-primary"/>
                <Ticker :text="next !== null ? reminderWhen(next, locale, now) : t('remindNoBus')"/>
              </p>
              <p v-for="line in missing" :key="line" class="text-body-small text-error">{{ line }}</p>
            </div>
            <button v-if="existing" type="button" class="btn btn-text" @click="remove">{{ t('remindDelete') }}</button>
            <button type="button" class="btn btn-filled" :disabled="!draft || next === null || saving" @click="save">
              <AlarmIcon class="m-icon"/>
              {{ existing ? t('save') : t('remindSet') }}
            </button>
          </div>
        </footer>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.scrim {
  position: fixed;
  inset: 0;
  z-index: 9500;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  background: var(--md-scrim);
  animation: fade 200ms linear;
}

@keyframes fade {
  from {
    opacity: 0;
  }
}

.sheet {
  display: flex;
  flex-direction: column;
  width: min(640px, 100%);
  max-height: calc(100dvh - var(--safe-top) - 56px);
  border-radius: 28px 28px 0 0;
  background: var(--md-surface-container-low);
  color: var(--md-on-surface);
  box-shadow: var(--md-elevation-1);
  animation: rise var(--spring-sheet-ms) var(--spring-sheet);
}

@keyframes rise {
  from {
    translate: 0 100%;
  }
}

.grip {
  align-self: center;
  flex-shrink: 0;
  width: 32px;
  height: 4px;
  margin: 22px 0;
  border-radius: 2px;
  background: color-mix(in srgb, var(--md-on-surface-variant) 40%, transparent);
}

.scroll {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 0;
  padding: 0 20px;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.days {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 6px;
}

.day {
  --squish: 0.9;
  aspect-ratio: 1;
  border: 0;
  border-radius: 9999px;
  background: var(--md-surface-container-highest);
  color: var(--md-on-surface-variant);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: border-radius var(--spring-fast-ms) var(--spring-fast), background-color 150ms linear, color 150ms linear, scale var(--spring-press-ms) var(--spring-press);
}

.day.is-on {
  border-radius: 12px;
  background: var(--md-primary);
  color: var(--md-on-primary);
}

.days-enter-active,
.days-leave-active {
  transition: opacity 200ms linear, translate var(--spring-fast-ms) var(--spring-fast);
}

.days-enter-from,
.days-leave-to {
  opacity: 0;
  translate: 0 -8px;
}

.choice {
  --squish: 0.88;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 36px;
  padding: 8px 12px;
  border: 0;
  border-radius: 8px;
  background: var(--md-surface-container-highest);
  color: var(--md-on-surface);
  font-size: 14px;
  line-height: 20px;
  font-weight: 500;
  cursor: pointer;
  transition: border-radius var(--spring-fast-ms) var(--spring-fast), background-color 150ms linear, color 150ms linear, scale var(--spring-press-ms) var(--spring-press);
}

.choice.is-on {
  border-radius: 18px;
  background: var(--md-primary);
  color: var(--md-on-primary);
}

.hour-row {
  display: flex;
  align-items: stretch;
}

.hour {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-shrink: 0;
  width: 36px;
  padding-right: 10px;
  border-right: 2px solid var(--md-outline-variant);
  font-size: 14px;
  line-height: 20px;
  font-weight: 700;
  color: var(--md-primary);
}

.hour.is-next-day {
  color: var(--md-outline);
}

.minutes {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 3px 0 3px 10px;
}

.foot {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 12px 20px calc(16px + var(--safe-bottom));
}

.permission {
  display: flex;
  align-items: center;
  padding: 6px 8px 6px 16px;
  border-radius: 16px;
  background: var(--md-secondary-container);
  color: var(--md-on-secondary-container);
}
</style>
