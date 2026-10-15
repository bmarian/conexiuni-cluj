<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'

type Range = {name: string; start: string; end: string}

type SchoolYear = {
  year: number
  start: string
  end: string
  breaks: Range[]
  status: 'saved' | 'built_in' | 'estimate'
}

type DayOverride = {date: string; schedule: string; source: string; detail: string}

type CalendarDay = {
  date: string
  schedule_day: string
  automatic: string
  profile_day: string
  holiday?: string
  override?: DayOverride
}

type CalendarView = {
  today: {date: string; profile_day: string; schedule_day: string; holiday?: string}
  editable: boolean
  needed?: number
  years: SchoolYear[]
  holidays: {date: string; name: string; schedule: string}[]
  overrides: DayOverride[]
}

const emit = defineEmits<{saved: []}>()

const view = ref<CalendarView | null>(null)
const loadError = ref('')
const selectedYear = ref<number | null>(null)
const draft = ref<SchoolYear | null>(null)
const saving = ref(false)
const saveMessage = ref('')
const saveFailed = ref(false)
const showAllHolidays = ref(false)
const dayDate = ref('')
const day = ref<CalendarDay | null>(null)
const dayBusy = ref(false)
const dayError = ref('')

const dayChoices = [
  {value: '', label: 'Automatic'},
  {value: 'weekday', label: 'Weekday'},
  {value: 'saturday', label: 'Saturday'},
  {value: 'sunday', label: 'Sunday'},
]

const dayLabels: Record<string, string> = {
  weekday: 'a school-term weekday',
  weekday_vacation: 'a school-holiday weekday',
  saturday: 'a Saturday',
  sunday: 'a Sunday',
}

const timetableLabels: Record<string, string> = {
  weekday: 'weekday timetable',
  saturday: 'Saturday timetable',
  sunday: 'Sunday timetable',
}

const statusLabels: Record<SchoolYear['status'], string> = {
  saved: 'saved',
  built_in: 'built in',
  estimate: 'estimate',
}

function dayName(date: string) {
  return new Date(`${date}T12:00:00`).toLocaleDateString('en-GB', {weekday: 'short', day: 'numeric', month: 'short', year: 'numeric'})
}

const yearLabel = (year: number) => `${year}–${String(year + 1).slice(2)}`

function pick(year: number) {
  const found = view.value?.years.find((y) => y.year === year)
  if (!found) return
  selectedYear.value = year
  draft.value = {...found, breaks: found.breaks.map((b) => ({...b}))}
  saveMessage.value = ''
}

async function load() {
  try {
    const res = await fetch('/api/admin/calendar', {credentials: 'same-origin'})
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    show(await res.json() as CalendarView)
    loadError.value = ''
    if (!dayDate.value) {
      dayDate.value = view.value?.today.date ?? ''
      await loadDay()
    }
  } catch (err) {
    loadError.value = (err as Error).message
  }
}

function show(next: CalendarView) {
  view.value = next
  const keep = next.years.find((y) => y.year === selectedYear.value)
  const preferred = next.years.find((y) => y.year === next.needed) ?? next.years.find((y) => y.status !== 'estimate') ?? next.years[0]
  const target = keep ?? preferred
  if (target) pick(target.year)
}

async function save() {
  const d = draft.value
  if (!d || saving.value) return
  saving.value = true
  saveMessage.value = ''
  try {
    const res = await fetch('/api/admin/calendar', {
      method: 'PUT',
      headers: {'Content-Type': 'application/json'},
      credentials: 'same-origin',
      body: JSON.stringify({year: d.year, start: d.start, end: d.end, breaks: d.breaks}),
    })
    const body = await res.json().catch(() => null)
    if (!res.ok) throw new Error(body?.error ?? `HTTP ${res.status}`)
    show(body as CalendarView)
    saveFailed.value = false
    saveMessage.value = 'Saved. Learning switches over within a minute.'
    emit('saved')
  } catch (err) {
    saveFailed.value = true
    saveMessage.value = (err as Error).message
  } finally {
    saving.value = false
  }
}

async function loadDay() {
  if (!dayDate.value) return
  dayError.value = ''
  try {
    const res = await fetch(`/api/admin/calendar/day?date=${encodeURIComponent(dayDate.value)}`, {credentials: 'same-origin'})
    const body = await res.json().catch(() => null)
    if (!res.ok) throw new Error(body?.error ?? `HTTP ${res.status}`)
    day.value = body as CalendarDay
  } catch (err) {
    day.value = null
    dayError.value = (err as Error).message
  }
}

