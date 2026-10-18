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

type DayOverride = {date: string; schedule: string; source: string}

type CalendarDay = {
  date: string
  schedule_day: string
  automatic: string
  holiday?: string
  override?: DayOverride
}

type LearningWindow = {
  day_type: string
  days: number
  from?: string
  last_year_days: number
}

type CalendarView = {
  today: {date: string; schedule_day: string; holiday?: string}
  editable: boolean
  needed?: number
  years: SchoolYear[]
  holidays: {date: string; name: string; schedule: string}[]
  overrides: DayOverride[]
  learning: {windows: LearningWindow[]}
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
const dayPicker = ref<HTMLInputElement | null>(null)

const dayChoices = [
  {value: '', label: 'Auto'},
  {value: 'weekday', label: 'Weekday'},
  {value: 'saturday', label: 'Saturday'},
  {value: 'sunday', label: 'Sunday'},
]

const timetableLabels: Record<string, string> = {
  weekday: 'Weekday',
  saturday: 'Saturday',
  sunday: 'Sunday',
}

const windowLabels: Record<string, string> = {
  weekday: 'School days',
  weekday_vacation: 'School holidays',
  saturday: 'Saturdays',
  sunday: 'Sundays',
}

const statusLabels: Record<SchoolYear['status'], string> = {
  saved: 'saved',
  built_in: 'built in',
  estimate: 'estimate',
}

function dayName(date: string) {
  return new Date(`${date}T12:00:00`).toLocaleDateString('en-GB', {weekday: 'short', day: 'numeric', month: 'short'})
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
    saveMessage.value = 'Saved'
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
  dayPicker.value?.scrollIntoView({block: 'center', behavior: 'smooth'})
  void loadDay()
}

function addBreak() {
  draft.value?.breaks.push({name: '', start: '', end: ''})
}

function removeBreak(i: number) {
  draft.value?.breaks.splice(i, 1)
}

const todayText = computed(() => {
  const t = view.value?.today
  if (!t) return ''
  const runs = timetableLabels[t.schedule_day] ?? t.schedule_day
  return t.holiday ? `${runs}, ${t.holiday}` : runs
})

const dayChoice = computed(() => day.value?.override?.source === 'admin' ? day.value.override.schedule : '')

const dayText = computed(() => {
  const d = day.value
  if (!d) return ''
  const runs = timetableLabels[d.schedule_day] ?? d.schedule_day
  const name = d.holiday ? `${d.holiday}: ${runs}` : runs
  if (d.override?.source === 'admin') return `${name}, set by you (auto ${timetableLabels[d.automatic] ?? d.automatic})`
  if (d.override) return `${name}, seen running`
  return `${name}, automatic`
})

const holidays = computed(() => {
  const all = view.value?.holidays ?? []
  return showAllHolidays.value ? all : all.slice(0, 5)
})

onMounted(load)
</script>

<template>
  <div class="calendar">
    <p v-if="loadError" class="bad">Could not load ({{ loadError }})</p>
    <template v-else-if="view">
      <dl class="kv">
        <div><dt>Today</dt><dd>{{ todayText }}</dd></div>
        <div v-if="!view.editable"><dt>Saving</dt><dd class="bad">off, no archive</dd></div>
      </dl>

      <h3>Timetable for a day</h3>
      <input ref="dayPicker" v-model="dayDate" class="date" type="date" aria-label="Day" @change="loadDay">
      <div class="segmented" role="radiogroup" aria-label="Timetable to run">
        <button
          v-for="c in dayChoices"
          :key="c.value"
          type="button"
          role="radio"
          :class="{ 'is-selected': !!day && dayChoice === c.value }"
          :aria-checked="!!day && dayChoice === c.value"
          :disabled="dayBusy || !view.editable || !day"
          @click="setDay(c.value)"
        >
          {{ c.label }}
        </button>
      </div>
      <p v-if="dayError" class="hint bad">{{ dayError }}</p>
      <p v-else-if="day" class="hint">{{ dayText }}</p>

      <details class="school" :open="!!view.needed">
        <summary>School years</summary>
        <div class="segmented years" role="tablist">
          <button
            v-for="y in view.years"
            :key="y.year"
            type="button"
            role="tab"
            :class="{ 'is-selected': y.year === selectedYear }"
            :aria-selected="y.year === selectedYear"
            @click="pick(y.year)"
          >
            {{ yearLabel(y.year) }}<small>{{ statusLabels[y.status] }}</small>
          </button>
        </div>

        <form v-if="draft" class="year-form" @submit.prevent="save">
          <div class="range">
            <span class="range-name">Term</span>
            <input v-model="draft.start" type="date" aria-label="First day of classes" required>
            <input v-model="draft.end" type="date" aria-label="Last day of classes" required>
          </div>
          <div v-for="(b, i) in draft.breaks" :key="i" class="range">
            <span class="range-name">
              <input v-model="b.name" type="text" maxlength="40" placeholder="Break" aria-label="Break name" required>
              <button type="button" class="link" :aria-label="`Remove ${b.name || 'break'}`" @click="removeBreak(i)">Remove</button>
            </span>
            <input v-model="b.start" type="date" :aria-label="`${b.name || 'Break'} starts`" required>
            <input v-model="b.end" type="date" :aria-label="`${b.name || 'Break'} ends`" required>
          </div>
          <div class="actions">
            <button type="button" class="link" @click="addBreak">Add break</button>
            <span v-if="saveMessage" :class="{bad: saveFailed}">{{ saveMessage }}</span>
            <button type="submit" class="save" :disabled="saving || !view.editable">
              {{ saving ? 'Saving…' : 'Save' }}
            </button>
          </div>
        </form>
      </details>

      <h3>Learns from</h3>
      <dl class="kv">
        <div v-for="w in view.learning.windows" :key="w.day_type">
          <dt>{{ windowLabels[w.day_type] ?? w.day_type }}</dt>
          <dd>
            {{ w.days }} days<template v-if="w.from"> since {{ dayName(w.from) }}</template><template v-if="w.last_year_days"> + {{ w.last_year_days }} last year</template>
          </dd>
        </div>
      </dl>

      <h3>Public holidays</h3>
      <dl class="kv">
        <div v-for="h in holidays" :key="h.date">
          <dt class="name">{{ dayName(h.date) }} · {{ h.name }}</dt>
          <dd><button type="button" class="link" @click="changeDay(h.date)">{{ timetableLabels[h.schedule] ?? h.schedule }}</button></dd>
        </div>
      </dl>
      <button v-if="view.holidays.length > 5" type="button" class="link" @click="showAllHolidays = !showAllHolidays">
        {{ showAllHolidays ? 'Fewer' : `All ${view.holidays.length}` }}
      </button>

      <template v-if="view.overrides.length">
        <h3>Changed days</h3>
        <dl class="kv">
          <div v-for="o in view.overrides" :key="o.date">
            <dt class="name">{{ dayName(o.date) }} · {{ timetableLabels[o.schedule] ?? o.schedule }}, {{ o.source === 'admin' ? 'by you' : 'seen' }}</dt>
            <dd><button type="button" class="link" :disabled="dayBusy || !view.editable" @click="setDay('', o.date)">Undo</button></dd>
          </div>
        </dl>
      </template>
    </template>
  </div>
</template>

<style scoped>
.bad {
  color: var(--a-bad);
}

h3 {
  margin: 1rem 0 0.375rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--a-muted);
}

