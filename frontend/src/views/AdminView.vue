<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, ref} from 'vue'
import {useHead} from '@unhead/vue'
import AdminDayBars, {type DayBar} from '@/components/admin/AdminDayBars.vue'
import AdminCalendar from '@/components/admin/AdminCalendar.vue'

useHead(() => ({
  title: 'Admin',
  meta: [{name: 'robots', content: 'noindex,nofollow'}],
}))

type Named = {id: number; name: string; color?: string; count: number}

type Accuracy = {
  label: string
  n: number
  within_1: number
  median_abs_sec: number
  p90_abs_sec: number
  bias_sec: number
}

type Overview = {
  generated_at: string
  problems: string[]
  now: {
    streams: number
    people: number
    buses_tracked: number
    buses_in_service: number
    lines_live: number
    feed_at: string | null
    fix_age_sec: number
    poll_sec: number
    polled_by: '' | 'viewers' | 'learner'
    feed_error?: string
    stops: Named[]
    lines: Named[]
  }
  accuracy: {
    today: Accuracy[]
    week: Accuracy[]
    kinds: Accuracy[]
    daily: (Accuracy & {date: string})[]
    timetable?: {today: Accuracy[]; week: Accuracy[]}
  }
  people: {
    today: number
    new: number
    last_week: number
    platforms: {platform: string; today: number; last_week: number}[]
    daily: {date: string; total: number; web: number; android: number; wear: number; other: number}[]
    versions: {platform: string; version: string; today: number; last_seen: string}[]
    stops: Named[]
    lines: Named[]
    plans: number
    places: number
    streams: number
    installs: number
    push_followers: number
    reminders: number
    reminder_users: number
  }
  quota: {
    vehicles: {used: number; limit: number}
    default: {used: number; limit: number}
    learning: {enabled: boolean; dedicated: boolean; used: number; budget: number}
    daily: {date: string; vehicles: number; default: number}[]
  }
  data: {
    warmup_at: string | null
    warmup_sec: number
    warmup_next: string
    warmup_running: boolean
    network_built_at?: string
    route_changes: {line: string; color: string; detected_at: number; kinds: string[]}[]
    samples_today: number
    samples_hour: number
    profiles: number
    stands_today: number
    delays_at: string | null
    archive: {
      enabled: boolean
      ran_at: string | null
      took_sec: number
      copied: number
      versions: number
      error?: string
      bytes: number
      samples: number
      oldest_date?: string
    }
  }
  server: {
    started_at: string
    revision: string
    built_at: string
    modified: boolean
    go_version: string
    heap_bytes: number
    sys_bytes: number
    goroutines: number
    db_bytes: number
    disk_free: number
    disk_total: number
    requests: number
    errors: number
    endpoints: {key: string; calls: number; avg_ms: number; errors: number}[]
    problems: {first: string; last: string; count: number; text: string}[]
  }
}

const REFRESH_MS = 30_000
const AUTH_FLAG_KEY = 'admin:authed'

const readAuthFlag = () => {
  try { return localStorage.getItem(AUTH_FLAG_KEY) === '1' } catch { return false }
}
const writeAuthFlag = (on: boolean) => {
  try {
    if (on) localStorage.setItem(AUTH_FLAG_KEY, '1')
    else localStorage.removeItem(AUTH_FLAG_KEY)
  } catch { /* storage blocked */ }
}

const state = ref<'login' | 'loading' | 'ready'>(readAuthFlag() ? 'loading' : 'login')
const token = ref('')
const loginError = ref('')
const loadError = ref('')
const data = ref<Overview | null>(null)
const loading = ref(false)
const clock = ref(Date.now())
let refreshTimer: ReturnType<typeof setInterval> | null = null
let clockTimer: ReturnType<typeof setInterval> | null = null

async function load() {
  if (loading.value) return
  loading.value = true
  try {
    const res = await fetch('/api/admin/overview', {credentials: 'same-origin'})
    if (res.status === 401) {
      writeAuthFlag(false)
      data.value = null
      state.value = 'login'
      return
    }
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    data.value = await res.json()
    loadError.value = ''
    state.value = 'ready'
  } catch (err) {
    loadError.value = (err as Error).message
    if (!data.value) state.value = 'ready'
  } finally {
    loading.value = false
  }
}