async function setDay(schedule: string, date = dayDate.value) {
  if (!date || dayBusy.value) return
  dayBusy.value = true
  dayError.value = ''
  try {
    const res = await fetch('/api/admin/calendar/day', {
      method: 'PUT',
      headers: {'Content-Type': 'application/json'},
      credentials: 'same-origin',
      body: JSON.stringify({date, schedule}),
    })
    const body = await res.json().catch(() => null)
    if (!res.ok) throw new Error(body?.error ?? `HTTP ${res.status}`)
    show(body.calendar as CalendarView)
    if (date === dayDate.value) day.value = body.day as CalendarDay
    emit('saved')
  } catch (err) {
    dayError.value = (err as Error).message
  } finally {
    dayBusy.value = false
  }
}

function changeDay(date: string) {
  dayDate.value = date
  void loadDay()
}

function addBreak() {
  draft.value?.breaks.push({name: '', start: '', end: ''})
}

function removeBreak(i: number) {
  draft.value?.breaks.splice(i, 1)
}

const todayLine = computed(() => {
  const t = view.value?.today
  if (!t) return ''
  const what = t.holiday ? `a public holiday (${t.holiday})` : dayLabels[t.profile_day] ?? t.profile_day
  return `Today is ${what}, so buses run the ${timetableLabels[t.schedule_day] ?? t.schedule_day}.`
})

const dayChoice = computed(() => day.value?.override?.source === 'admin' ? day.value.override.schedule : '')

const dayLine = computed(() => {
  const d = day.value
  if (!d) return ''
  const name = d.holiday ? `${dayName(d.date)}, ${d.holiday},` : dayName(d.date)
  const runs = timetableLabels[d.schedule_day] ?? d.schedule_day
  if (d.override?.source === 'admin') {
    return `${name} runs the ${runs}, set by you. Automatic would be the ${timetableLabels[d.automatic] ?? d.automatic}.`
  }
  if (d.override) return `${name} runs the ${runs}, as seen running that day.`
  return `${name} runs the ${runs}, chosen automatically.`
})

const holidays = computed(() => {
  const all = view.value?.holidays ?? []
  return showAllHolidays.value ? all : all.slice(0, 5)
})

onMounted(load)
</script>

<template>
  <div class="calendar">
    <p v-if="loadError" class="text bad">Could not load the calendar ({{ loadError }}).</p>
    <template v-else-if="view">
      <p class="text">{{ todayLine }}</p>
      <p v-if="!view.editable" class="text bad">No archive database, so the calendar can't be saved.</p>

      <h3>Timetable for a day</h3>
      <div class="row">
        <input v-model="dayDate" type="date" aria-label="Day" @change="loadDay">
        <div class="choices" role="radiogroup" aria-label="Timetable to run">
          <button
            v-for="c in dayChoices"
            :key="c.value"
            type="button"
            role="radio"
            class="choice"
            :class="{ 'is-selected': !!day && dayChoice === c.value }"
            :aria-checked="!!day && dayChoice === c.value"
            :disabled="dayBusy || !view.editable || !day"
            @click="setDay(c.value)"
          >
            {{ c.label }}
          </button>
        </div>
      </div>
      <p v-if="dayError" class="text bad">{{ dayError }}</p>
      <p v-else-if="day" class="text note">{{ dayLine }}</p>

      <h3>School years</h3>
      <div class="years" role="tablist">
        <button
          v-for="y in view.years"
          :key="y.year"
          type="button"
          role="tab"
          class="year"
          :class="{ 'is-selected': y.year === selectedYear, 'is-estimate': y.status === 'estimate' }"
          :aria-selected="y.year === selectedYear"
          @click="pick(y.year)"
        >
          {{ yearLabel(y.year) }} <span>{{ statusLabels[y.status] }}</span>
        </button>
      </div>

      <form v-if="draft" class="year-form" @submit.prevent="save">
        <p v-if="draft.status === 'estimate'" class="text note">
          Estimated from the year before. Check it against the ministry's school year and the Cluj ski week, then save.
        </p>
        <div class="row">
          <span class="row-name">Term</span>
          <input v-model="draft.start" type="date" aria-label="First day of classes" required>
          <span class="to">to</span>
          <input v-model="draft.end" type="date" aria-label="Last day of classes" required>
        </div>
        <div v-for="(b, i) in draft.breaks" :key="i" class="row">
          <input v-model="b.name" class="row-name" type="text" maxlength="40" placeholder="Break" aria-label="Break name" required>
          <input v-model="b.start" type="date" :aria-label="`${b.name || 'Break'} starts`" required>
          <span class="to">to</span>
          <input v-model="b.end" type="date" :aria-label="`${b.name || 'Break'} ends`" required>
          <button type="button" class="link" :aria-label="`Remove ${b.name || 'break'}`" @click="removeBreak(i)">Remove</button>
        </div>
        <div class="actions">
          <button type="button" class="link" @click="addBreak">Add a break</button>
          <button type="submit" class="save" :disabled="saving || !view.editable">
            {{ saving ? 'Saving…' : `Save ${yearLabel(draft.year)}` }}
          </button>
        </div>
        <p v-if="saveMessage" class="text" :class="{bad: saveFailed}">{{ saveMessage }}</p>
      </form>

      <h3>Public holidays</h3>
      <ul class="list">
        <li v-for="h in holidays" :key="h.date">
          <span>{{ dayName(h.date) }}</span>
          <span class="name">{{ h.name }}</span>
          <span class="muted">{{ timetableLabels[h.schedule] ?? h.schedule }}</span>
          <button type="button" class="link" @click="changeDay(h.date)">Change</button>
        </li>
      </ul>
      <button v-if="view.holidays.length > 5" type="button" class="link" @click="showAllHolidays = !showAllHolidays">
        {{ showAllHolidays ? 'Show fewer' : `Show all ${view.holidays.length}` }}
      </button>

      <h3>Days with their own timetable</h3>
      <ul v-if="view.overrides.length" class="list">
        <li v-for="o in view.overrides" :key="o.date">
          <span>{{ dayName(o.date) }}</span>
          <span class="name">{{ timetableLabels[o.schedule] ?? o.schedule }}</span>
          <span class="muted">{{ o.source === 'admin' ? 'set by you' : 'seen running' }}</span>
          <button type="button" class="link" :disabled="dayBusy || !view.editable" @click="setDay('', o.date)">Undo</button>
          <span v-if="o.source !== 'admin'" class="muted detail">{{ o.detail }}</span>
        </li>
      </ul>
      <p v-else class="text">No day has its own timetable, set by you or seen running.</p>
    </template>
  </div>
