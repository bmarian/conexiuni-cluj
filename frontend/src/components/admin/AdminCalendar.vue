<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import ChoiceRow from '@/components/ui/ChoiceRow.vue'
import SubLabel from '@/components/ui/SubLabel.vue'
import type {Choice} from '@/components/ui/types.ts'
import type {LearningWindow} from '@/components/admin/types.ts'
import {dayName, timetableLabels} from '@/utils/adminFormat.ts'
import {AddIcon, CloseIcon, ExpandMoreIcon} from '@/utils/icons.ts'

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

type CalendarView = {
  today: {date: string; schedule_day: string; holiday?: string}
  editable: boolean
  needed?: number
  years: SchoolYear[]
  holidays: {date: string; name: string; schedule: string}[]
  overrides: DayOverride[]
  learning: {windows: LearningWindow[]}
}

const props = defineProps<{city: string}>()
const emit = defineEmits<{saved: [], learning: [windows: LearningWindow[]]}>()

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

const statusLabels: Record<SchoolYear['status'], string> = {
  saved: 'saved',
  built_in: 'built in',
  estimate: 'estimate',
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
    const res = await fetch(`/api/admin/calendar?city=${props.city}`, {credentials: 'same-origin'})
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
  emit('learning', next.learning.windows)
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
    const res = await fetch(`/api/admin/calendar?city=${props.city}`, {
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
    const res = await fetch(`/api/admin/calendar/day?city=${props.city}&date=${encodeURIComponent(dayDate.value)}`, {credentials: 'same-origin'})
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
    const res = await fetch(`/api/admin/calendar/day?city=${props.city}`, {
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

const dayChoices = computed<Choice<string>[]>(() => {
  const disabled = dayBusy.value || !view.value?.editable || !day.value
  return [{value: '', label: 'Auto', disabled}, ...Object.entries(timetableLabels).map(([value, label]) => ({value, label, disabled}))]
})

function chooseDay(schedule: string) {
  if (schedule !== dayChoice.value) void setDay(schedule)
}

const yearChoices = computed<Choice<string>[]>(() => (view.value?.years ?? []).map((y) => ({value: String(y.year), label: yearLabel(y.year)})))

function yearStatus(value: string) {
  const y = view.value?.years.find((y) => String(y.year) === value)
  return y ? statusLabels[y.status] : ''
}

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
    <p v-if="loadError" class="admin-note admin-bad">Could not load ({{ loadError }})</p>
    <template v-else-if="view">
      <dl class="admin-rows">
        <div><dt>Today</dt><dd>{{ todayText }}</dd></div>
        <div v-if="!view.editable"><dt>Saving</dt><dd class="admin-bad">off, no archive</dd></div>
      </dl>

      <SubLabel text="Timetable for a day"/>
      <div class="day">
        <input ref="dayPicker" v-model="dayDate" class="admin-field" type="date" aria-label="Day" @change="loadDay">
        <ChoiceRow :options="dayChoices" :model-value="dayChoice" aria-label="Timetable to run" @update:model-value="chooseDay"/>
        <p v-if="dayError" class="admin-note admin-bad">{{ dayError }}</p>
        <p v-else-if="day" class="admin-note">{{ dayText }}</p>
      </div>

      <details class="admin-fold" :open="!!view.needed">
        <summary class="state"><span>School years</span><ExpandMoreIcon class="m-icon"/></summary>
        <div class="school">
          <ChoiceRow :options="yearChoices" :model-value="String(selectedYear)" class="years" @update:model-value="pick(Number($event))">
            <template #label="{ option }">
              <span class="year">
                <span>{{ option.label }}</span>
                <small>{{ yearStatus(option.value) }}</small>
              </span>
            </template>
          </ChoiceRow>

          <form v-if="draft" class="year-form" @submit.prevent="save">
            <div class="range">
              <span class="range-name text-label-large">Term</span>
              <input v-model="draft.start" class="admin-field" type="date" aria-label="First day of classes" required>
              <input v-model="draft.end" class="admin-field" type="date" aria-label="Last day of classes" required>
            </div>
            <div v-for="(b, i) in draft.breaks" :key="i" class="range">
              <span class="range-name">
                <input v-model="b.name" class="admin-field" type="text" maxlength="40" placeholder="Break" aria-label="Break name" required>
                <button type="button" class="icon-btn" :aria-label="`Remove ${b.name || 'break'}`" :title="`Remove ${b.name || 'break'}`" @click="removeBreak(i)">
                  <CloseIcon class="m-icon"/>
                </button>
              </span>
              <input v-model="b.start" class="admin-field" type="date" :aria-label="`${b.name || 'Break'} starts`" required>
              <input v-model="b.end" class="admin-field" type="date" :aria-label="`${b.name || 'Break'} ends`" required>
            </div>
            <div class="actions">
              <button type="button" class="btn btn-text" @click="addBreak"><AddIcon class="m-icon"/>Add break</button>
              <span class="text-body-small" :class="{ 'admin-bad': saveFailed }">{{ saveMessage }}</span>
              <button type="submit" class="btn btn-filled" :disabled="saving || !view.editable">
                {{ saving ? 'Saving…' : 'Save' }}
              </button>
            </div>
          </form>
        </div>
      </details>

      <SubLabel text="Public holidays"/>
      <dl class="admin-rows">
        <div v-for="h in holidays" :key="h.date" class="has-action">
          <dt class="name">{{ dayName(h.date) }} · {{ h.name }}</dt>
          <dd><button type="button" class="btn btn-text" @click="changeDay(h.date)">{{ timetableLabels[h.schedule] ?? h.schedule }}</button></dd>
        </div>
      </dl>
      <button v-if="view.holidays.length > 5" type="button" class="btn btn-text more" @click="showAllHolidays = !showAllHolidays">
        {{ showAllHolidays ? 'Fewer' : `All ${view.holidays.length}` }}
      </button>

      <template v-if="view.overrides.length">
        <SubLabel text="Changed days"/>
        <dl class="admin-rows">
          <div v-for="o in view.overrides" :key="o.date" class="has-action">
            <dt class="name">{{ dayName(o.date) }} · {{ timetableLabels[o.schedule] ?? o.schedule }}, {{ o.source === 'admin' ? 'by you' : 'seen' }}</dt>
            <dd><button type="button" class="btn btn-text" :disabled="dayBusy || !view.editable" @click="setDay('', o.date)">Undo</button></dd>
          </div>
        </dl>
      </template>
    </template>
  </div>
</template>

<style scoped>
.day {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 4px 16px 8px;
}

.day .admin-note {
  padding: 0 4px;
}

/* Four choices on a phone: room for "Saturday" over the usual padding. */
.day :deep(.choice) {
  padding: 0 4px;
}

.school {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 4px 16px 12px;
}

.years {
  height: 56px;
}

.year {
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 0;
  font-variant-numeric: tabular-nums;
}

.year small {
  font-size: 11px;
  line-height: 14px;
  font-weight: 400;
}

.year-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.range {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 8px;
}

.range-name {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  gap: 4px;
  padding-left: 4px;
  color: var(--md-on-surface-variant);
}

.range-name .admin-field {
  flex: 1;
  margin-left: -4px;
}

.range-name .icon-btn {
  margin-right: -4px;
}

.actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: -12px;
}

.actions > span {
  flex: 1;
  min-width: 0;
}

.more {
  margin-left: 4px;
}
</style>