async function login() {
  const t = token.value.trim()
  if (!t) return
  loginError.value = ''
  const res = await fetch('/api/admin/login', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    credentials: 'same-origin',
    body: JSON.stringify({token: t}),
  }).catch(() => null)
  if (!res || !res.ok) {
    loginError.value = res?.status === 401 ? 'Wrong token' : `Could not log in${res ? ` (HTTP ${res.status})` : ''}`
    return
  }
  token.value = ''
  writeAuthFlag(true)
  state.value = 'loading'
  await load()
}

async function logout() {
  await fetch('/api/admin/logout', {method: 'POST', credentials: 'same-origin'}).catch(() => {})
  writeAuthFlag(false)
  data.value = null
  state.value = 'login'
}

function onVisibility() {
  if (document.visibilityState === 'visible' && state.value === 'ready') void load()
}

onMounted(() => {
  if (state.value === 'loading') void load()
  refreshTimer = setInterval(() => {
    if (document.visibilityState === 'visible' && state.value === 'ready') void load()
  }, REFRESH_MS)
  clockTimer = setInterval(() => { clock.value = Date.now() }, 1000)
  document.addEventListener('visibilitychange', onVisibility)
})

onBeforeUnmount(() => {
  if (refreshTimer) clearInterval(refreshTimer)
  if (clockTimer) clearInterval(clockTimer)
  document.removeEventListener('visibilitychange', onVisibility)
})

const num = (n: number) => n.toLocaleString('en-GB')
const pct = (share: number) => `${Math.round(share * 100)}%`

function dur(sec: number) {
  sec = Math.round(Math.abs(sec))
  if (sec < 60) return `${sec} s`
  const min = Math.floor(sec / 60)
  if (min < 60) return sec % 60 && min < 10 ? `${min} min ${sec % 60} s` : `${min} min`
  const h = Math.floor(min / 60)
  if (h < 48) return min % 60 ? `${h} h ${min % 60} min` : `${h} h`
  return `${Math.floor(h / 24)} days`
}

function ago(iso: string | number | null | undefined) {
  if (iso == null || iso === '') return 'never'
  const t = typeof iso === 'number' ? iso : new Date(iso).getTime()
  if (!Number.isFinite(t)) return 'never'
  const sec = (clock.value - t) / 1000
  if (sec < 5) return 'just now'
  return sec > 0 ? `${dur(sec)} ago` : `in ${dur(-sec)}`
}

function bytes(n: number) {
  if (!n) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(units.length - 1, Math.floor(Math.log(n) / Math.log(1024)))
  const v = n / 1024 ** i
  return `${v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`
}

function clockTime(iso: string | null | undefined) {
  if (!iso) return ''
  return new Date(iso).toLocaleTimeString('en-GB', {hour: '2-digit', minute: '2-digit'})
}

function dayName(date: string) {
  return new Date(`${date}T12:00:00`).toLocaleDateString('en-GB', {weekday: 'short', day: 'numeric', month: 'short'})
}

function came(sec: number) {
  if (Math.abs(sec) < 15) return 'on time'
  return sec > 0 ? `${dur(sec)} late` : `${dur(sec)} early`
}

const horizonLabels: Record<string, string> = {
  '1-2': '1–2 min',
  '3-5': '3–5 min',
  '6-10': '6–10 min',
  '11-20': '11–20 min',
  '21-30': '21–30 min',
}

const kindLabels: Record<string, string> = {
  on_trip: 'Bus on its trip',
  standing: 'Bus waiting at the terminus',
  feeder: 'Bus still on its way back',
}

const changeLabels: Record<string, string> = {
  later: 'later',
  earlier: 'earlier',
  trips_added: 'trips added',
  trips_removed: 'trips removed',
  stops_added: 'stops added',
  stops_removed: 'stops removed',
}

const platformLabels: Record<string, string> = {
  web: 'Web',
  android: 'Android',
  wear: 'Watch',
  other: 'Other',
}

const updatedAt = computed(() => data.value
  ? new Date(data.value.generated_at).toLocaleTimeString('en-GB')
  : '')

const accuracyAll = computed(() => data.value?.accuracy.today.find(a => a.label === 'all'))
const accuracyWeekAll = computed(() => data.value?.accuracy.week.find(a => a.label === 'all'))

