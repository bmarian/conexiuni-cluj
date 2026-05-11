<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, ref} from 'vue'
import {useHead} from '@unhead/vue'

useHead(() => ({
  title: 'Admin',
  meta: [{name: 'robots', content: 'noindex,nofollow'}],
}))

type VisitorTotals = {
  lifetime: number
  last7d: number
  today: number
}

type DailyVisit = {
  date: string
  count: number
}

type TopEntry = {
  key: string
  count: number
}

type StatsResponse = {
  visitors: VisitorTotals
  daily_visits: DailyVisit[]
  active_now: number
  top_routes: TopEntry[]
  top_stops: TopEntry[]
  top_api: TopEntry[]
  pwa_installs: number
  tranzy_quota: {
    vehicles_remaining: number
    vehicles_limit: number
    default_remaining: number
    default_limit: number
  }
  generated_at: string
}

type LogsResponse = {
  lines: string[]
  count: number
}

type AuthState = 'login' | 'loading' | 'ready' | 'error'
type SortMode = 'count_desc' | 'count_asc' | 'alpha'
type StatusKind = 'ok' | 'redirect' | 'client' | 'server' | 'other'

const tokenInput = ref('')
const authState = ref<AuthState>('loading')
const errorMessage = ref('')
const stats = ref<StatsResponse | null>(null)
const logs = ref<string[]>([])
const logTail = ref(200)
const logsLoading = ref(false)
const activeNow = ref(0)
const livePolling = ref(false)
let activePollTimer: ReturnType<typeof setInterval> | null = null

// Top-list filter/sort state (one entry per kind)
const routesSearch = ref('')
const routesSort = ref<SortMode>('count_desc')
const stopsSearch = ref('')
const stopsSort = ref<SortMode>('count_desc')
const apiSearch = ref('')
const apiSort = ref<SortMode>('count_desc')

// Log filter state
const logSearch = ref('')
const logStatusEnabled = ref<Record<StatusKind, boolean>>({
  ok: true,
  redirect: true,
  client: true,
  server: true,
  other: true,
})
const logMethodFilter = ref<string>('all')
const logOrderNewest = ref(true)

// Sparkline range (days). Backend currently returns 30, we slice client-side.
const sparkRangeDays = ref(30)

const stopLivePolling = () => {
  if (activePollTimer) {
    clearInterval(activePollTimer)
    activePollTimer = null
  }
  livePolling.value = false
}

const startLivePolling = () => {
  if (activePollTimer) return
  livePolling.value = true
  pollActive()
  activePollTimer = setInterval(pollActive, 5000)
}

const toggleLive = () => {
  if (livePolling.value) stopLivePolling()
  else startLivePolling()
}