</template>

<style scoped>
.text {
  margin: 0 0 0.5rem;
  color: var(--a-muted);
}

.note {
  font-size: 0.8125rem;
}

.bad {
  color: var(--a-bad);
}

h3 {
  margin: 1rem 0 0.5rem;
  font-size: 0.8125rem;
  font-weight: 600;
}

.years {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
  margin-bottom: 0.75rem;
}

.year {
  font: inherit;
  font-size: 0.8125rem;
  font-weight: 600;
  padding: 0.25rem 0.625rem;
  border: 1px solid var(--a-rule);
  border-radius: 999px;
  background: none;
  color: var(--a-text);
  cursor: pointer;
  font-variant-numeric: tabular-nums;
}

.year span {
  font-weight: 400;
  color: var(--a-muted);
}

.year.is-estimate {
  border-style: dashed;
}

.year.is-selected {
  background: var(--a-text);
  border-color: var(--a-text);
  color: var(--a-bg);
}

.year.is-selected span {
  color: inherit;
  opacity: 0.75;
}

.choices {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.25rem;
}

.choice {
  font: inherit;
  font-size: 0.8125rem;
  font-weight: 600;
  padding: 0.25rem 0.625rem;
  border: 1px solid var(--a-rule);
  border-radius: 999px;
  background: none;
  color: var(--a-text);
  cursor: pointer;
}

.choice.is-selected {
  background: var(--a-text);
  border-color: var(--a-text);
  color: var(--a-bg);
}

.choice:disabled {
  cursor: default;
  opacity: 0.6;
}

.choice.is-selected:disabled {
  opacity: 1;
}

.year-form {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem;
}

.row-name {
  width: 7rem;
  font-weight: 600;
  font-size: 0.8125rem;
}

input {
  font: inherit;
  font-size: 0.8125rem;
  padding: 0.25rem 0.5rem;
  border: 1px solid var(--a-rule);
  border-radius: 0.5rem;
  background: transparent;
  color: inherit;
  min-width: 0;
}

input:focus {
  outline: 2px solid var(--a-bar-strong);
  outline-offset: 1px;
}

.to {
  color: var(--a-muted);
  font-size: 0.8125rem;
}

.actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  margin-top: 0.25rem;
}

.link {
  font: inherit;
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0.25rem 0.5rem;
  border: 0;
  border-radius: 0.75rem;
  background: none;
  color: var(--a-muted);
  cursor: pointer;
}

.link:hover {
  background: var(--a-hover);
  color: var(--a-text);
}

.save {
  font: inherit;
  font-size: 0.8125rem;
  font-weight: 600;
  padding: 0.375rem 0.875rem;
  border: 0;
  border-radius: 0.75rem;
  background: var(--a-text);
  color: var(--a-bg);
  cursor: pointer;
}

.save:disabled {
  opacity: 0.5;
  cursor: default;
}

.list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.list li {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem 0.75rem;
  padding: 0.3125rem 0;
  border-bottom: 1px solid var(--a-hair);
  font-size: 0.8125rem;
  font-variant-numeric: tabular-nums;
}

.list li:last-child {
  border-bottom: 0;
}

.list .name {
  flex: 1;
  min-width: 8rem;
}

.muted {
  color: var(--a-muted);
}

.detail {
  flex-basis: 100%;
  font-size: 0.75rem;
}
</style>