const liveRows = computed(() => (data.value?.accuracy.week ?? []).filter(a => a.label !== 'all'))

const timetableLabels: Record<string, string> = {
  ours: 'Ours',
  ctp: 'CTP alone',
}

const timetableRows = computed(() => data.value?.accuracy.timetable?.week ?? [])
const timetableToday = (label: string) => data.value?.accuracy.timetable?.today.find(a => a.label === label)
const timetableWeekN = computed(() => timetableRows.value.find(a => a.label === 'ours')?.n ?? 0)

const summary = computed(() => {
  const d = data.value
  if (!d) return ''
  const parts = [
    `${num(d.now.people)} ${d.now.people === 1 ? 'person' : 'people'} watching`,
    `${num(d.now.buses_tracked)} buses tracked`,
  ]
  if (accuracyAll.value?.n) parts.push(`${pct(accuracyAll.value.within_1)} of live times within a minute`)
  return parts.join(' · ')
})

const pollText = computed(() => {
  const n = data.value?.now
  if (!n || !n.poll_sec) return 'not polling, nobody is watching'
  return n.polled_by === 'viewers'
    ? `polled every ${dur(n.poll_sec)} for the people watching`
    : `polled every ${dur(n.poll_sec)} by the learner`
})

const peopleBars = computed<DayBar[]>(() => (data.value?.people.daily ?? []).map(d => {
  const split = (['web', 'android', 'wear', 'other'] as const)
    .filter(p => d[p] > 0)
    .map(p => `${platformLabels[p]} ${num(d[p])}`)
  return {
    date: d.date,
    value: d.total,
    caption: `${dayName(d.date)} · ${num(d.total)} ${d.total === 1 ? 'person' : 'people'}${split.length ? ` (${split.join(', ')})` : ''}`,
  }
}))

const accuracyBars = computed<DayBar[]>(() => (data.value?.accuracy.daily ?? []).map(d => ({
  date: d.date,
  value: d.n ? d.within_1 : 0,
  caption: d.n
    ? `${dayName(d.date)} · ${pct(d.within_1)} within a minute, typical miss ${dur(d.median_abs_sec)} (${num(d.n)} arrivals)`
    : `${dayName(d.date)} · no arrivals scored`,
})))

const quotaBars = computed<DayBar[]>(() => (data.value?.quota.daily ?? []).map(d => ({
  date: d.date,
  value: d.vehicles + d.default,
  caption: `${dayName(d.date)} · ${num(d.vehicles)} vehicle calls, ${num(d.default)} other`,
})))

const versionsByPlatform = computed(() => {
  const out: {platform: string; versions: Overview['people']['versions']}[] = []
  for (const v of data.value?.people.versions ?? []) {
    let group = out.find(g => g.platform === v.platform)
    if (!group) out.push(group = {platform: v.platform, versions: []})
    group.versions.push(v)
  }
  return out
})

const uptime = computed(() => data.value ? dur((clock.value - new Date(data.value.server.started_at).getTime()) / 1000) : '')

const diskUsed = computed(() => {
  const s = data.value?.server
  if (!s || !s.disk_total) return null
  return 1 - s.disk_free / s.disk_total
})

function quotaShare(used: number, limit: number) {
  return limit > 0 ? Math.min(1, used / limit) : 0
}
</script>