.kv {
  margin: 0;
}

.kv > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  min-height: 2.25rem;
  padding: 0.25rem 0;
  border-bottom: 1px solid var(--a-hair);
}

.kv > div:last-child {
  border-bottom: 0;
}

.kv dt {
  flex-shrink: 0;
  color: var(--a-muted);
}

.kv dt.name {
  flex-shrink: 1;
  min-width: 0;
  color: var(--a-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.kv dd {
  margin: 0;
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.hint {
  margin: 0.375rem 0 0;
  font-size: 0.8125rem;
  color: var(--a-muted);
}

input {
  font: inherit;
  font-size: 1rem;
  min-width: 0;
  min-height: 2.5rem;
  padding: 0.375rem 0.625rem;
  border: 1px solid var(--a-rule);
  border-radius: 0.75rem;
  background: transparent;
  color: inherit;
}

input:focus {
  outline: 2px solid var(--a-bar-strong);
  outline-offset: 1px;
}

.date {
  width: 100%;
  margin-bottom: 0.5rem;
}

.segmented {
  display: flex;
  gap: 2px;
  padding: 2px;
  border-radius: 0.75rem;
  background: var(--a-hair);
}

.segmented button {
  flex: 1;
  min-width: 0;
  min-height: 2.25rem;
  padding: 0.25rem 0.375rem;
  border: 0;
  border-radius: 0.625rem;
  background: none;
  color: var(--a-muted);
  font: inherit;
  font-size: 0.8125rem;
  font-weight: 600;
  cursor: pointer;
  font-variant-numeric: tabular-nums;
}

.segmented button.is-selected {
  background: var(--a-bg);
  color: var(--a-text);
  box-shadow: 0 1px 2px rgb(0 0 0 / 0.12);
}

.segmented button:disabled {
  cursor: default;
}

.segmented button:disabled:not(.is-selected) {
  opacity: 0.6;
}

.years button {
  display: flex;
  flex-direction: column;
  align-items: center;
  line-height: 1.2;
}

.years small {
  font-size: 0.6875rem;
  font-weight: 400;
}

.school {
  margin-top: 0.75rem;
}

.school summary {
  padding: 0.4375rem 0;
  font-size: 0.8125rem;
  font-weight: 600;
  color: var(--a-muted);
  cursor: pointer;
}

.school[open] summary {
  margin-bottom: 0.375rem;
}

.year-form {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin-top: 0.75rem;
}

.range {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 0.375rem;
}

.range-name {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  gap: 0.375rem;
  font-size: 0.8125rem;
  font-weight: 600;
}

.range-name input {
  flex: 1;
}

.actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  font-size: 0.8125rem;
}

.link {
  font: inherit;
  font-size: 0.8125rem;
  font-weight: 600;
  min-height: 2.25rem;
  padding: 0 0.5rem;
  border: 0;
  border-radius: 0.75rem;
  background: none;
  color: var(--a-muted);
  cursor: pointer;
}

.link:hover:not(:disabled) {
  background: var(--a-hover);
  color: var(--a-text);
}

.link:disabled {
  cursor: default;
  opacity: 0.6;
}

.kv .link {
  margin-right: -0.5rem;
}

.save {
  font: inherit;
  font-size: 0.875rem;
  font-weight: 600;
  min-height: 2.5rem;
  padding: 0 1.25rem;
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
</style>