const authedFetch = async (path: string) => {
  const res = await fetch(path, {
    credentials: 'same-origin',
  })
  if (res.status === 401) {
    authState.value = 'login'
    throw new Error('unauthorized')
  }
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}`)
  }
  return res.json()
}

const loadStats = async (silentUnauthorized = false) => {
  try {
    authState.value = stats.value ? 'ready' : 'loading'
    const data = (await authedFetch('/api/admin/stats')) as StatsResponse
    stats.value = data
    activeNow.value = data.active_now
    authState.value = 'ready'
    errorMessage.value = ''
  } catch (err) {
    if ((err as Error).message === 'unauthorized') {
      errorMessage.value = silentUnauthorized ? '' : 'Invalid token'
    } else {
      authState.value = 'error'
      errorMessage.value = (err as Error).message
    }
  }
}

const pollActive = async () => {
  if (authState.value !== 'ready') return
  try {
    const data = (await authedFetch('/api/admin/stats')) as StatsResponse
    activeNow.value = data.active_now
  } catch {
    /* swallow — caller will surface auth errors */
  }
}

const loadLogs = async () => {
  logsLoading.value = true
  try {
    const data = (await authedFetch(`/api/admin/logs?tail=${logTail.value}`)) as LogsResponse
    logs.value = data.lines
  } catch (err) {
    if ((err as Error).message !== 'unauthorized') {
      errorMessage.value = (err as Error).message
    }
  } finally {
    logsLoading.value = false
  }
}

const submitLogin = async () => {
  const t = tokenInput.value.trim()
  if (!t) return
  authState.value = 'loading'
  errorMessage.value = ''
  const res = await fetch('/api/admin/login', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    credentials: 'same-origin',
    body: JSON.stringify({token: t}),
  })
  if (res.status === 401) {
    authState.value = 'login'
    errorMessage.value = 'Invalid token'
    return
  }
  if (!res.ok) {
    authState.value = 'error'
    errorMessage.value = `HTTP ${res.status}`
    return
  }
  tokenInput.value = ''
  await loadStats()
  if ((authState.value as AuthState) === 'ready') {
    await loadLogs()
  }
}

const logout = async () => {
  stopLivePolling()
  await fetch('/api/admin/logout', {
    method: 'POST',
    credentials: 'same-origin',
  }).catch(() => {})
  stats.value = null
  logs.value = []
  authState.value = 'login'
  errorMessage.value = ''
}

const refreshAll = async () => {
  await loadStats()
  if (authState.value === 'ready') {
    await loadLogs()
  }
}

const formatNumber = (n: number) => n.toLocaleString('en-US')

const sparklinePath = computed(() => {
  const points = sparkVisits.value
  const empty = {line: '', area: '', max: 0, points: [] as Array<{x: number; y: number; v: DailyVisit}>}
  if (points.length === 0) return empty
  const width = 720
  const height = 160
  const padX = 8
  const padY = 12
  const maxV = Math.max(...points.map(p => p.count), 1)
  const stepX = points.length > 1 ? (width - padX * 2) / (points.length - 1) : 0
  const coords = points.map((p, i) => {
    const x = padX + i * stepX
    const y = padY + (1 - p.count / maxV) * (height - padY * 2)
    return {x, y, v: p}
  })
  const first = coords[0]
  const last = coords[coords.length - 1]
  if (!first || !last) return empty
  const line = coords.map((c, i) => `${i === 0 ? 'M' : 'L'} ${c.x.toFixed(1)} ${c.y.toFixed(1)}`).join(' ')
  const area = `${line} L ${last.x.toFixed(1)} ${height - padY} L ${first.x.toFixed(1)} ${height - padY} Z`
  return {line, area, max: maxV, points: coords}
})

const hoveredPoint = ref<{x: number; y: number; v: DailyVisit} | null>(null)

const classifyStatus = (line: string): 'ok' | 'redirect' | 'client' | 'server' | 'other' => {
  const m = line.match(/status=(\d+)/)
  if (!m || !m[1]) return 'other'
  const code = parseInt(m[1], 10)
  if (code >= 500) return 'server'
  if (code >= 400) return 'client'
  if (code >= 300) return 'redirect'
  if (code >= 200) return 'ok'
  return 'other'
}

const barPercent = (count: number, list: TopEntry[]) => {
  const top = list[0]?.count ?? 1
  return `${(count / top) * 100}%`
}

const applyTopFilter = (list: TopEntry[], search: string, sort: SortMode) => {
  const q = search.trim().toLowerCase()
  const filtered = q ? list.filter(e => e.key.toLowerCase().includes(q)) : list
  const sorted = [...filtered]
  if (sort === 'count_asc') sorted.sort((a, b) => a.count - b.count)
  else if (sort === 'alpha') sorted.sort((a, b) => a.key.localeCompare(b.key))
  // count_desc is the backend default; keep order.
  return sorted
}

const filteredTopRoutes = computed(() =>
  applyTopFilter(stats.value?.top_routes ?? [], routesSearch.value, routesSort.value),
)
const filteredTopStops = computed(() =>
  applyTopFilter(stats.value?.top_stops ?? [], stopsSearch.value, stopsSort.value),
)
const filteredTopAPI = computed(() =>
  applyTopFilter(stats.value?.top_api ?? [], apiSearch.value, apiSort.value),
)

const extractMethod = (line: string): string => {
  const m = line.match(/method=(\w+)/)
  return m && m[1] ? m[1] : ''
}

const availableMethods = computed(() => {
  const set = new Set<string>()
  for (const line of logs.value) {
    const m = extractMethod(line)
    if (m) set.add(m)
  }
  return [...set].sort()
})

const statusCounts = computed(() => {
  const counts: Record<StatusKind, number> = {ok: 0, redirect: 0, client: 0, server: 0, other: 0}
  for (const line of logs.value) counts[classifyStatus(line)]++
  return counts
})

const filteredLogs = computed(() => {
  const q = logSearch.value.trim().toLowerCase()
  const method = logMethodFilter.value
  const out: string[] = []
  for (const line of logs.value) {
    if (!logStatusEnabled.value[classifyStatus(line)]) continue
    if (method !== 'all' && extractMethod(line) !== method) continue
    if (q && !line.toLowerCase().includes(q)) continue
    out.push(line)
  }
  return logOrderNewest.value ? out.reverse() : out
})

const toggleStatus = (kind: StatusKind) => {
  logStatusEnabled.value[kind] = !logStatusEnabled.value[kind]
}

const clearLogFilters = () => {
  logSearch.value = ''
  logMethodFilter.value = 'all'
  logStatusEnabled.value = {ok: true, redirect: true, client: true, server: true, other: true}
}

const sparkVisits = computed(() => {
  const all = stats.value?.daily_visits ?? []
  return all.slice(-sparkRangeDays.value)
})

const wowDelta = computed<number | null>(() => {
  const all = stats.value?.daily_visits ?? []
  if (all.length < 14) return null
  const last7 = all.slice(-7).reduce((s, d) => s + d.count, 0)
  const prev7 = all.slice(-14, -7).reduce((s, d) => s + d.count, 0)
  if (prev7 === 0) return null
  return ((last7 - prev7) / prev7) * 100
})

const rangeTotal = computed(() => sparkVisits.value.reduce((s, d) => s + d.count, 0))

onMounted(async () => {
  await loadStats(true)
  if (authState.value === 'ready') {
    await loadLogs()
  }
})

onBeforeUnmount(() => {
  stopLivePolling()
})
</script>

<template>
  <Teleport to="body">
    <div class="admin-shell">
      <!-- Login -->
      <div v-if="authState === 'login'" class="admin-login">
        <form class="admin-login-card" @submit.prevent="submitLogin">
          <div class="admin-login-title">conexiuni-cluj · admin</div>
          <div class="admin-login-sub">Enter access token</div>
          <input
            type="text"
            name="username"
            value="admin"
            autocomplete="username"
            readonly
            tabindex="-1"
            aria-hidden="true"
            class="admin-hidden-username"
          />
          <input
            v-model="tokenInput"
            type="password"
            name="password"
            autocomplete="current-password"
            spellcheck="false"
            placeholder="token"
            class="admin-input"
            autofocus
          />
          <button type="submit" class="admin-button-primary">Unlock</button>
          <div v-if="errorMessage" class="admin-error">{{ errorMessage }}</div>
        </form>
      </div>

      <!-- Dashboard -->
      <div v-else class="admin-dash">
        <header class="admin-header">
          <div class="admin-header-left">
            <span class="admin-pulse" :class="{'admin-pulse-idle': !livePolling}"></span>
            <span class="admin-title">conexiuni-cluj · admin</span>
            <span v-if="stats" class="admin-generated">
              updated {{ new Date(stats.generated_at).toLocaleTimeString() }}
            </span>
          </div>
          <div class="admin-header-right">
            <button
              class="admin-button admin-button-live"
              :class="{'admin-button-live-on': livePolling}"
              @click="toggleLive"
            >
              <span class="admin-live-dot"></span>
              {{ livePolling ? 'Live' : 'Go live' }}
            </button>
            <button class="admin-button" :disabled="authState === 'loading'" @click="refreshAll">
              {{ authState === 'loading' ? '…' : 'Refresh' }}
            </button>
            <button class="admin-button-ghost" @click="logout">Logout</button>
          </div>
        </header>

        <div v-if="authState === 'loading' && !stats" class="admin-loading">Loading…</div>
        <div v-else-if="authState === 'error' && !stats" class="admin-error-box">
          Failed to load stats: {{ errorMessage }}
        </div>

        <template v-if="stats">
          <!-- KPI cards -->
          <section class="admin-kpis">
            <div class="admin-kpi admin-kpi-accent">
              <div class="admin-kpi-label">Active now</div>
              <div class="admin-kpi-value">{{ formatNumber(activeNow) }}</div>
              <div class="admin-kpi-sub">live SSE subscribers</div>
            </div>
            <div class="admin-kpi">
              <div class="admin-kpi-label">Today</div>
              <div class="admin-kpi-value">{{ formatNumber(stats.visitors.today) }}</div>
              <div class="admin-kpi-sub">unique visitors</div>
            </div>
            <div class="admin-kpi">
              <div class="admin-kpi-label">Last 7 days</div>
              <div class="admin-kpi-value">{{ formatNumber(stats.visitors.last7d) }}</div>
              <div class="admin-kpi-sub">unique visitors</div>
            </div>
            <div class="admin-kpi">
              <div class="admin-kpi-label">Lifetime</div>
              <div class="admin-kpi-value">{{ formatNumber(stats.visitors.lifetime) }}</div>
              <div class="admin-kpi-sub">all-time uniques</div>
            </div>
            <div class="admin-kpi">
              <div class="admin-kpi-label">PWA installs</div>
              <div class="admin-kpi-value">{{ formatNumber(stats.pwa_installs) }}</div>
              <div class="admin-kpi-sub">install events</div>
            </div>
            <div class="admin-kpi">
              <div class="admin-kpi-label">Tranzy · vehicles</div>
              <div class="admin-kpi-value">{{ formatNumber(stats.tranzy_quota.vehicles_remaining) }}</div>
              <div class="admin-kpi-sub">of {{ formatNumber(stats.tranzy_quota.vehicles_limit) }} left today</div>
            </div>
            <div class="admin-kpi">
              <div class="admin-kpi-label">Tranzy · default</div>
              <div class="admin-kpi-value">{{ formatNumber(stats.tranzy_quota.default_remaining) }}</div>
              <div class="admin-kpi-sub">of {{ formatNumber(stats.tranzy_quota.default_limit) }} left today</div>
            </div>
          </section>

          <!-- Visitor sparkline -->
          <section class="admin-card">
            <div class="admin-card-head">
              <h2>Daily unique visitors</h2>
              <div class="admin-spark-controls">
                <div class="admin-chip-group">
                  <button class="admin-chip" :class="{'admin-chip-on': sparkRangeDays === 7}" @click="sparkRangeDays = 7">7d</button>
                  <button class="admin-chip" :class="{'admin-chip-on': sparkRangeDays === 14}" @click="sparkRangeDays = 14">14d</button>
                  <button class="admin-chip" :class="{'admin-chip-on': sparkRangeDays === 30}" @click="sparkRangeDays = 30">30d</button>
                </div>
                <span class="admin-card-meta">
                  total {{ formatNumber(rangeTotal) }} · peak {{ formatNumber(sparklinePath.max) }}
                </span>
                <span
                  v-if="wowDelta !== null"
                  class="admin-delta"
                  :class="wowDelta >= 0 ? 'admin-delta-up' : 'admin-delta-down'"
                  title="Last 7d total vs. previous 7d"
                >
                  WoW {{ wowDelta >= 0 ? '+' : '' }}{{ wowDelta.toFixed(1) }}%
                </span>
              </div>
            </div>
            <div class="admin-spark">
              <svg viewBox="0 0 720 160" preserveAspectRatio="none" class="admin-spark-svg">
                <defs>
                  <linearGradient id="adminSparkFill" x1="0" x2="0" y1="0" y2="1">
                    <stop offset="0%" stop-color="currentColor" stop-opacity="0.45"/>
                    <stop offset="100%" stop-color="currentColor" stop-opacity="0"/>
                  </linearGradient>
                </defs>
                <path :d="sparklinePath.area" fill="url(#adminSparkFill)"/>
                <path :d="sparklinePath.line" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                <g>
                  <circle
                    v-for="p in sparklinePath.points"
                    :key="p.v.date"
                    :cx="p.x"
                    :cy="p.y"
                    r="6"
                    fill="transparent"
                    @mouseenter="hoveredPoint = p"
                    @mouseleave="hoveredPoint = null"
                  />
                  <circle
                    v-if="hoveredPoint"
                    :cx="hoveredPoint.x"
                    :cy="hoveredPoint.y"
                    r="3.5"
                    fill="currentColor"
                  />
                </g>
              </svg>
              <div v-if="hoveredPoint" class="admin-spark-tip" :style="{left: `${(hoveredPoint.x / 720) * 100}%`}">
                <div class="admin-spark-tip-date">{{ hoveredPoint.v.date }}</div>
                <div class="admin-spark-tip-count">{{ formatNumber(hoveredPoint.v.count) }} visitors</div>
              </div>
            </div>
            <div class="admin-spark-axis">
              <span>{{ sparkVisits[0]?.date }}</span>
              <span>{{ sparkVisits[sparkVisits.length - 1]?.date }}</span>
            </div>
          </section>

          <!-- Top lists grid -->
          <section class="admin-grid">
            <div class="admin-card">
              <div class="admin-card-head">
                <h2>Top routes</h2>
                <div class="admin-top-controls">
                  <input v-model="routesSearch" type="text" placeholder="filter…" class="admin-input admin-input-sm" spellcheck="false"/>
                  <select v-model="routesSort" class="admin-select">
                    <option value="count_desc">most → least</option>
                    <option value="count_asc">least → most</option>
                    <option value="alpha">A → Z</option>
                  </select>
                  <span class="admin-card-meta">{{ filteredTopRoutes.length }}/{{ stats.top_routes.length }}</span>
                </div>
              </div>
              <ul v-if="filteredTopRoutes.length" class="admin-bars">
                <li v-for="(r, i) in filteredTopRoutes" :key="r.key">
                  <span class="admin-bar-rank">#{{ i + 1 }}</span>
                  <span class="admin-bar-label">{{ r.key }}</span>
                  <span class="admin-bar-track">
                    <span class="admin-bar-fill" :style="{width: barPercent(r.count, stats.top_routes)}"></span>
                  </span>
                  <span class="admin-bar-count">{{ formatNumber(r.count) }}</span>
                </li>
              </ul>
              <div v-else class="admin-empty">{{ stats.top_routes.length ? 'No matches' : 'No data yet' }}</div>
            </div>

            <div class="admin-card">
              <div class="admin-card-head">
                <h2>Top stops</h2>
                <div class="admin-top-controls">
                  <input v-model="stopsSearch" type="text" placeholder="filter…" class="admin-input admin-input-sm" spellcheck="false"/>
                  <select v-model="stopsSort" class="admin-select">
                    <option value="count_desc">most → least</option>
                    <option value="count_asc">least → most</option>
                    <option value="alpha">A → Z</option>
                  </select>
                  <span class="admin-card-meta">{{ filteredTopStops.length }}/{{ stats.top_stops.length }}</span>
                </div>
              </div>
              <ul v-if="filteredTopStops.length" class="admin-bars">
                <li v-for="(s, i) in filteredTopStops" :key="s.key">
                  <span class="admin-bar-rank">#{{ i + 1 }}</span>
                  <span class="admin-bar-label">Stop {{ s.key }}</span>
                  <span class="admin-bar-track">
                    <span class="admin-bar-fill" :style="{width: barPercent(s.count, stats.top_stops)}"></span>
                  </span>
                  <span class="admin-bar-count">{{ formatNumber(s.count) }}</span>
                </li>
              </ul>
              <div v-else class="admin-empty">{{ stats.top_stops.length ? 'No matches' : 'No data yet' }}</div>
            </div>

            <div class="admin-card">
              <div class="admin-card-head">
                <h2>Top API endpoints</h2>
                <div class="admin-top-controls">
                  <input v-model="apiSearch" type="text" placeholder="filter…" class="admin-input admin-input-sm" spellcheck="false"/>
                  <select v-model="apiSort" class="admin-select">
                    <option value="count_desc">most → least</option>
                    <option value="count_asc">least → most</option>
                    <option value="alpha">A → Z</option>
                  </select>
                  <span class="admin-card-meta">{{ filteredTopAPI.length }}/{{ stats.top_api.length }}</span>
                </div>
              </div>
              <ul v-if="filteredTopAPI.length" class="admin-bars">
                <li v-for="(a, i) in filteredTopAPI" :key="a.key">
                  <span class="admin-bar-rank">#{{ i + 1 }}</span>
                  <span class="admin-bar-label admin-bar-mono">{{ a.key }}</span>
                  <span class="admin-bar-track">
                    <span class="admin-bar-fill" :style="{width: barPercent(a.count, stats.top_api)}"></span>
                  </span>
                  <span class="admin-bar-count">{{ formatNumber(a.count) }}</span>
                </li>
              </ul>
              <div v-else class="admin-empty">{{ stats.top_api.length ? 'No matches' : 'No data yet' }}</div>
            </div>
          </section>

          <!-- Logs -->
          <section class="admin-card">
            <div class="admin-card-head">
              <h2>Access logs</h2>
              <div class="admin-log-controls">
                <label>
                  tail
                  <input v-model.number="logTail" type="number" min="10" max="1000" step="10" class="admin-input admin-input-sm"/>
                </label>
                <button class="admin-button" :disabled="logsLoading" @click="loadLogs">
                  {{ logsLoading ? '…' : 'Reload' }}
                </button>
              </div>
            </div>

            <div class="admin-log-filters">
              <div class="admin-chip-group">
                <button
                  class="admin-chip admin-chip-ok"
                  :class="{'admin-chip-on': logStatusEnabled.ok}"
                  @click="toggleStatus('ok')"
                >2xx <span class="admin-chip-n">{{ statusCounts.ok }}</span></button>
                <button
                  class="admin-chip admin-chip-redirect"
                  :class="{'admin-chip-on': logStatusEnabled.redirect}"
                  @click="toggleStatus('redirect')"
                >3xx <span class="admin-chip-n">{{ statusCounts.redirect }}</span></button>
                <button
                  class="admin-chip admin-chip-client"
                  :class="{'admin-chip-on': logStatusEnabled.client}"
                  @click="toggleStatus('client')"
                >4xx <span class="admin-chip-n">{{ statusCounts.client }}</span></button>
                <button
                  class="admin-chip admin-chip-server"
                  :class="{'admin-chip-on': logStatusEnabled.server}"
                  @click="toggleStatus('server')"
                >5xx <span class="admin-chip-n">{{ statusCounts.server }}</span></button>
                <button
                  v-if="statusCounts.other"
                  class="admin-chip"
                  :class="{'admin-chip-on': logStatusEnabled.other}"
                  @click="toggleStatus('other')"
                >other <span class="admin-chip-n">{{ statusCounts.other }}</span></button>
              </div>
              <select v-model="logMethodFilter" class="admin-select">
                <option value="all">all methods</option>
                <option v-for="m in availableMethods" :key="m" :value="m">{{ m }}</option>
              </select>
              <input
                v-model="logSearch"
                type="text"
                placeholder="search…"
                spellcheck="false"
                class="admin-input admin-input-sm admin-log-search"
              />
              <button class="admin-button-ghost admin-button-sm" @click="logOrderNewest = !logOrderNewest" title="Toggle order">
                {{ logOrderNewest ? 'newest ↓' : 'oldest ↑' }}
              </button>
              <button class="admin-button-ghost admin-button-sm" @click="clearLogFilters">clear</button>
              <span class="admin-card-meta admin-log-count">{{ filteredLogs.length }}/{{ logs.length }}</span>
            </div>

            <div v-if="logs.length === 0 && !logsLoading" class="admin-empty">No log lines</div>
            <div v-else-if="filteredLogs.length === 0" class="admin-empty">No log lines match the filters</div>
            <pre v-else class="admin-logs">
<span
  v-for="(line, i) in filteredLogs"
  :key="i"
  class="admin-log-line"
  :class="`admin-log-${classifyStatus(line)}`"
>{{ line }}</span></pre>
          </section>
        </template>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.admin-shell {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background: #0b1220;
  color: #e6edf3;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
  overflow-y: auto;
  padding: 0;
}

/* Login */
.admin-login {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  padding: 1rem;
}

.admin-login-card {
  width: 100%;
  max-width: 360px;
  background: #111a2c;
  border: 1px solid #1f2a44;
  border-radius: 14px;
  padding: 2rem 1.75rem;
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.45);
}

.admin-login-title {
  font-weight: 600;
  font-size: 1.05rem;
  letter-spacing: 0.02em;
}

.admin-login-sub {
  color: #94a3b8;
  font-size: 0.85rem;
  margin-bottom: 0.5rem;
}

.admin-input {
  width: 100%;
  background: #0b1220;
  border: 1px solid #233052;
  color: #e6edf3;
  border-radius: 8px;
  padding: 0.6rem 0.75rem;
  font-size: 0.95rem;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  outline: none;
  transition: border-color 0.15s;
}

.admin-hidden-username {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
  opacity: 0;
  pointer-events: none;
}

.admin-input:focus {
  border-color: #38bdf8;
}

.admin-input-sm {
  width: 5rem;
  padding: 0.3rem 0.5rem;
  font-size: 0.85rem;
}

.admin-button-primary,
.admin-button,
.admin-button-ghost {
  border-radius: 8px;
  padding: 0.55rem 0.95rem;
  font-size: 0.9rem;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s, transform 0.05s;
  border: 1px solid transparent;
}

.admin-button-primary {
  background: #38bdf8;
  color: #0b1220;
  border-color: #38bdf8;
}

.admin-button-primary:hover {
  background: #7dd3fc;
  border-color: #7dd3fc;
}

.admin-button {
  background: #1c2942;
  color: #e6edf3;
  border-color: #2a3a5e;
}

.admin-button:hover:not(:disabled) {
  background: #243454;
}

.admin-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.admin-button-ghost {
  background: transparent;
  color: #94a3b8;
  border-color: transparent;
}

.admin-button-ghost:hover {
  color: #e6edf3;
}

.admin-error {
  color: #fca5a5;
  font-size: 0.85rem;
  margin-top: 0.25rem;
}

.admin-error-box {
  margin: 1.5rem;
  padding: 1rem;
  border: 1px solid #7f1d1d;
  background: #1a0d10;
  color: #fca5a5;
  border-radius: 8px;
}

/* Dashboard */
.admin-dash {
  padding: 1.5rem clamp(1rem, 3vw, 2rem) 3rem;
  max-width: 1400px;
  margin: 0 auto;
}

.admin-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding-bottom: 1.5rem;
  border-bottom: 1px solid #1f2a44;
  margin-bottom: 1.5rem;
  flex-wrap: wrap;
}

.admin-header-left {
  display: flex;
  align-items: center;
  gap: 0.65rem;
}

.admin-header-right {
  display: flex;
  gap: 0.5rem;
}

.admin-pulse {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #34d399;
  box-shadow: 0 0 0 0 rgba(52, 211, 153, 0.6);
  animation: admin-pulse 2s infinite;
}

.admin-pulse.admin-pulse-idle {
  background: #475569;
  animation: none;
  box-shadow: none;
}

@keyframes admin-pulse {
  0% { box-shadow: 0 0 0 0 rgba(52, 211, 153, 0.6); }
  70% { box-shadow: 0 0 0 10px rgba(52, 211, 153, 0); }
  100% { box-shadow: 0 0 0 0 rgba(52, 211, 153, 0); }
}

.admin-button-live {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
}

.admin-live-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #475569;
  transition: background 0.2s, box-shadow 0.2s;
}

.admin-button-live-on {
  background: #052e1a;
  border-color: #14532d;
  color: #86efac;
}

.admin-button-live-on .admin-live-dot {
  background: #34d399;
  box-shadow: 0 0 0 0 rgba(52, 211, 153, 0.6);
  animation: admin-pulse 2s infinite;
}

.admin-button-live-on:hover {
  background: #07401f;
}

.admin-title {
  font-weight: 600;
  letter-spacing: 0.01em;
}

.admin-generated {
  color: #64748b;
  font-size: 0.8rem;
  margin-left: 0.5rem;
}

.admin-loading {
  color: #94a3b8;
  padding: 2rem;
  text-align: center;
}

/* KPI cards */
.admin-kpis {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.85rem;
  margin-bottom: 1.5rem;
}

.admin-kpi {
  background: #111a2c;
  border: 1px solid #1f2a44;
  border-radius: 12px;
  padding: 1rem 1.1rem;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  transition: border-color 0.2s, transform 0.2s;
}

.admin-kpi:hover {
  border-color: #2a3a5e;
  transform: translateY(-1px);
}

.admin-kpi-accent {
  background: linear-gradient(135deg, #0c2030 0%, #111a2c 100%);
  border-color: #1e4d6a;
}

.admin-kpi-label {
  color: #94a3b8;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.admin-kpi-value {
  font-size: 2rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  line-height: 1.1;
}

.admin-kpi-accent .admin-kpi-value {
  color: #7dd3fc;
}

.admin-kpi-sub {
  color: #64748b;
  font-size: 0.78rem;
}

/* Cards */
.admin-card {
  background: #111a2c;
  border: 1px solid #1f2a44;
  border-radius: 12px;
  padding: 1.1rem 1.25rem;
  margin-bottom: 1.25rem;
}

.admin-card-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 1rem;
  flex-wrap: wrap;
}

.admin-card-head h2 {
  font-size: 1rem;
  font-weight: 600;
  margin: 0;
}

.admin-card-meta {
  color: #64748b;
  font-size: 0.8rem;
}

.admin-empty {
  color: #64748b;
  font-size: 0.85rem;
  padding: 0.5rem 0;
}

/* Sparkline */
.admin-spark {
  position: relative;
  color: #38bdf8;
  width: 100%;
}

.admin-spark-svg {
  width: 100%;
  height: 160px;
  display: block;
}

.admin-spark-tip {
  position: absolute;
  top: -10px;
  transform: translateX(-50%);
  background: #0b1220;
  border: 1px solid #2a3a5e;
  border-radius: 6px;
  padding: 0.35rem 0.55rem;
  font-size: 0.78rem;
  white-space: nowrap;
  pointer-events: none;
  color: #e6edf3;
}

.admin-spark-tip-date {
  color: #94a3b8;
  font-variant-numeric: tabular-nums;
}

.admin-spark-tip-count {
  font-weight: 600;
}

.admin-spark-axis {
  display: flex;
  justify-content: space-between;
  color: #64748b;
  font-size: 0.72rem;
  margin-top: 0.4rem;
  font-variant-numeric: tabular-nums;
}

/* Grid */
.admin-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 1.25rem;
  margin-bottom: 1.25rem;
}

.admin-grid .admin-card {
  margin-bottom: 0;
}

/* Bars */
.admin-bars {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.admin-bars li {
  display: grid;
  grid-template-columns: 2rem minmax(0, 1fr) minmax(0, 1.4fr) auto;
  align-items: center;
  gap: 0.6rem;
  font-size: 0.85rem;
}

.admin-bar-rank {
  color: #475569;
  font-variant-numeric: tabular-nums;
  font-size: 0.78rem;
}

.admin-bar-label {
  color: #cbd5e1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.admin-bar-mono {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 0.78rem;
  color: #94a3b8;
}

.admin-bar-track {
  position: relative;
  height: 8px;
  background: #0b1220;
  border-radius: 4px;
  overflow: hidden;
}

.admin-bar-fill {
  position: absolute;
  inset: 0 auto 0 0;
  background: linear-gradient(90deg, #38bdf8 0%, #818cf8 100%);
  border-radius: 4px;
  transition: width 0.4s cubic-bezier(0.22, 1, 0.36, 1);
}

.admin-bar-count {
  font-variant-numeric: tabular-nums;
  color: #e6edf3;
  font-weight: 500;
}

/* Logs */
.admin-log-controls {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  color: #94a3b8;
  font-size: 0.8rem;
}

.admin-log-controls label {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}

.admin-logs {
  background: #0b1220;
  border: 1px solid #1f2a44;
  border-radius: 8px;
  margin: 0;
  padding: 0.75rem;
  max-height: 480px;
  overflow: auto;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 0.72rem;
  line-height: 1.5;
  white-space: pre;
  display: flex;
  flex-direction: column;
}

.admin-log-line {
  display: block;
  padding: 0.05rem 0;
  border-left: 3px solid transparent;
  padding-left: 0.5rem;
}

.admin-log-ok       { color: #86efac; border-left-color: #16a34a; }
.admin-log-redirect { color: #93c5fd; border-left-color: #2563eb; }
.admin-log-client   { color: #fcd34d; border-left-color: #d97706; }
.admin-log-server   { color: #fca5a5; border-left-color: #b91c1c; }
.admin-log-other    { color: #cbd5e1; border-left-color: #475569; }

/* Shared select */
.admin-select {
  background: #0b1220;
  border: 1px solid #233052;
  color: #e6edf3;
  border-radius: 6px;
  padding: 0.3rem 0.5rem;
  font-size: 0.8rem;
  font-family: inherit;
  outline: none;
  cursor: pointer;
  transition: border-color 0.15s;
}

.admin-select:focus {
  border-color: #38bdf8;
}

/* Compact button variant */
.admin-button-sm {
  padding: 0.3rem 0.55rem;
  font-size: 0.78rem;
  border-radius: 6px;
}

/* Chip groups (sparkline range, status filters) */
.admin-chip-group {
  display: inline-flex;
  gap: 0.25rem;
  align-items: center;
  flex-wrap: wrap;
}

.admin-chip {
  background: transparent;
  border: 1px solid #233052;
  color: #94a3b8;
  border-radius: 6px;
  padding: 0.25rem 0.55rem;
  font-size: 0.75rem;
  font-weight: 500;
  cursor: pointer;
  font-family: inherit;
  transition: background 0.15s, border-color 0.15s, color 0.15s;
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
}

.admin-chip:hover {
  border-color: #2a3a5e;
  color: #e6edf3;
}

.admin-chip-on {
  background: #1c2942;
  border-color: #38bdf8;
  color: #e6edf3;
}

.admin-chip-n {
  font-variant-numeric: tabular-nums;
  color: #64748b;
  font-size: 0.7rem;
}

.admin-chip-on .admin-chip-n {
  color: #94a3b8;
}

.admin-chip-ok.admin-chip-on       { border-color: #16a34a; }
.admin-chip-redirect.admin-chip-on { border-color: #2563eb; }
.admin-chip-client.admin-chip-on   { border-color: #d97706; }
.admin-chip-server.admin-chip-on   { border-color: #b91c1c; }

/* Sparkline controls */
.admin-spark-controls {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  flex-wrap: wrap;
}

.admin-delta {
  font-size: 0.78rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  padding: 0.15rem 0.45rem;
  border-radius: 4px;
}

.admin-delta-up   { color: #86efac; background: rgba(22, 163, 74, 0.12); }
.admin-delta-down { color: #fca5a5; background: rgba(185, 28, 28, 0.12); }

/* Top-list controls */
.admin-top-controls {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  flex-wrap: wrap;
}

.admin-top-controls .admin-input-sm {
  width: 7rem;
}

/* Log filters */
.admin-log-filters {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
  margin-bottom: 0.75rem;
  padding-bottom: 0.75rem;
  border-bottom: 1px dashed #1f2a44;
}

.admin-log-search {
  flex: 1 1 12rem;
  min-width: 8rem;
}

.admin-log-count {
  margin-left: auto;
  font-variant-numeric: tabular-nums;
}
</style>