<template>
  <Teleport to="body">
    <div class="admin">
      <form v-if="state === 'login'" class="login" @submit.prevent="login">
        <h1>Admin</h1>
        <input
          type="text"
          name="username"
          value="admin"
          autocomplete="username"
          readonly
          tabindex="-1"
          aria-hidden="true"
          class="hidden-username"
        />
        <input
          v-model="token"
          type="password"
          name="password"
          autocomplete="current-password"
          spellcheck="false"
          placeholder="Token"
          aria-label="Token"
          autofocus
        />
        <button type="submit" class="primary">Unlock</button>
        <p v-if="loginError" class="bad">{{ loginError }}</p>
      </form>

      <div v-else-if="!data" class="page">
        <p class="muted">{{ loadError ? `Could not load: ${loadError}` : 'Loading…' }}</p>
        <button v-if="loadError" type="button" class="text-button" @click="load">Try again</button>
      </div>

      <div v-else class="page">
        <header class="top">
          <span class="title">Conexiuni Cluj</span>
          <span class="top-actions">
            <button type="button" class="text-button" :disabled="loading" @click="load">
              {{ loading ? 'Updating…' : `Updated ${updatedAt}` }}
            </button>
            <button type="button" class="text-button" @click="logout">Log out</button>
          </span>
        </header>
        <p v-if="loadError" class="bad small">Could not refresh ({{ loadError }}), showing {{ updatedAt }}.</p>

        <section class="status">
          <template v-if="data.problems.length">
            <p v-for="p in data.problems" :key="p" class="problem">{{ p }}</p>
          </template>
          <p v-else class="ok">Everything is running.</p>
          <p class="muted">{{ summary }}</p>
        </section>

        <div class="columns">
          <section>
            <div class="label"><span>Right now</span><i></i></div>
            <div class="figures">
              <div>
                <b>{{ num(data.now.people) }}</b>
                <span>{{ data.now.people === 1 ? 'person' : 'people' }} watching on {{ num(data.now.streams) }} {{ data.now.streams === 1 ? 'screen' : 'screens' }}</span>
              </div>
              <div>
                <b>{{ num(data.now.buses_tracked) }}</b>
                <span>buses tracked of {{ num(data.now.buses_in_service) }} in service, on {{ num(data.now.lines_live) }} lines</span>
              </div>
            </div>
            <p class="line-text">
              <template v-if="data.now.feed_at">Positions fetched {{ ago(data.now.feed_at) }}, the fixes were {{ dur(data.now.fix_age_sec) }} old;</template>
              <template v-else>No positions fetched yet;</template>
              {{ pollText }}.
            </p>
            <p v-if="data.now.feed_error" class="bad small">Last error: {{ data.now.feed_error }}</p>

            <div v-if="data.now.lines.length || data.now.stops.length" class="pair">
              <div v-if="data.now.lines.length">
                <h3>Lines on screens</h3>
                <ul class="rows">
                  <li v-for="l in data.now.lines" :key="l.id">
                    <span class="badge" :style="{backgroundColor: l.color || '#64748b'}">{{ l.name }}</span>
                    <span class="count">{{ num(l.count) }}</span>
                  </li>
                </ul>
              </div>
              <div v-if="data.now.stops.length">
                <h3>Stops on screens</h3>
                <ul class="rows">
                  <li v-for="s in data.now.stops" :key="s.id">
                    <span class="name">{{ s.name }}</span>
                    <span class="count">{{ num(s.count) }}</span>
                  </li>
                </ul>
              </div>
            </div>
          </section>

          <section>
            <div class="label"><span>Accuracy</span><i></i></div>
            <div class="figures">
              <div>
                <b>{{ accuracyAll?.n ? pct(accuracyAll.within_1) : '–' }}</b>
                <span>{{ accuracyAll?.n ? 'of live times right within a minute today' : 'no live times scored yet today' }}</span>
              </div>
              <div>
                <b>{{ timetableToday('ours')?.n ? pct(timetableToday('ours')!.within_1) : '–' }}</b>
                <span v-if="timetableToday('ours')?.n">
                  of our timetable right within a minute today; CTP alone {{ pct(timetableToday('ctp')?.within_1 ?? 0) }}
                </span>
                <span v-else>timetable not scored yet today</span>
              </div>
            </div>

            <p class="line-text">How close each time was to the bus reaching the stop, last 7 days.</p>
            <table class="table accuracy">
              <thead>
                <tr>
                  <th></th>
                  <th>Right within a minute</th>
                  <th>1 in 10 off by</th>
                  <th>Buses came</th>
                </tr>
              </thead>
              <tbody>
                <tr class="group"><th colspan="4">Live time, when it said</th></tr>
                <tr v-for="r in liveRows" :key="r.label">
                  <td>{{ horizonLabels[r.label] ?? r.label }}</td>
                  <td>{{ r.n ? pct(r.within_1) : '–' }}</td>
                  <td>{{ r.n ? dur(r.p90_abs_sec) : '–' }}</td>
                  <td>{{ r.n ? came(r.bias_sec) : '–' }}</td>
                </tr>
              </tbody>
              <tbody>
                <tr class="group"><th colspan="4">Timetable</th></tr>
                <tr v-for="r in timetableRows" :key="r.label">
                  <td>{{ timetableLabels[r.label] ?? r.label }}</td>
                  <td>{{ r.n ? pct(r.within_1) : '–' }}</td>
                  <td>{{ r.n ? dur(r.p90_abs_sec) : '–' }}</td>
                  <td>{{ r.n ? came(r.bias_sec) : '–' }}</td>
                </tr>
              </tbody>
            </table>
            <p class="note">
              A live time counts once for every minute a screen showed it<template v-if="accuracyWeekAll?.n"> ({{ num(accuracyWeekAll.n) }})</template>;
              the timetable at every stop a bus reached after it was matched to its departure<template v-if="timetableWeekN"> ({{ num(timetableWeekN) }})</template>.
              CTP only prints when buses leave each end, so between stops its time is that departure plus the distance at a flat bus pace,
              where the app starts before it learns anything.
            </p>

            <AdminDayBars :days="accuracyBars" :max="1"/>

            <h3>Live times by where the bus was, 7 days</h3>
            <ul class="rows">
              <li v-for="k in data.accuracy.kinds" :key="k.label">
                <span class="name">{{ kindLabels[k.label] ?? k.label }}</span>
                <span class="count">{{ k.n ? `${pct(k.within_1)} of ${num(k.n)}` : '–' }}</span>
              </li>
            </ul>
          </section>

          <section>
            <div class="label"><span>People</span><i></i></div>
            <div class="figures">
              <div>
                <b>{{ num(data.people.today) }}</b>
                <span>today, {{ num(data.people.new) }} new; {{ num(data.people.last_week) }} a week ago</span>
              </div>
              <div>
                <b>{{ num(data.people.plans) }}</b>
                <span>trips planned, {{ num(data.people.places) }} place searches today</span>
              </div>
            </div>

            <table class="table">
              <thead>
                <tr>
                  <th></th>
                  <th>Today</th>
                  <th>Week ago</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="p in data.people.platforms" :key="p.platform">
                  <td>{{ platformLabels[p.platform] ?? p.platform }}</td>
                  <td>{{ num(p.today) }}</td>
                  <td>{{ num(p.last_week) }}</td>
                </tr>
              </tbody>
            </table>

            <AdminDayBars :days="peopleBars"/>

            <div v-if="data.people.lines.length || data.people.stops.length" class="pair">
              <div v-if="data.people.lines.length">
                <h3>Lines opened today</h3>
                <ul class="rows">
                  <li v-for="l in data.people.lines" :key="l.id">
                    <span class="badge" :style="{backgroundColor: l.color || '#64748b'}">{{ l.name }}</span>
                    <span class="count">{{ num(l.count) }}</span>
                  </li>
                </ul>
              </div>
              <div v-if="data.people.stops.length">
                <h3>Stops opened today</h3>
                <ul class="rows">
                  <li v-for="s in data.people.stops" :key="s.id">
                    <span class="name">{{ s.name }}</span>
                    <span class="count">{{ num(s.count) }}</span>
                  </li>
                </ul>
              </div>
            </div>

            <div v-for="g in versionsByPlatform" :key="g.platform">
              <h3>{{ platformLabels[g.platform] ?? g.platform }} versions</h3>
              <ul class="rows">
                <li v-for="v in g.versions" :key="v.version">
                  <span class="name">{{ v.version }}</span>
                  <span class="count">{{ v.today ? `${num(v.today)} today` : `last seen ${dayName(v.last_seen)}` }}</span>
                </li>
              </ul>
            </div>

            <p class="line-text">
              {{ num(data.people.streams) }} live {{ data.people.streams === 1 ? 'screen' : 'screens' }} opened today.
              {{ num(data.people.push_followers) }} follow line changes,
              {{ num(data.people.reminder_users) }} have {{ num(data.people.reminders) }} reminders.
              The web app was installed {{ num(data.people.installs) }} times.
            </p>
          </section>

          <section>
            <div class="label"><span>Tranzy</span><i></i></div>
            <div class="meter">
              <p><span>Vehicle calls</span><span>{{ num(data.quota.vehicles.used) }} of {{ num(data.quota.vehicles.limit) }}</span></p>
              <div class="track"><span :style="{width: pct(quotaShare(data.quota.vehicles.used, data.quota.vehicles.limit))}"></span></div>
            </div>
            <div class="meter">
              <p><span>Other calls</span><span>{{ num(data.quota.default.used) }} of {{ num(data.quota.default.limit) }}</span></p>
              <div class="track"><span :style="{width: pct(quotaShare(data.quota.default.used, data.quota.default.limit))}"></span></div>
            </div>
            <div v-if="data.quota.learning.enabled" class="meter">
              <p>
                <span>Learner{{ data.quota.learning.dedicated ? ', own key' : '' }}</span>
                <span>{{ num(data.quota.learning.used) }} of {{ num(data.quota.learning.budget) }}</span>
              </p>
              <div class="track"><span :style="{width: pct(quotaShare(data.quota.learning.used, data.quota.learning.budget))}"></span></div>
            </div>
            <p v-else class="line-text">The learner is off.</p>
            <AdminDayBars :days="quotaBars"/>
          </section>

          <section>
            <div class="label"><span>Timetables and learning</span><i></i></div>
            <p class="line-text">
              <template v-if="data.data.warmup_running">Timetables are being refreshed now.</template>
              <template v-else-if="data.data.warmup_at">
                Timetables refreshed {{ ago(data.data.warmup_at) }} in {{ dur(data.data.warmup_sec) }}.
              </template>
              <template v-else>Timetables not refreshed since the start.</template>
              Next at {{ clockTime(data.data.warmup_next) }}.
              <template v-if="data.data.network_built_at">Network bundle built {{ ago(data.data.network_built_at) }}.</template>
            </p>
            <p class="line-text">
              {{ num(data.data.samples_today) }} stop-to-stop times learned today, {{ num(data.data.samples_hour) }} in the last hour;
              {{ num(data.data.profiles) }} profiles. {{ num(data.data.stands_today) }} terminus stands today.
              Delays relearned {{ ago(data.data.delays_at) }}.
            </p>
            <p class="line-text">
              <template v-if="!data.data.archive.enabled">No archive database, so no history is kept.</template>
              <template v-else-if="data.data.archive.ran_at">
                Archive holds {{ num(data.data.archive.samples) }} stop-to-stop times<template v-if="data.data.archive.oldest_date"> since {{ dayName(data.data.archive.oldest_date) }}</template>, {{ bytes(data.data.archive.bytes) }}.
                Last copied {{ ago(data.data.archive.ran_at) }}: {{ num(data.data.archive.copied) }} rows, {{ num(data.data.archive.versions) }} new timetable or stop versions, in {{ dur(data.data.archive.took_sec) }}.
              </template>
              <template v-else>The archive makes its first copy a few minutes after the start.</template>
            </p>

            <h3>Timetable changes, 7 days</h3>
            <ul v-if="data.data.route_changes.length" class="rows">
              <li v-for="c in data.data.route_changes" :key="`${c.line}-${c.detected_at}`">
                <span class="badge" :style="{backgroundColor: c.color || '#64748b'}">{{ c.line }}</span>
                <span class="name">{{ c.kinds.map(k => changeLabels[k] ?? k).join(', ') }}</span>
                <span class="count">{{ ago(c.detected_at) }}</span>
              </li>
            </ul>
            <p v-else class="line-text">None.</p>
          </section>

          <section>
            <div class="label"><span>Calendar</span><i></i></div>
            <AdminCalendar @saved="load"/>
          </section>

          <section>
            <div class="label"><span>Server</span><i></i></div>
            <p class="line-text">
              Up {{ uptime }}<template v-if="data.server.revision">, build {{ data.server.revision }}{{ data.server.modified ? ' with local changes' : '' }}</template><template v-if="data.server.built_at"> from {{ dayName(data.server.built_at.slice(0, 10)) }}</template>, {{ data.server.go_version }}.
              {{ bytes(data.server.heap_bytes) }} in use, {{ bytes(data.server.sys_bytes) }} from the system, {{ num(data.server.goroutines) }} goroutines.
              Database {{ bytes(data.server.db_bytes) }}<template v-if="diskUsed != null">, disk {{ pct(diskUsed) }} full ({{ bytes(data.server.disk_free) }} free)</template>.
            </p>
            <p class="line-text">
              {{ num(data.server.requests) }} requests today,
              <span :class="{bad: data.server.errors > 0}">{{ num(data.server.errors) }} server errors</span>.
            </p>

            <table class="table endpoints">
              <thead>
                <tr>
                  <th>Today</th>
                  <th>Calls</th>
                  <th>Avg</th>
                  <th>Errors</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="e in data.server.endpoints" :key="e.key">
                  <td>{{ e.key }}</td>
                  <td>{{ num(e.calls) }}</td>
                  <td>{{ e.key.endsWith(' stream') ? '–' : `${num(e.avg_ms)} ms` }}</td>
                  <td :class="{bad: e.errors > 0}">{{ e.errors ? num(e.errors) : '–' }}</td>
                </tr>
              </tbody>
            </table>

            <h3>Logged failures, 24 hours</h3>
            <ul v-if="data.server.problems.length" class="log">
              <li v-for="p in data.server.problems" :key="p.text">
                <span class="when">{{ ago(p.last) }}<template v-if="p.count > 1">, {{ num(p.count) }} times since {{ clockTime(p.first) }}</template></span>
                <code>{{ p.text }}</code>
              </li>
            </ul>
            <p v-else class="line-text">None.</p>
          </section>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.admin {
  --a-bg: #ffffff;
  --a-text: #0f172a;
  --a-muted: #64748b;
  --a-faint: #94a3b8;
  --a-rule: #e2e8f0;
  --a-hair: #f1f5f9;
  --a-bar: #cbd5e1;
  --a-bar-strong: #64748b;
  --a-hover: rgb(0 0 0 / 0.05);
  --a-bad: #b91c1c;

  position: fixed;
  inset: 0;
  z-index: 50;
  overflow-y: auto;
  background: var(--a-bg);
  color: var(--a-text);
  color-scheme: light;
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
  font-size: 0.9375rem;
  line-height: 1.45;
  -webkit-text-size-adjust: 100%;
}

