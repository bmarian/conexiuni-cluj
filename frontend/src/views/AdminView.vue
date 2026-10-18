<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, ref} from 'vue'
import {useHead} from '@unhead/vue'
import AdminDayBars, {type DayBar} from '@/components/admin/AdminDayBars.vue'
import AdminCalendar from '@/components/admin/AdminCalendar.vue'
import {cities, currentCity} from '@/utils/city.ts'

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
    daily: (Accuracy & {date: string})[]
    timetable: Accuracy[]
  }
  people: {
    today: number
    new: number
    last_week: number
    platforms: {platform: string; today: number; last_week: number}[]
    daily: {date: string; total: number; web: number; android: number; wear: number}[]
    versions: {platform: string; version: string; today: number; last_seen: string}[]
    stops: Named[]
    lines: Named[]
    plans: number
    places: number
    installs: number
    push_followers: number
    reminders: number
    reminder_users: number
    moves: {sent: number; empty: number; received: number; old_push: number}
  }
  quota: {
    quotas: {name: string; used: number; limit: number}[]
    learning: {enabled: boolean; dedicated: boolean; used: number; budget: number}
  }
  data: {
    warmup_at: string | null
    warmup_next: string
    warmup_running: boolean
    route_changes: {line: string; color: string; detected_at: number; kinds: string[]}[]
    samples_today: number
    samples_hour: number
    delays_at: string | null
    archive: {enabled: boolean; ran_at: string | null; error?: string; bytes: number; oldest_date?: string}
  }
  server: {
    started_at: string
    revision: string
    modified: boolean
    heap_bytes: number
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
const adminCity = ref(currentCity)
const cityOpen = ref(false)
const cityName = computed(() => cities.value.find((c) => c.slug === adminCity.value)?.name ?? adminCity.value)

function pickCity(slug: string) {
  cityOpen.value = false
  if (slug === adminCity.value) return
  adminCity.value = slug
  void load()
}

function onDocumentPointerDown(e: PointerEvent) {
  if (!(e.target as HTMLElement).closest('.city-pick')) cityOpen.value = false
}
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
    const res = await fetch(`/api/admin/overview?city=${adminCity.value}`, {credentials: 'same-origin'})
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
  document.addEventListener('pointerdown', onDocumentPointerDown)
})

onBeforeUnmount(() => {
  if (refreshTimer) clearInterval(refreshTimer)
  if (clockTimer) clearInterval(clockTimer)
  document.removeEventListener('visibilitychange', onVisibility)
  document.removeEventListener('pointerdown', onDocumentPointerDown)
})

const num = (n: number) => n.toLocaleString('en-GB')
const pct = (share: number) => `${Math.round(share * 100)}%`