@media (prefers-color-scheme: dark) {
  .admin {
    --a-bg: #0f172a;
    --a-text: #f1f5f9;
    --a-muted: #94a3b8;
    --a-faint: #64748b;
    --a-rule: #334155;
    --a-hair: #1e293b;
    --a-bar: #334155;
    --a-bar-strong: #94a3b8;
    --a-hover: rgb(255 255 255 / 0.06);
    --a-bad: #f87171;

    color-scheme: dark;
  }
}

.admin *,
.admin *::before,
.admin *::after {
  box-sizing: border-box;
}

.login {
  width: min(20rem, calc(100% - 2rem));
  margin: 18vh auto 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.login h1 {
  margin: 0 0 0.25rem;
  font-size: 1.25rem;
  font-weight: 700;
}

.login input {
  font: inherit;
  padding: 0.625rem 0.75rem;
  border: 1px solid var(--a-rule);
  border-radius: 0.75rem;
  background: transparent;
  color: inherit;
}

.login input:focus {
  outline: 2px solid var(--a-bar-strong);
  outline-offset: 1px;
}

.hidden-username {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
  pointer-events: none;
}

.primary {
  font: inherit;
  font-weight: 600;
  padding: 0.625rem 0.75rem;
  border: 0;
  border-radius: 0.75rem;
  background: var(--a-text);
  color: var(--a-bg);
  cursor: pointer;
}

.page {
  max-width: 72rem;
  margin: 0 auto;
  padding: max(1rem, env(safe-area-inset-top)) 1rem max(2.5rem, env(safe-area-inset-bottom));
}

.top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.title {
  font-weight: 700;
}

.top-actions {
  display: flex;
  gap: 0.25rem;
}

.text-button {
  font: inherit;
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0.25rem 0.5rem;
  margin: 0 -0.25rem;
  border: 0;
  border-radius: 0.75rem;
  background: none;
  color: var(--a-muted);
  cursor: pointer;
  font-variant-numeric: tabular-nums;
}

.text-button:hover:not(:disabled) {
  background: var(--a-hover);
  color: var(--a-text);
}

.status {
  margin: 1.25rem 0 0;
}

.ok,
.problem {
  margin: 0 0 0.25rem;
  font-size: 1.375rem;
  font-weight: 700;
  line-height: 1.25;
}

.problem {
  color: var(--a-bad);
}

.muted {
  margin: 0.25rem 0 0;
  color: var(--a-muted);
}

.bad {
  color: var(--a-bad);
}

.small {
  font-size: 0.8125rem;
  margin: 0.25rem 0 0;
}

.columns {
  column-gap: 3rem;
}

@media (min-width: 60rem) {
  .columns {
    column-count: 2;
  }
}

section {
  min-width: 0;
  padding-top: 1.5rem;
  break-inside: avoid;
}

.label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin: 0 0 0.75rem;
}