function dur(sec: number) {
  sec = Math.round(Math.abs(sec))
  if (sec < 60) return `${sec} s`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min} min`
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

function minSec(sec: number) {
  sec = Math.round(Math.abs(sec))
  return `${Math.floor(sec / 60)}:${String(sec % 60).padStart(2, '0')}`
}

function came(sec: number) {
  if (Math.abs(sec) < 15) return 'on time'
  return `${minSec(sec)} ${sec > 0 ? 'late' : 'early'}`
}

const horizonLabels: Record<string, string> = {
  'all': 'All',
  '1-2': '1–2 min',
  '3-5': '3–5 min',
  '6-10': '6–10 min',
  '11-20': '11–20 min',
  '21-30': '21–30 min',
}

const timetableLabels: Record<string, string> = {
  ours: 'Ours',
  ctp: 'CTP',
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
}

const updatedAt = computed(() => data.value ? clockTime(data.value.generated_at) : '')

const liveToday = computed(() => data.value?.accuracy.today.find(a => a.label === 'all'))

// "All" first, then by how far ahead the time was.
const liveRows = computed(() => {
  const week = data.value?.accuracy.week ?? []
  return [...week.filter(a => a.label === 'all'), ...week.filter(a => a.label !== 'all')]
})

const pollText = computed(() => {
  const n = data.value?.now
  if (!n || !n.poll_sec) return 'off'
  return n.polled_by === 'viewers' ? `every ${dur(n.poll_sec)}` : `every ${dur(n.poll_sec)}, learner`
})

const peopleBars = computed<DayBar[]>(() => (data.value?.people.daily ?? []).map(d => {
  const split = (['web', 'android', 'wear'] as const)
    .filter(p => d[p] > 0)
    .map(p => `${platformLabels[p]} ${num(d[p])}`)
  return {
    date: d.date,
    value: d.total,
    caption: `${dayName(d.date)} · ${num(d.total)}${split.length ? ` (${split.join(', ')})` : ''}`,
  }
}))

const accuracyBars = computed<DayBar[]>(() => (data.value?.accuracy.daily ?? []).map(d => ({
  date: d.date,
  value: d.n ? d.within_1 : 0,
  caption: d.n ? `${dayName(d.date)} · ${pct(d.within_1)} of ${num(d.n)}` : `${dayName(d.date)} · none`,
})))

const versionsToday = computed(() => (data.value?.people.versions ?? []).filter(v => v.today > 0))

const moves = computed(() => {
  const m = data.value?.people.moves
  if (!m) return ''
  const visits = m.sent + m.empty
  if (!visits && !m.old_push) return ''
  return `${num(visits)} today, ${num(m.old_push)} push left`
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

const meters = computed(() => {
  const q = data.value?.quota
  if (!q) return []
  const out = [
    ...q.quotas.map((quota) => ({label: quota.name, used: quota.used, limit: quota.limit})),
  ]
  if (q.learning.enabled) {
    out.push({label: q.learning.dedicated ? 'Learner, own key' : 'Learner', used: q.learning.used, limit: q.learning.budget})
  }
  return out
})
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
          <h1>Admin</h1>
          <div v-if="cities.length > 1" class="city-pick">
            <button type="button" class="text-button city-pick-btn" aria-label="City" :aria-expanded="cityOpen" @click="cityOpen = !cityOpen">
              {{ cityName }}
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"
                   stroke-linejoin="round" aria-hidden="true"><path d="m6 9 6 6 6-6"/></svg>
            </button>
            <div v-if="cityOpen" class="city-menu" role="dialog" aria-label="City">
              <button v-for="city in cities" :key="city.slug" type="button" class="city-option"
                      :class="{ 'is-current': city.slug === adminCity }" @click="pickCity(city.slug)">
                {{ city.name }}
              </button>
            </div>
          </div>
          <button type="button" class="text-button" :disabled="loading" @click="load">
            {{ loading ? 'Updating…' : updatedAt }}
          </button>
          <button type="button" class="text-button" @click="logout">Log out</button>
        </header>
        <p v-if="loadError" class="bad small">Refresh failed ({{ loadError }})</p>

        <div class="stats">
          <div><b>{{ num(data.now.people) }}</b><span>watching</span></div>
          <div><b>{{ num(data.people.today) }}</b><span>people today</span></div>
          <div><b>{{ liveToday?.n ? pct(liveToday.within_1) : '–' }}</b><span>within 1 min</span></div>
        </div>

        <ul v-if="data.problems.length" class="problems">
          <li v-for="p in data.problems" :key="p">{{ p }}</li>
        </ul>

        <div class="columns">
          <section>
            <div class="label"><span>Now</span><i></i></div>
            <dl class="kv">
              <div><dt>Buses</dt><dd>{{ num(data.now.buses_tracked) }} of {{ num(data.now.buses_in_service) }} · {{ num(data.now.lines_live) }} lines</dd></div>
              <div><dt>Positions</dt><dd>{{ data.now.feed_at ? ago(data.now.feed_at) : 'none yet' }}</dd></div>
              <div><dt>Polling</dt><dd>{{ pollText }}</dd></div>
              <div v-if="data.now.feed_error"><dt>Error</dt><dd class="bad">{{ data.now.feed_error }}</dd></div>
            </dl>
            <template v-if="data.now.lines.length">
              <h3>Lines on screens</h3>
              <p class="badges">
                <span v-for="l in data.now.lines" :key="l.id">
                  <span class="badge" :style="{backgroundColor: l.color || '#64748b'}">{{ l.name }}</span>{{ num(l.count) }}
                </span>
              </p>
            </template>
            <template v-if="data.now.stops.length">
              <h3>Stops on screens</h3>
              <dl class="kv">
                <div v-for="s in data.now.stops" :key="s.id"><dt class="name">{{ s.name }}</dt><dd>{{ num(s.count) }}</dd></div>
              </dl>
            </template>
          </section>

          <section>
            <div class="label"><span>People</span><i></i></div>
            <table class="table">
              <thead>
                <tr><th></th><th>Today</th><th>Week ago</th></tr>
              </thead>
              <tbody>
                <tr class="total"><td>All</td><td>{{ num(data.people.today) }}</td><td>{{ num(data.people.last_week) }}</td></tr>
                <tr v-for="p in data.people.platforms" :key="p.platform">
                  <td>{{ platformLabels[p.platform] ?? p.platform }}</td>
                  <td>{{ num(p.today) }}</td>
                  <td>{{ num(p.last_week) }}</td>
                </tr>
              </tbody>
            </table>
            <AdminDayBars :days="peopleBars"/>
            <dl class="kv">
              <div><dt>New today</dt><dd>{{ num(data.people.new) }}</dd></div>
              <div><dt>Trips planned</dt><dd>{{ num(data.people.plans) }}</dd></div>
              <div><dt>Place searches</dt><dd>{{ num(data.people.places) }}</dd></div>
              <div><dt>Follow line changes</dt><dd>{{ num(data.people.push_followers) }}</dd></div>
              <div><dt>Reminders</dt><dd>{{ num(data.people.reminders) }} from {{ num(data.people.reminder_users) }}</dd></div>
              <div><dt>Web installs</dt><dd>{{ num(data.people.installs) }}</dd></div>
              <div v-if="moves"><dt>Old address</dt><dd>{{ moves }}</dd></div>
              <div v-for="v in versionsToday" :key="v.platform + v.version">
                <dt>{{ platformLabels[v.platform] ?? v.platform }} {{ v.version }}</dt><dd>{{ num(v.today) }}</dd>
              </div>
            </dl>
            <template v-if="data.people.lines.length">
              <h3>Lines today</h3>
              <p class="badges">
                <span v-for="l in data.people.lines" :key="l.id">
                  <span class="badge" :style="{backgroundColor: l.color || '#64748b'}">{{ l.name }}</span>{{ num(l.count) }}
                </span>
              </p>
            </template>
            <template v-if="data.people.stops.length">
              <h3>Stops today</h3>
              <dl class="kv">
                <div v-for="s in data.people.stops" :key="s.id"><dt class="name">{{ s.name }}</dt><dd>{{ num(s.count) }}</dd></div>
              </dl>
            </template>
          </section>

          <section>
            <div class="label"><span>Accuracy, 7 days</span><i></i></div>
            <table class="table">
              <thead>
                <tr><th></th><th>Within 1 min</th><th>1 in 10 off by</th><th>Buses came</th></tr>
              </thead>
              <tbody>
                <tr class="group"><th colspan="4">Live, when it said</th></tr>
                <tr v-for="r in liveRows" :key="r.label" :class="{total: r.label === 'all'}">
                  <td>{{ horizonLabels[r.label] ?? r.label }}</td>
                  <td>{{ r.n ? pct(r.within_1) : '–' }}</td>
                  <td>{{ r.n ? minSec(r.p90_abs_sec) : '–' }}</td>
                  <td>{{ r.n ? came(r.bias_sec) : '–' }}</td>
                </tr>
                <tr class="group"><th colspan="4">Timetable</th></tr>
                <tr v-for="r in data.accuracy.timetable" :key="r.label">
                  <td>{{ timetableLabels[r.label] ?? r.label }}</td>
                  <td>{{ r.n ? pct(r.within_1) : '–' }}</td>
                  <td>{{ r.n ? minSec(r.p90_abs_sec) : '–' }}</td>
                  <td>{{ r.n ? came(r.bias_sec) : '–' }}</td>
                </tr>
              </tbody>
            </table>
            <AdminDayBars :days="accuracyBars" :max="1"/>
          </section>

          <section>
            <div class="label"><span>Tranzy today</span><i></i></div>
            <div v-for="m in meters" :key="m.label" class="meter">
              <p><span>{{ m.label }}</span><span>{{ num(m.used) }} / {{ num(m.limit) }}</span></p>
              <div class="track"><span :style="{width: pct(quotaShare(m.used, m.limit))}"></span></div>
            </div>
          </section>

          <section>
            <div class="label"><span>Learning</span><i></i></div>
            <dl class="kv">
              <div>
                <dt>Timetables</dt>
                <dd>{{ data.data.warmup_running ? 'refreshing' : ago(data.data.warmup_at) }} · next {{ clockTime(data.data.warmup_next) }}</dd>
              </div>
              <div><dt>Learned today</dt><dd>{{ num(data.data.samples_today) }} · {{ num(data.data.samples_hour) }} last hour</dd></div>
              <div><dt>Delays</dt><dd>{{ ago(data.data.delays_at) }}</dd></div>
              <div>
                <dt>Archive</dt>
                <dd v-if="!data.data.archive.enabled" class="bad">off</dd>
                <dd v-else-if="data.data.archive.ran_at">
                  {{ bytes(data.data.archive.bytes) }}<template v-if="data.data.archive.oldest_date"> since {{ dayName(data.data.archive.oldest_date) }}</template> · {{ ago(data.data.archive.ran_at) }}
                </dd>
                <dd v-else>not run yet</dd>
              </div>
              <div v-if="!data.data.route_changes.length"><dt>Timetable changes</dt><dd>none this week</dd></div>
            </dl>
            <template v-if="data.data.route_changes.length">
              <h3>Timetable changes</h3>
              <dl class="kv">
                <div v-for="c in data.data.route_changes" :key="`${c.line}-${c.detected_at}`">
                  <dt class="name">
                    <span class="badge" :style="{backgroundColor: c.color || '#64748b'}">{{ c.line }}</span>
                    {{ c.kinds.map(k => changeLabels[k] ?? k).join(', ') }}
                  </dt>
                  <dd>{{ ago(c.detected_at) }}</dd>
                </div>
              </dl>
            </template>
          </section>

          <section>
            <div class="label"><span>Calendar</span><i></i></div>
            <AdminCalendar :city="adminCity" @saved="load"/>
          </section>

          <section>
            <div class="label"><span>Server</span><i></i></div>
            <dl class="kv">
              <div><dt>Up</dt><dd>{{ uptime }}<template v-if="data.server.revision"> · {{ data.server.revision }}{{ data.server.modified ? '+' : '' }}</template></dd></div>
              <div>
                <dt>Requests today</dt>
                <dd>{{ num(data.server.requests) }} · <span :class="{bad: data.server.errors > 0}">{{ num(data.server.errors) }} errors</span></dd>
              </div>
              <div><dt>Memory</dt><dd>{{ bytes(data.server.heap_bytes) }}</dd></div>
              <div><dt>Database</dt><dd>{{ bytes(data.server.db_bytes) }}</dd></div>
              <div v-if="diskUsed != null"><dt>Disk</dt><dd>{{ pct(diskUsed) }} used · {{ bytes(data.server.disk_free) }} free</dd></div>
            </dl>

            <details class="more">
              <summary>Endpoints today</summary>
              <table class="table endpoints">
                <thead>
                  <tr><th></th><th>Calls</th><th>Avg</th><th>Errors</th></tr>
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
            </details>

            <h3>Failures, 24 h</h3>
            <ul v-if="data.server.problems.length" class="log">
              <li v-for="p in data.server.problems" :key="p.text">
                <span class="when">{{ ago(p.last) }}<template v-if="p.count > 1"> · {{ num(p.count) }}×</template></span>
                <code>{{ p.text }}</code>
              </li>
            </ul>
            <p v-else class="muted small">None</p>
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
  overflow-x: hidden;
  background: var(--a-bg);
  color: var(--a-text);
  color-scheme: light;
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
  font-size: 0.9375rem;
  line-height: 1.4;
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

h1 {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 700;
}

.login input {
  font: inherit;
  font-size: 1rem;
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
  max-width: 64rem;
  margin: 0 auto;
  padding: max(0.75rem, env(safe-area-inset-top)) 1rem max(2.5rem, env(safe-area-inset-bottom));
}

.top {
  display: flex;
  align-items: center;
  gap: 0.25rem;
}

.top h1 {
  flex: 1;
}

.text-button {
  font: inherit;
  font-size: 0.8125rem;
  font-weight: 600;
  min-height: 2.25rem;
  padding: 0 0.625rem;
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

.city-pick {
  position: relative;
}

.city-pick-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
}

.city-pick-btn svg {
  width: 0.875rem;
  height: 0.875rem;
}

.city-menu {
  position: absolute;
  top: calc(100% + 0.25rem);
  right: 0;
  z-index: 10;
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
  min-width: 11rem;
  padding: 0.375rem;
  border: 1px solid var(--a-rule);
  border-radius: 0.75rem;
  background: var(--a-bg);
  box-shadow: 0 10px 30px -8px rgb(0 0 0 / 0.25);
}

.city-option {
  font: inherit;
  font-size: 0.8125rem;
  font-weight: 600;
  text-align: left;
  padding: 0.5rem 0.625rem;
  border: 0;
  border-radius: 0.5rem;
  background: none;
  color: var(--a-text);
  cursor: pointer;
}

.city-option:hover {
  background: var(--a-hover);
}

.city-option.is-current {
  background: var(--a-hair);
}

.stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.75rem;
  max-width: 30rem;
  margin: 0.75rem 0 0;
}

.stats b {
  display: block;
  font-size: 1.75rem;
  font-weight: 700;
  line-height: 1.1;
  font-variant-numeric: tabular-nums;
}

.stats span {
  display: block;
  font-size: 0.75rem;
  color: var(--a-muted);
}

.problems {
  list-style: none;
  margin: 1rem 0 0;
  padding: 0;
  color: var(--a-bad);
  font-weight: 600;
}

.problems li + li {
  margin-top: 0.25rem;
}

.muted {
  margin: 0;
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
  padding-top: 1.75rem;
  break-inside: avoid;
}

.label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin: 0 0 0.5rem;
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
  background: var(--a-rule);
}

h3 {
  margin: 1rem 0 0.25rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--a-muted);
}

.kv {
  margin: 0;
}

.kv > div {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.4375rem 0;
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
  min-width: 0;
  text-align: right;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}

.badges {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem 0.875rem;
  margin: 0.25rem 0 0;
  font-variant-numeric: tabular-nums;
  color: var(--a-muted);
}

.badges > span {
  display: inline-flex;
  align-items: center;
  gap: 0.3125rem;
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
  vertical-align: middle;
}

.table {
  width: 100%;
  border-collapse: collapse;
  font-variant-numeric: tabular-nums;
}

.table th {
  padding: 0 0 0.25rem;
  font-size: 0.6875rem;
  font-weight: 600;
  line-height: 1.2;
  color: var(--a-faint);
  text-align: right;
  vertical-align: bottom;
}

.table td {
  padding: 0.4375rem 0;
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

.table td:first-child {
  color: var(--a-muted);
}

.table .total td {
  font-weight: 700;
  color: var(--a-text);
}

.table .group th {
  padding-top: 0.875rem;
  text-align: left;
}

.table tbody > .group:first-child th {
  padding-top: 0.25rem;
}

.table + :deep(.day-bars),
.kv + :deep(.day-bars) {
  margin-top: 1rem;
}

:deep(.day-bars) + .kv {
  margin-top: 0.75rem;
}

.meter + .meter {
  margin-top: 0.75rem;
}

.meter p {
  display: flex;
  justify-content: space-between;
  margin: 0 0 0.25rem;
  font-variant-numeric: tabular-nums;
}

.meter p span:first-child {
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

.more {
  margin-top: 0.5rem;
}

.more summary {
  padding: 0.4375rem 0;
  font-size: 0.8125rem;
  font-weight: 600;
  color: var(--a-muted);
  cursor: pointer;
}

.endpoints {
  font-size: 0.8125rem;
}

.endpoints td:first-child {
  white-space: normal;
  word-break: break-all;
}

.log {
  list-style: none;
  margin: 0;
  padding: 0;
}

.log li {
  padding: 0.4375rem 0;
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