.label span {
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--a-muted);
  white-space: nowrap;
}

.label i {
  flex: 1;
  height: 1px;
  background: var(--a-hair);
}

.figures {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem;
  margin-bottom: 0.75rem;
}

.figures b {
  display: block;
  font-size: 1.875rem;
  font-weight: 700;
  line-height: 1.1;
  font-variant-numeric: tabular-nums;
}

.figures span {
  display: block;
  margin-top: 0.125rem;
  font-size: 0.8125rem;
  color: var(--a-muted);
}

.line-text {
  margin: 0 0 0.75rem;
  font-size: 0.875rem;
  color: var(--a-muted);
}

.note {
  margin: 0.375rem 0 1rem;
  font-size: 0.75rem;
  color: var(--a-faint);
}

h3 {
  margin: 1rem 0 0.375rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--a-muted);
}

.pair {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
  column-gap: 1.5rem;
}

.rows {
  list-style: none;
  margin: 0;
  padding: 0;
}

.rows li {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-height: 1.875rem;
  border-bottom: 1px solid var(--a-hair);
  font-size: 0.875rem;
}

.rows li:last-child {
  border-bottom: 0;
}

.rows .name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rows .badge + .count {
  margin-left: auto;
}

.count {
  color: var(--a-muted);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.badge {
  flex-shrink: 0;
  min-width: 2rem;
  height: 1.375rem;
  padding: 0 0.375rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.5rem;
  color: #ffffff;
  font-size: 0.75rem;
  font-weight: 800;
}

.table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
  font-variant-numeric: tabular-nums;
}

.table th {
  padding: 0 0 0.25rem;
  font-size: 0.6875rem;
  font-weight: 600;
  color: var(--a-faint);
  text-align: right;
}

.table td {
  padding: 0.3125rem 0;
  border-top: 1px solid var(--a-hair);
  text-align: right;
  white-space: nowrap;
}

.table th:first-child,
.table td:first-child {
  text-align: left;
}

.table th + th,
.table td + td {
  padding-left: 0.75rem;
}

.accuracy thead th {
  vertical-align: bottom;
}

.table .group th {
  padding-top: 0.75rem;
  text-align: left;
}

.table tbody:first-of-type .group th {
  padding-top: 0.25rem;
}

.table + :deep(.day-bars),
.note + :deep(.day-bars) {
  margin-top: 1rem;
}

.endpoints {
  margin-bottom: 0.25rem;
}

.endpoints td:first-child {
  max-width: 0;
  width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 0.8125rem;
}

.meter {
  margin-bottom: 0.75rem;
}

.meter p {
  display: flex;
  justify-content: space-between;
  margin: 0 0 0.25rem;
  font-size: 0.875rem;
  font-variant-numeric: tabular-nums;
}

.meter p span:last-child {
  color: var(--a-muted);
}

.track {
  height: 0.375rem;
  border-radius: 999px;
  background: var(--a-hair);
  overflow: hidden;
}

.track span {
  display: block;
  height: 100%;
  background: var(--a-bar-strong);
}

.meter + :deep(.day-bars),
.line-text + :deep(.day-bars) {
  margin-top: 1rem;
}

.log {
  list-style: none;
  margin: 0;
  padding: 0;
}

.log li {
  padding: 0.375rem 0;
  border-bottom: 1px solid var(--a-hair);
}

.log li:last-child {
  border-bottom: 0;
}

.when {
  display: block;
  font-size: 0.75rem;
  color: var(--a-muted);
}

.log code {
  display: block;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 0.75rem;
  overflow-wrap: anywhere;
}
</style>
