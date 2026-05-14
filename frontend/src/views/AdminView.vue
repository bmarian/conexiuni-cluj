<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, ref} from 'vue'
import {useHead} from '@unhead/vue'
import {useSettingsStore} from '@/stores/settings.ts'

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

type DailyTranzyQuota = {
  date: string
  vehicles: number
  default: number
  total: number
}

type TopEntry = {
  key: string
  count: number
}

type EndpointTiming = {
  key: string
  count: number
  avg_ms: number
}

type CacheGroup = {
  prefix: string
  count: number
  expired_count: number
  earliest_expires_at: string
  latest_expires_at: string
  lifespan_ms: number
}

type WarmupSnapshot = {
  last_started_at?: string
  last_completed_at?: string
  last_duration_ms?: number
  next_scheduled_at: string
}

type SegmentLearningRoute = {
  route_id: number
  route_short_name: string
  samples: number
  profiles: number
}

type SegmentLearningSnapshot = {
  observed_at?: string
  vehicles: number
  accepted: number
  stored: number
  profiles_created: number
  profiles_updated: number
  profiles_unchanged: number
  rejected: number
  ignored_reset: number
  ignored_no_progress: number
  ignored_non_adjacent: number
  ignored_no_tracker: number
  stale: number
  invalid: number
  sample_errors: number
  profile_errors: number
}

type SegmentLearning = {
  total_samples: number
  total_profiles: number
  routes_with_profiles: number
  last_sample_at?: string
  last_snapshot: SegmentLearningSnapshot
  top_routes: SegmentLearningRoute[]
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
    vehicles_used: number
    default_remaining: number
    default_limit: number
    default_used: number
  }
  daily_tranzy_quota: DailyTranzyQuota[]
  endpoint_response_times: EndpointTiming[]
  cache_groups: CacheGroup[]
  segment_learning: SegmentLearning
  warmup: WarmupSnapshot
  generated_at: string
}

type AuthState = 'login' | 'loading' | 'ready' | 'error'
type SortMode = 'count_desc' | 'count_asc' | 'alpha'

const AUTH_FLAG_KEY = 'admin:authed'
const hasAuthFlag = () => {
  try { return localStorage.getItem(AUTH_FLAG_KEY) === '1' } catch { return false }
}
const setAuthFlag = (v: boolean) => {
  try { v ? localStorage.setItem(AUTH_FLAG_KEY, '1') : localStorage.removeItem(AUTH_FLAG_KEY) } catch { /* noop */ }
}

const tokenInput = ref('')
const settings = useSettingsStore()
const authState = ref<AuthState>(hasAuthFlag() ? 'loading' : 'login')
const errorMessage = ref('')
const stats = ref<StatsResponse | null>(null)
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
    setAuthFlag(false)
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
    stats.value = data
    activeNow.value = data.active_now
  } catch {
    /* swallow — caller will surface auth errors */
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
  setAuthFlag(true)
  await loadStats()
}

const logout = async () => {
  stopLivePolling()
  await fetch('/api/admin/logout', {
    method: 'POST',
    credentials: 'same-origin',
  }).catch(() => {})
  setAuthFlag(false)
  stats.value = null
  authState.value = 'login'
  errorMessage.value = ''
}

const refreshAll = async () => {
  await loadStats()
}

const formatNumber = (n: number) => n.toLocaleString('en-US')

const formatRoDay = (date?: string) => {
  if (!date) return ''
  const [, month, day] = date.split('-')
  if (!month || !day) return date
  return `${day}-${month}`
}

const formatMs = (ms: number) => {
  if (!Number.isFinite(ms)) return '0 ms'
  if (ms >= 1000) return `${(ms / 1000).toFixed(ms >= 10000 ? 0 : 1)} s`
  return `${Math.round(ms)} ms`
}

const nowTick = ref(Date.now())
let nowTimer: ReturnType<typeof setInterval> | null = null

const formatDuration = (ms: number) => {
  if (!Number.isFinite(ms) || ms <= 0) return '0s'
  const sec = Math.round(ms / 1000)
  if (sec < 60) return `${sec}s`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m ${sec % 60}s`
  const h = Math.floor(min / 60)
  if (h < 24) return `${h}h ${min % 60}m`
  const d = Math.floor(h / 24)
  return `${d}d ${h % 24}h`
}

const YEAR_MS = 365 * 24 * 60 * 60 * 1000

const formatRelative = (iso?: string) => {
  if (!iso) return '—'
  const t = new Date(iso).getTime()
  if (!Number.isFinite(t)) return '—'
  const delta = t - nowTick.value
  if (Math.abs(delta) > YEAR_MS) return delta > 0 ? 'never' : 'long ago'
  if (Math.abs(delta) < 5_000) return 'now'
  if (delta > 0) return `in ${formatDuration(delta)}`
  return `${formatDuration(-delta)} ago`
}

const formatLifespan = (ms: number) => {
  if (!Number.isFinite(ms) || ms <= 0) return '—'
  if (ms > YEAR_MS) return '∞'
  return formatDuration(ms)
}

const formatDateTime = (iso?: string) => {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString()
}

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

const barPercent = (count: number, list: TopEntry[]) => {
  const top = list[0]?.count ?? 1
  return `${(count / top) * 100}%`
}

const latencyBarPercent = (ms: number) => {
  const top = endpointResponseTimes.value[0]?.avg_ms ?? 1
  return `${(ms / top) * 100}%`
}

const quotaBarPercent = (count: number) => {
  return `${(count / tranzyQuotaPeak.value) * 100}%`
}

const usedPercent = (used: number, limit: number) => {
  if (!Number.isFinite(used) || !Number.isFinite(limit) || limit <= 0) return '0%'
  const pct = Math.max(0, Math.min(100, (used / limit) * 100))
  return `${pct}%`
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

const recentTranzyQuota = computed(() => (stats.value?.daily_tranzy_quota ?? []).filter(d => d.total > 0).slice(-14))

const tranzyQuotaPeak = computed(() => Math.max(...recentTranzyQuota.value.map(d => d.total), 1))

const recentTranzyQuotaTotal = computed(() => recentTranzyQuota.value.reduce((sum, d) => sum + d.total, 0))

const endpointResponseTimes = computed(() => stats.value?.endpoint_response_times ?? [])

const cacheGroups = computed(() => stats.value?.cache_groups ?? [])

const warmupInfo = computed(() => stats.value?.warmup ?? null)

const segmentLearning = computed(() => stats.value?.segment_learning ?? null)

const segmentLearningRoutePeak = computed(() =>
  Math.max(...(segmentLearning.value?.top_routes ?? []).map(r => r.profiles), 1),
)

const segmentLearningBarPercent = (profiles: number) => `${(profiles / segmentLearningRoutePeak.value) * 100}%`

const segmentSnapshot = computed(() => segmentLearning.value?.last_snapshot ?? null)

const segmentSnapshotIgnored = computed(() => {
  const s = segmentSnapshot.value
  if (!s) return 0
  return s.ignored_reset + s.ignored_no_progress + s.ignored_non_adjacent + s.ignored_no_tracker
})

const cacheHealthyCount = computed(() =>
  cacheGroups.value.reduce((n, g) => n + (g.count - g.expired_count), 0),
)
const cacheExpiredCount = computed(() =>
  cacheGroups.value.reduce((n, g) => n + g.expired_count, 0),
)

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
  nowTimer = setInterval(() => { nowTick.value = Date.now() }, 1000)
  if (!hasAuthFlag()) return
  await loadStats(true)
})

onBeforeUnmount(() => {
  stopLivePolling()
  if (nowTimer) {
    clearInterval(nowTimer)
    nowTimer = null
  }
})
</script>

<template>
  <Teleport to="body">
    <div
      class="admin-shell"
      :class="{
        'is-dark': settings.isDark,
        'is-arcade': settings.arcadeActive,
        'is-legacy-blue': settings.legacyBlueActive,
      }"
    >
      <div v-if="authState === 'login'" class="admin-login">
        <form class="admin-login-card" @submit.prevent="submitLogin">
          <div class="admin-login-mark">cc</div>
          <div>
            <div class="admin-login-title">conexiuni-cluj</div>
            <div class="admin-login-sub">Admin access</div>
          </div>
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

      <div v-else class="admin-dash">
        <header class="admin-header">
          <div class="admin-header-left">
            <span class="admin-brand-mark">cc</span>
            <div class="admin-title-stack">
              <span class="admin-title">conexiuni-cluj</span>
              <span class="admin-section">admin console</span>
            </div>
            <span class="admin-status-pill">
              <span class="admin-pulse" :class="{'admin-pulse-idle': !livePolling}"></span>
              {{ livePolling ? 'live polling' : 'manual refresh' }}
            </span>
            <span v-if="stats" class="admin-generated">
              updated {{ new Date(stats.generated_at).toLocaleTimeString() }}
            </span>
          </div>
          <div class="admin-header-right">
            <button
              type="button"
              class="admin-button admin-button-live"
              :class="{'admin-button-live-on': livePolling}"
              @click="toggleLive"
            >
              <span class="admin-live-dot"></span>
              {{ livePolling ? 'Live' : 'Go live' }}
            </button>
            <button type="button" class="admin-button" :disabled="authState === 'loading'" @click="refreshAll">
              <svg class="admin-button-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" d="M20 11a8.1 8.1 0 0 0-15.5-2M4 5v4h4m-4 4a8.1 8.1 0 0 0 15.5 2m.5 3v-4h-4"/>
              </svg>
              {{ authState === 'loading' ? '…' : 'Refresh' }}
            </button>
            <button type="button" class="admin-button-ghost" @click="logout">
              <svg class="admin-button-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4m-6-4 5-5-5-5m5 5H3"/>
              </svg>
              Logout
            </button>
          </div>
        </header>

        <div v-if="authState === 'loading' && !stats" class="admin-loading">Loading…</div>
        <div v-else-if="authState === 'error' && !stats" class="admin-error-box">
          Failed to load stats: {{ errorMessage }}
        </div>

        <template v-if="stats">
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
              <div class="admin-kpi-value">{{ formatNumber(stats.tranzy_quota.vehicles_used) }}</div>
              <div class="admin-kpi-sub">
                used today · {{ formatNumber(stats.tranzy_quota.vehicles_remaining) }} left of
                {{ formatNumber(stats.tranzy_quota.vehicles_limit) }}
              </div>
              <div class="admin-kpi-meter">
                <span :style="{width: usedPercent(stats.tranzy_quota.vehicles_used, stats.tranzy_quota.vehicles_limit)}"></span>
              </div>
            </div>
            <div class="admin-kpi">
              <div class="admin-kpi-label">Tranzy · default</div>
              <div class="admin-kpi-value">{{ formatNumber(stats.tranzy_quota.default_used) }}</div>
              <div class="admin-kpi-sub">
                used today · {{ formatNumber(stats.tranzy_quota.default_remaining) }} left of
                {{ formatNumber(stats.tranzy_quota.default_limit) }}
              </div>
              <div class="admin-kpi-meter admin-kpi-meter-alt">
                <span :style="{width: usedPercent(stats.tranzy_quota.default_used, stats.tranzy_quota.default_limit)}"></span>
              </div>
            </div>
          </section>

          <section class="admin-grid admin-grid-two">
            <div class="admin-card">
              <div class="admin-card-head">
                <h2>Warmup</h2>
                <span class="admin-card-meta">daily pass · 04:00 Europe/Bucharest</span>
              </div>
              <div v-if="warmupInfo" class="admin-warmup">
                <div class="admin-warmup-row">
                  <span class="admin-warmup-label">Next pass</span>
                  <span class="admin-warmup-value">
                    {{ formatRelative(warmupInfo.next_scheduled_at) }}
                    <span class="admin-warmup-sub">{{ formatDateTime(warmupInfo.next_scheduled_at) }}</span>
                  </span>
                </div>
                <div class="admin-warmup-row">
                  <span class="admin-warmup-label">Last completed</span>
                  <span class="admin-warmup-value">
                    {{ formatRelative(warmupInfo.last_completed_at) }}
                    <span class="admin-warmup-sub">{{ formatDateTime(warmupInfo.last_completed_at) }}</span>
                  </span>
                </div>
                <div class="admin-warmup-row">
                  <span class="admin-warmup-label">Last duration</span>
                  <span class="admin-warmup-value">
                    {{ warmupInfo.last_duration_ms ? formatDuration(warmupInfo.last_duration_ms) : '—' }}
                  </span>
                </div>
              </div>
              <div v-else class="admin-empty">No warmup recorded yet</div>
            </div>

            <div class="admin-card">
              <div class="admin-card-head">
                <h2>Cache expiration</h2>
                <span class="admin-card-meta">
                  {{ formatNumber(cacheHealthyCount) }} fresh
                  <span v-if="cacheExpiredCount" class="admin-cache-stale">· {{ formatNumber(cacheExpiredCount) }} expired</span>
                </span>
              </div>
              <div v-if="cacheGroups.length" class="admin-cache-table-wrap">
                <table class="admin-cache-table">
                  <thead>
                    <tr>
                      <th>Cache</th>
                      <th class="admin-cache-num">Entries</th>
                      <th>Expires</th>
                      <th class="admin-cache-num">Lifespan</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="g in cacheGroups" :key="g.prefix">
                      <td class="admin-cache-prefix">{{ g.prefix.toUpperCase() }}</td>
                      <td class="admin-cache-num">{{ formatNumber(g.count) }}</td>
                      <td class="admin-cache-expiry">
                        <span :class="{'admin-cache-stale': g.expired_count >= g.count}">
                          {{ formatRelative(g.earliest_expires_at) }}
                        </span>
                        <span v-if="g.count > 1" class="admin-cache-range">
                          → {{ formatRelative(g.latest_expires_at) }}
                        </span>
                        <span v-if="g.expired_count && g.expired_count < g.count" class="admin-cache-stale admin-cache-tag">
                          {{ g.expired_count }} stale
                        </span>
                      </td>
                      <td class="admin-cache-num admin-cache-life">{{ formatLifespan(g.lifespan_ms) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div v-else class="admin-empty">No cached entries</div>
            </div>

            <div class="admin-card">
              <div class="admin-card-head">
                <h2>Travel-time learning</h2>
                <span class="admin-card-meta">
                  {{ formatNumber(segmentLearning?.routes_with_profiles ?? 0) }} routes
                </span>
              </div>
              <div v-if="segmentLearning" class="admin-learning">
                <div class="admin-learning-metrics">
                  <div class="admin-learning-metric">
                    <span>Samples</span>
                    <strong>{{ formatNumber(segmentLearning.total_samples) }}</strong>
                  </div>
                  <div class="admin-learning-metric">
                    <span>Profiles</span>
                    <strong>{{ formatNumber(segmentLearning.total_profiles) }}</strong>
                  </div>
                  <div class="admin-learning-metric">
                    <span>Last sample</span>
                    <strong>{{ formatRelative(segmentLearning.last_sample_at) }}</strong>
                  </div>
                </div>

                <div v-if="segmentSnapshot?.observed_at" class="admin-learning-snapshot">
                  <div class="admin-learning-snapshot-head">
                    <span>Latest snapshot</span>
                    <strong>{{ formatRelative(segmentSnapshot.observed_at) }}</strong>
                  </div>
                  <div class="admin-learning-pills">
                    <span>{{ formatNumber(segmentSnapshot.vehicles) }} vehicles</span>
                    <span>{{ formatNumber(segmentSnapshot.accepted) }} accepted</span>
                    <span>{{ formatNumber(segmentSnapshot.stored) }} stored</span>
                    <span>{{ formatNumber(segmentSnapshotIgnored) }} ignored</span>
                    <span v-if="segmentSnapshot.sample_errors || segmentSnapshot.profile_errors" class="admin-learning-warn">
                      {{ formatNumber(segmentSnapshot.sample_errors + segmentSnapshot.profile_errors) }} errors
                    </span>
                  </div>
                </div>

                <ul v-if="segmentLearning.top_routes.length" class="admin-bars admin-learning-bars">
                  <li v-for="(route, i) in segmentLearning.top_routes" :key="route.route_id">
                    <span class="admin-bar-rank">#{{ i + 1 }}</span>
                    <span class="admin-bar-label">{{ route.route_short_name || route.route_id }}</span>
                    <span class="admin-bar-track">
                      <span class="admin-bar-fill admin-bar-fill-learning" :style="{width: segmentLearningBarPercent(route.profiles)}"></span>
                    </span>
                    <span class="admin-bar-count">{{ formatNumber(route.profiles) }} · {{ formatNumber(route.samples) }}</span>
                  </li>
                </ul>
                <div v-else class="admin-empty">No learned profiles yet</div>
              </div>
              <div v-else class="admin-empty">No learning stats yet</div>
            </div>
          </section>

          <section class="admin-card">
            <div class="admin-card-head">
              <h2>Daily unique visitors</h2>
              <div class="admin-spark-controls">
                <div class="admin-chip-group">
                  <button type="button" class="admin-chip" :class="{'admin-chip-on': sparkRangeDays === 7}" @click="sparkRangeDays = 7">7d</button>
                  <button type="button" class="admin-chip" :class="{'admin-chip-on': sparkRangeDays === 14}" @click="sparkRangeDays = 14">14d</button>
                  <button type="button" class="admin-chip" :class="{'admin-chip-on': sparkRangeDays === 30}" @click="sparkRangeDays = 30">30d</button>
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
                <div class="admin-spark-tip-date">{{ formatRoDay(hoveredPoint.v.date) }}</div>
                <div class="admin-spark-tip-count">{{ formatNumber(hoveredPoint.v.count) }} visitors</div>
              </div>
            </div>
            <div class="admin-spark-axis">
              <span>{{ formatRoDay(sparkVisits[0]?.date) }}</span>
              <span>{{ formatRoDay(sparkVisits[sparkVisits.length - 1]?.date) }}</span>
            </div>
          </section>

          <section class="admin-grid admin-grid-two">
            <div class="admin-card">
              <div class="admin-card-head">
                <h2>Tranzy quota usage</h2>
                <span class="admin-card-meta">
                  {{ recentTranzyQuota.length }} day{{ recentTranzyQuota.length === 1 ? '' : 's' }} ·
                  total {{ formatNumber(recentTranzyQuotaTotal) }}
                </span>
              </div>
              <ul v-if="recentTranzyQuota.length" class="admin-quota-bars">
                <li v-for="d in recentTranzyQuota" :key="d.date">
                  <span class="admin-quota-date">{{ formatRoDay(d.date) }}</span>
                  <span class="admin-quota-track">
                    <span
                      v-if="d.vehicles"
                      class="admin-quota-fill admin-quota-vehicles"
                      :style="{width: quotaBarPercent(d.vehicles)}"
                    ></span>
                    <span
                      v-if="d.default"
                      class="admin-quota-fill admin-quota-default"
                      :style="{width: quotaBarPercent(d.default)}"
                    ></span>
                  </span>
                  <span class="admin-bar-count">{{ formatNumber(d.total) }}</span>
                </li>
              </ul>
              <div v-else class="admin-empty">No quota usage yet</div>
              <div class="admin-legend">
                <span><i class="admin-legend-vehicles"></i>vehicles</span>
                <span><i class="admin-legend-default"></i>default</span>
              </div>
            </div>

            <div class="admin-card">
              <div class="admin-card-head">
                <h2>Slowest API averages</h2>
                <span class="admin-card-meta">{{ endpointResponseTimes.length }} endpoints</span>
              </div>
              <ul v-if="endpointResponseTimes.length" class="admin-bars admin-timing-bars">
                <li v-for="(a, i) in endpointResponseTimes" :key="a.key">
                  <span class="admin-bar-rank">#{{ i + 1 }}</span>
                  <span class="admin-bar-label admin-bar-mono">{{ a.key }}</span>
                  <span class="admin-bar-track">
                    <span class="admin-bar-fill admin-bar-fill-latency" :style="{width: latencyBarPercent(a.avg_ms)}"></span>
                  </span>
                  <span class="admin-bar-count">{{ formatMs(a.avg_ms) }} · {{ formatNumber(a.count) }}</span>
                </li>
              </ul>
              <div v-else class="admin-empty">No response-time data yet</div>
            </div>
          </section>

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
                  <span class="admin-bar-label">{{ s.key }}</span>
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

/* Warmup + cache */
.admin-warmup {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.admin-warmup-row {
  display: grid;
  grid-template-columns: 9rem 1fr;
  align-items: baseline;
  gap: 0.75rem;
  font-size: 0.88rem;
}

.admin-warmup-label {
  color: #94a3b8;
  font-size: 0.78rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.admin-warmup-value {
  color: #e6edf3;
  font-variant-numeric: tabular-nums;
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem 0.6rem;
  align-items: baseline;
}

.admin-warmup-sub {
  color: #64748b;
  font-size: 0.78rem;
}

.admin-learning {
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
}

.admin-learning-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.5rem;
}

.admin-learning-metric,
.admin-learning-snapshot {
  background: var(--admin-subtle, #0b1220);
  border: 1px solid var(--admin-border, #1f2a44);
  border-radius: 8px;
}

.admin-learning-metric {
  min-width: 0;
  padding: 0.65rem 0.7rem;
}

.admin-learning-metric span,
.admin-learning-snapshot-head span {
  display: block;
  color: var(--admin-muted, #94a3b8);
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  white-space: nowrap;
}

.admin-learning-metric strong,
.admin-learning-snapshot-head strong {
  color: var(--admin-text, #e6edf3);
  font-size: 1rem;
  font-weight: 650;
  font-variant-numeric: tabular-nums;
  line-height: 1.25;
  overflow-wrap: anywhere;
}

.admin-learning-snapshot {
  padding: 0.7rem;
}

.admin-learning-snapshot-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 0.75rem;
  margin-bottom: 0.55rem;
}

.admin-learning-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.admin-learning-pills span {
  background: var(--admin-surface, #111a2c);
  border: 1px solid var(--admin-border, #233052);
  border-radius: 999px;
  color: var(--admin-muted, #94a3b8);
  font-size: 0.74rem;
  padding: 0.2rem 0.48rem;
  font-variant-numeric: tabular-nums;
}

.admin-learning-pills .admin-learning-warn {
  border-color: rgba(239, 68, 68, 0.45);
  color: #fca5a5;
}

.admin-learning-bars li {
  grid-template-columns: 2rem minmax(5rem, 1fr) minmax(4rem, 1fr) auto;
}

.admin-bar-fill-learning {
  background: linear-gradient(90deg, #22c55e 0%, #38bdf8 100%);
}

.admin-cache-table-wrap {
  overflow-x: auto;
  overflow-y: hidden;
  -webkit-overflow-scrolling: touch;
}

.admin-cache-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.85rem;
}

.admin-cache-table th {
  text-align: left;
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.07em;
  text-transform: uppercase;
  color: #94a3b8;
  padding: 0 0.75rem 0.5rem 0;
  border-bottom: 1px solid #1f2a44;
  white-space: nowrap;
}

.admin-cache-table th:last-child,
.admin-cache-table td:last-child {
  padding-right: 0;
}

.admin-cache-table td {
  padding: 0.45rem 0.75rem 0.45rem 0;
  border-bottom: 1px dashed #1f2a44;
  white-space: nowrap;
  vertical-align: baseline;
}

.admin-cache-table tbody tr:last-child td {
  border-bottom: none;
}

.admin-cache-num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.admin-cache-prefix {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 0.78rem;
  color: #cbd5e1;
  letter-spacing: 0.02em;
}

.admin-cache-expiry {
  color: #e6edf3;
  font-variant-numeric: tabular-nums;
}

.admin-cache-range {
  color: #64748b;
  font-size: 0.78rem;
  margin-left: 0.25rem;
}

.admin-cache-tag {
  display: inline-block;
  margin-left: 0.4rem;
  padding: 0.05rem 0.4rem;
  border-radius: 4px;
  background: rgba(252, 165, 165, 0.12);
  font-size: 0.72rem;
}

.admin-cache-life {
  color: #64748b;
  font-size: 0.78rem;
}

.admin-cache-stale {
  color: #fca5a5;
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

.admin-grid-two {
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 360px), 1fr));
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

.admin-bar-fill-latency {
  background: linear-gradient(90deg, #f59e0b 0%, #ef4444 100%);
}

.admin-bar-count {
  font-variant-numeric: tabular-nums;
  color: #e6edf3;
  font-weight: 500;
  white-space: nowrap;
}

.admin-timing-bars li {
  grid-template-columns: 2rem minmax(0, 1.2fr) minmax(4rem, 1fr) auto;
}

.admin-quota-bars {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.admin-quota-bars li {
  display: grid;
  grid-template-columns: 3.2rem minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.6rem;
  font-size: 0.85rem;
}

.admin-quota-date {
  color: #94a3b8;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 0.78rem;
  font-variant-numeric: tabular-nums;
}

.admin-quota-track {
  height: 10px;
  background: #0b1220;
  border-radius: 5px;
  overflow: hidden;
  display: flex;
}

.admin-quota-fill {
  display: block;
  height: 100%;
  min-width: 2px;
}

.admin-quota-vehicles {
  background: #38bdf8;
}

.admin-quota-default {
  background: #a78bfa;
}

.admin-legend {
  display: flex;
  align-items: center;
  gap: 0.85rem;
  margin-top: 0.85rem;
  color: #64748b;
  font-size: 0.78rem;
}

.admin-legend span {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}

.admin-legend i {
  width: 0.65rem;
  height: 0.65rem;
  border-radius: 2px;
  display: inline-block;
}

.admin-legend-vehicles {
  background: #38bdf8;
}

.admin-legend-default {
  background: #a78bfa;
}

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

.admin-shell {
  --admin-bg: #f8fafc;
  --admin-bg-soft: #eef2f7;
  --admin-surface: rgba(255, 255, 255, 0.94);
  --admin-surface-strong: #ffffff;
  --admin-subtle: rgba(241, 245, 249, 0.82);
  --admin-input: #ffffff;
  --admin-border: rgba(203, 213, 225, 0.74);
  --admin-border-strong: #94a3b8;
  --admin-text: #0f172a;
  --admin-muted: #64748b;
  --admin-faint: #94a3b8;
  --admin-track: #e2e8f0;
  --admin-terminal: #08111f;
  --admin-accent: #0284c7;
  --admin-accent-soft: rgba(14, 165, 233, 0.13);
  --admin-focus: rgba(14, 165, 233, 0.2);
  --admin-radius: 0.875rem;
  --admin-radius-sm: 0.625rem;
  --admin-shadow: 0 18px 48px -34px rgba(15, 23, 42, 0.42);
  background:
    radial-gradient(circle at top left, rgba(14, 165, 233, 0.16), transparent 28rem),
    linear-gradient(180deg, var(--admin-bg-soft) 0%, var(--admin-bg) 18rem);
  color: var(--admin-text);
  font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
}

.admin-shell.is-dark {
  --admin-bg: #0f172a;
  --admin-bg-soft: #111827;
  --admin-surface: rgba(15, 23, 42, 0.92);
  --admin-surface-strong: #111827;
  --admin-subtle: rgba(30, 41, 59, 0.58);
  --admin-input: #0b1220;
  --admin-border: rgba(51, 65, 85, 0.72);
  --admin-border-strong: #475569;
  --admin-text: #f1f5f9;
  --admin-muted: #94a3b8;
  --admin-faint: #64748b;
  --admin-track: #070e1c;
  --admin-terminal: #020817;
  --admin-accent: #38bdf8;
  --admin-accent-soft: rgba(56, 189, 248, 0.13);
  --admin-focus: rgba(56, 189, 248, 0.2);
  --admin-shadow: 0 20px 54px -36px rgba(0, 0, 0, 0.78);
  background:
    radial-gradient(circle at top left, rgba(14, 165, 233, 0.18), transparent 28rem),
    linear-gradient(180deg, var(--admin-bg-soft) 0%, var(--admin-bg) 18rem);
}

.admin-shell.is-arcade {
  --admin-accent: #f59e0b;
  --admin-accent-soft: rgba(245, 158, 11, 0.14);
  --admin-focus: rgba(245, 158, 11, 0.2);
}

.admin-shell.is-legacy-blue {
  --admin-accent: #245edc;
  --admin-accent-soft: rgba(36, 94, 220, 0.14);
  --admin-focus: rgba(36, 94, 220, 0.2);
  --admin-radius: 0.375rem;
  --admin-radius-sm: 0.25rem;
  font-family: Tahoma, Verdana, ui-sans-serif, system-ui, sans-serif;
}

.admin-dash {
  width: min(100%, 1500px);
  max-width: none;
  padding: clamp(0.875rem, 2vw, 1.5rem) clamp(0.875rem, 3vw, 2rem) 3rem;
}

.admin-header {
  align-items: center;
  padding: 0.85rem;
  margin-bottom: 1rem;
  border: 1px solid var(--admin-border);
  border-radius: var(--admin-radius);
  background: var(--admin-surface);
  box-shadow: var(--admin-shadow);
}

.admin-header-left {
  min-width: 0;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.admin-header-right {
  align-items: center;
  flex-wrap: wrap;
}

.admin-brand-mark,
.admin-login-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.35rem;
  height: 2.35rem;
  flex: 0 0 auto;
  border-radius: 0.8rem;
  background: linear-gradient(135deg, var(--admin-accent), #10b981);
  color: #ffffff;
  font-size: 0.8rem;
  font-weight: 900;
  letter-spacing: 0;
  text-transform: uppercase;
  box-shadow: 0 8px 18px -12px var(--admin-accent);
}

.admin-shell.is-legacy-blue .admin-brand-mark,
.admin-shell.is-legacy-blue .admin-login-mark {
  border-radius: 0.25rem;
  background: linear-gradient(180deg, #4b8cf7, #245edc 48%, #1941a5);
  box-shadow: inset 1px 1px 0 rgba(255, 255, 255, 0.55);
}

.admin-title-stack {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  min-width: 0;
}

.admin-title {
  color: var(--admin-text);
  font-size: 0.98rem;
  font-weight: 800;
  line-height: 1.05;
  letter-spacing: 0;
}

.admin-section {
  color: var(--admin-muted);
  font-size: 0.68rem;
  font-weight: 700;
  line-height: 1.1;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.admin-status-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  min-height: 1.8rem;
  padding: 0.25rem 0.6rem;
  border: 1px solid var(--admin-border);
  border-radius: 999px;
  background: var(--admin-subtle);
  color: var(--admin-muted);
  font-size: 0.74rem;
  font-weight: 700;
  white-space: nowrap;
}

.admin-generated {
  margin-left: 0;
  color: var(--admin-faint);
  font-size: 0.78rem;
  font-variant-numeric: tabular-nums;
}

.admin-login-card,
.admin-card,
.admin-kpi {
  background: var(--admin-surface);
  border-color: var(--admin-border);
  border-radius: var(--admin-radius);
  box-shadow: var(--admin-shadow);
}

.admin-login {
  background:
    radial-gradient(circle at 50% 28%, var(--admin-accent-soft), transparent 22rem),
    transparent;
}

.admin-login-card {
  max-width: 23rem;
  padding: 1.5rem;
  gap: 0.85rem;
}

.admin-login-title {
  color: var(--admin-text);
  font-size: 1.1rem;
  font-weight: 800;
  letter-spacing: 0;
}

.admin-login-sub {
  color: var(--admin-muted);
  margin: 0.15rem 0 0.25rem;
}

.admin-button-primary,
.admin-button,
.admin-button-ghost {
  min-height: 2.25rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.42rem;
  border-radius: var(--admin-radius-sm);
  font-size: 0.82rem;
  line-height: 1;
}

.admin-button-icon {
  width: 0.95rem;
  height: 0.95rem;
  flex: 0 0 auto;
}

.admin-button-primary {
  background: var(--admin-accent);
  border-color: var(--admin-accent);
  color: #ffffff;
  font-weight: 800;
}

.admin-button-primary:hover {
  background: #0ea5e9;
  border-color: #0ea5e9;
}

.admin-button {
  background: var(--admin-subtle);
  border-color: var(--admin-border);
  color: var(--admin-text);
}

.admin-button:hover:not(:disabled) {
  background: var(--admin-accent-soft);
  border-color: var(--admin-accent);
}

.admin-button-ghost {
  color: var(--admin-muted);
}

.admin-button-ghost:hover {
  background: var(--admin-subtle);
  color: var(--admin-text);
}

.admin-button-live-on {
  background: rgba(16, 185, 129, 0.12);
  border-color: rgba(16, 185, 129, 0.42);
  color: #059669;
}

.admin-shell.is-dark .admin-button-live-on {
  color: #86efac;
}

.admin-input,
.admin-select {
  background: var(--admin-input);
  border-color: var(--admin-border);
  color: var(--admin-text);
  border-radius: var(--admin-radius-sm);
}

.admin-input::placeholder {
  color: var(--admin-faint);
}

.admin-input:focus,
.admin-select:focus {
  border-color: var(--admin-accent);
  box-shadow: 0 0 0 3px var(--admin-focus);
}

.admin-kpis {
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 9.75rem), 1fr));
  gap: 0.75rem;
  margin-bottom: 1rem;
}

.admin-kpi {
  --kpi-accent: var(--admin-accent);
  position: relative;
  min-height: 8.1rem;
  overflow: hidden;
  padding: 0.95rem;
}

.admin-kpi:nth-child(2) { --kpi-accent: #10b981; }
.admin-kpi:nth-child(3) { --kpi-accent: #06b6d4; }
.admin-kpi:nth-child(4) { --kpi-accent: #8b5cf6; }
.admin-kpi:nth-child(5) { --kpi-accent: #f43f5e; }
.admin-kpi:nth-child(6) { --kpi-accent: #0ea5e9; }
.admin-kpi:nth-child(7) { --kpi-accent: #a855f7; }

.admin-kpi::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 3px;
  background: var(--kpi-accent);
}

.admin-kpi:hover {
  border-color: var(--admin-border-strong);
  transform: translateY(-1px);
}

.admin-kpi-accent {
  background:
    linear-gradient(135deg, var(--admin-accent-soft), transparent 72%),
    var(--admin-surface);
  border-color: color-mix(in srgb, var(--admin-accent) 45%, var(--admin-border));
}

.admin-kpi-label {
  color: var(--admin-muted);
  font-size: 0.68rem;
  font-weight: 800;
  letter-spacing: 0.08em;
}

.admin-kpi-value {
  color: var(--admin-text);
  font-size: clamp(1.55rem, 2.1vw, 2.15rem);
  font-weight: 800;
}

.admin-kpi-accent .admin-kpi-value {
  color: var(--admin-accent);
}

.admin-kpi-sub {
  color: var(--admin-muted);
  font-size: 0.76rem;
  line-height: 1.35;
}

.admin-kpi-meter {
  height: 0.35rem;
  margin-top: auto;
  overflow: hidden;
  border-radius: 999px;
  background: var(--admin-track);
}

.admin-kpi-meter span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, #38bdf8, #10b981);
  transition: width 0.35s ease;
}

.admin-kpi-meter-alt span {
  background: linear-gradient(90deg, #a78bfa, #f472b6);
}

.admin-card {
  padding: clamp(1rem, 1.35vw, 1.25rem);
  margin-bottom: 1rem;
}

.admin-card-head {
  align-items: center;
  margin-bottom: 0.9rem;
}

.admin-card-head h2 {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  color: var(--admin-text);
  font-size: 0.95rem;
  font-weight: 800;
}

.admin-card-head h2::before {
  content: '';
  width: 0.55rem;
  height: 0.55rem;
  border-radius: 999px;
  background: var(--admin-accent);
  box-shadow: 0 0 0 3px var(--admin-accent-soft);
}

.admin-card-meta,
.admin-empty {
  color: var(--admin-faint);
}

.admin-spark {
  color: var(--admin-accent);
  padding: 0.35rem 0.25rem 0;
  border: 1px solid var(--admin-border);
  border-radius: var(--admin-radius-sm);
  background:
    linear-gradient(to right, transparent 0, transparent calc(100% - 1px), var(--admin-border) calc(100% - 1px)),
    var(--admin-subtle);
}

.admin-spark-svg {
  height: clamp(132px, 12vw, 174px);
}

.admin-spark-tip {
  background: var(--admin-surface-strong);
  border-color: var(--admin-border);
  color: var(--admin-text);
  box-shadow: var(--admin-shadow);
}

.admin-spark-axis {
  color: var(--admin-faint);
}

.admin-grid {
  align-items: start;
  gap: 1rem;
  margin-bottom: 1rem;
}

.admin-grid .admin-card {
  height: auto;
}

.admin-bars {
  gap: 0.42rem;
}

.admin-bars li,
.admin-quota-bars li {
  min-height: 1.75rem;
  color: var(--admin-text);
}

.admin-bar-rank {
  color: var(--admin-faint);
}

.admin-bar-label,
.admin-bar-count {
  color: var(--admin-text);
}

.admin-bar-mono,
.admin-quota-date {
  color: var(--admin-muted);
}

.admin-bar-track,
.admin-quota-track {
  background: var(--admin-track);
}

.admin-bar-track {
  height: 0.46rem;
}

.admin-bar-fill {
  background: linear-gradient(90deg, #38bdf8, #60a5fa);
}

.admin-bar-fill-latency {
  background: linear-gradient(90deg, #f59e0b, #ef4444);
}

.admin-quota-track {
  height: 0.65rem;
}

.admin-legend {
  color: var(--admin-muted);
}

.admin-chip {
  min-height: 1.72rem;
  border-color: var(--admin-border);
  border-radius: var(--admin-radius-sm);
  color: var(--admin-muted);
}

.admin-chip:hover {
  border-color: var(--admin-border-strong);
  color: var(--admin-text);
}

.admin-chip-on {
  background: var(--admin-accent-soft);
  border-color: var(--admin-accent);
  color: var(--admin-text);
}

.admin-warmup-label {
  color: var(--admin-muted);
}

.admin-warmup-value {
  color: var(--admin-text);
}

.admin-warmup-sub {
  color: var(--admin-faint);
}

.admin-cache-table th {
  color: var(--admin-muted);
  border-bottom-color: var(--admin-border);
}

.admin-cache-table td {
  border-bottom-color: var(--admin-border);
}

.admin-cache-prefix {
  color: var(--admin-text);
}

.admin-cache-expiry {
  color: var(--admin-text);
}

.admin-cache-range,
.admin-cache-life {
  color: var(--admin-faint);
}

.admin-chip-n,
.admin-chip-on .admin-chip-n {
  color: var(--admin-faint);
}

.admin-delta {
  border-radius: var(--admin-radius-sm);
}

.admin-error-box {
  border-color: rgba(239, 68, 68, 0.42);
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
}

.admin-shell.is-arcade {
  --admin-bg: #fef3c7;
  --admin-bg-soft: #facc15;
  --admin-surface: #fffbeb;
  --admin-surface-strong: #fefce8;
  --admin-subtle: #fef9c3;
  --admin-input: #fff7ed;
  --admin-border: #f59e0b;
  --admin-border-strong: #78350f;
  --admin-text: #451a03;
  --admin-muted: #92400e;
  --admin-faint: #b45309;
  --admin-track: #451a03;
  --admin-terminal: #1c1608;
  --admin-accent: #f59e0b;
  --admin-accent-soft: rgba(245, 158, 11, 0.2);
  --admin-focus: rgba(245, 158, 11, 0.28);
  --admin-radius: 0.35rem;
  --admin-radius-sm: 0.18rem;
  --admin-shadow: 4px 4px 0 rgba(146, 64, 14, 0.42);
  background:
    repeating-linear-gradient(45deg, rgba(120, 53, 15, 0.08) 0 1px, transparent 1px 10px),
    linear-gradient(180deg, #facc15 0, #fef3c7 15rem, #fefce8 100%);
  font-family: 'Trebuchet MS', ui-sans-serif, system-ui, sans-serif;
}

.admin-shell.is-dark.is-arcade {
  --admin-bg: #1c1608;
  --admin-bg-soft: #211a05;
  --admin-surface: #211a05;
  --admin-surface-strong: #2a2006;
  --admin-subtle: #2a2006;
  --admin-input: #1c1608;
  --admin-border: #b45309;
  --admin-border-strong: #f59e0b;
  --admin-text: #fde68a;
  --admin-muted: #fbbf24;
  --admin-faint: #d97706;
  --admin-track: #050403;
  --admin-terminal: #090602;
  --admin-accent: #fbbf24;
  --admin-accent-soft: rgba(217, 119, 6, 0.24);
  --admin-focus: rgba(251, 191, 36, 0.24);
  --admin-shadow: 4px 4px 0 rgba(0, 0, 0, 0.56);
  background:
    repeating-linear-gradient(45deg, rgba(251, 191, 36, 0.07) 0 1px, transparent 1px 10px),
    linear-gradient(180deg, #2a2006 0, #1c1608 15rem, #130f05 100%);
}

.admin-shell.is-arcade::before {
  content: '';
  position: fixed;
  inset: 0;
  pointer-events: none;
  background: repeating-linear-gradient(to bottom, rgba(255, 255, 255, 0.1) 0 1px, transparent 1px 5px);
  opacity: 0.32;
}

.admin-shell.is-dark.is-arcade::before {
  background: repeating-linear-gradient(to bottom, rgba(251, 191, 36, 0.1) 0 1px, transparent 1px 5px);
  opacity: 0.28;
}

.admin-shell.is-arcade .admin-header {
  border: 2px solid #451a03;
  background: linear-gradient(180deg, #fde047, #facc15 55%, #f59e0b);
  box-shadow: 4px 4px 0 #92400e;
}

.admin-shell.is-dark.is-arcade .admin-header {
  border-color: #f59e0b;
  background: linear-gradient(180deg, #422006, #2a2006 55%, #1c1608);
  box-shadow: 4px 4px 0 #050403;
}

.admin-shell.is-arcade .admin-header::after {
  content: '';
  flex-basis: 100%;
  height: 0.32rem;
  background: repeating-linear-gradient(90deg, #ef4444 0 1rem, #facc15 1rem 2rem, #22c55e 2rem 3rem, #38bdf8 3rem 4rem);
  border: 1px solid rgba(69, 26, 3, 0.4);
}

.admin-shell.is-arcade .admin-brand-mark,
.admin-shell.is-arcade .admin-login-mark {
  position: relative;
  overflow: hidden;
  border: 2px solid #451a03;
  border-radius: 999px;
  background: #facc15;
  color: transparent;
  box-shadow: inset -5px -5px 0 #f59e0b, 3px 3px 0 #451a03;
}

.admin-shell.is-arcade .admin-brand-mark::after,
.admin-shell.is-arcade .admin-login-mark::after {
  content: '';
  position: absolute;
  right: -2px;
  top: 50%;
  width: 58%;
  height: 58%;
  background: #451a03;
  clip-path: polygon(100% 0, 0 50%, 100% 100%);
  transform: translateY(-50%);
}

.admin-shell.is-arcade .admin-login-card,
.admin-shell.is-arcade .admin-card,
.admin-shell.is-arcade .admin-kpi {
  border: 2px solid var(--admin-border);
  box-shadow: var(--admin-shadow);
}

.admin-shell.is-arcade .admin-kpi::before {
  height: 0.34rem;
  background: repeating-linear-gradient(90deg, var(--kpi-accent) 0 0.9rem, #451a03 0.9rem 1.15rem);
}

.admin-shell.is-arcade .admin-kpi-value,
.admin-shell.is-arcade .admin-card-head h2,
.admin-shell.is-arcade .admin-title {
  font-weight: 900;
  text-shadow: 1px 1px 0 rgba(255, 255, 255, 0.55);
}

.admin-shell.is-dark.is-arcade .admin-kpi-value,
.admin-shell.is-dark.is-arcade .admin-card-head h2,
.admin-shell.is-dark.is-arcade .admin-title {
  text-shadow: 1px 1px 0 #000000;
}

.admin-shell.is-arcade .admin-card-head h2::before {
  width: 0.72rem;
  height: 0.72rem;
  border-radius: 0;
  box-shadow: 0 0 0 2px #fde68a;
}

.admin-shell.is-arcade .admin-button-primary,
.admin-shell.is-arcade .admin-button,
.admin-shell.is-arcade .admin-button-ghost,
.admin-shell.is-arcade .admin-chip {
  border: 2px solid #451a03;
  box-shadow: 2px 2px 0 rgba(69, 26, 3, 0.42);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.admin-shell.is-arcade .admin-button-primary,
.admin-shell.is-arcade .admin-chip-on {
  background: #facc15;
  color: #451a03;
}

.admin-shell.is-dark.is-arcade .admin-button-primary,
.admin-shell.is-dark.is-arcade .admin-chip-on {
  background: #854d0e;
  color: #fde68a;
}

.admin-shell.is-arcade .admin-input,
.admin-shell.is-arcade .admin-select,
.admin-shell.is-arcade .admin-spark {
  border: 2px solid var(--admin-border);
  box-shadow: inset 2px 2px 0 rgba(146, 64, 14, 0.16);
}

.admin-shell.is-arcade .admin-bar-track,
.admin-shell.is-arcade .admin-quota-track,
.admin-shell.is-arcade .admin-kpi-meter {
  border: 1px solid #451a03;
  border-radius: 0;
}

.admin-shell.is-arcade .admin-bar-fill,
.admin-shell.is-arcade .admin-quota-vehicles,
.admin-shell.is-arcade .admin-kpi-meter span {
  background: repeating-linear-gradient(90deg, #38bdf8 0 0.9rem, #60a5fa 0.9rem 1.8rem);
}

.admin-shell.is-arcade .admin-bar-fill-latency {
  background: repeating-linear-gradient(90deg, #f59e0b 0 0.9rem, #ef4444 0.9rem 1.8rem);
}

.admin-shell.is-arcade .admin-bar-fill-learning {
  background: repeating-linear-gradient(90deg, #22c55e 0 0.9rem, #38bdf8 0.9rem 1.8rem);
}

.admin-shell.is-arcade .admin-quota-default,
.admin-shell.is-arcade .admin-kpi-meter-alt span {
  background: repeating-linear-gradient(90deg, #a78bfa 0 0.9rem, #f472b6 0.9rem 1.8rem);
}

.admin-shell.is-legacy-blue {
  --admin-bg: var(--xp-tan, #ece9d8);
  --admin-bg-soft: #ffffff;
  --admin-surface: #ffffff;
  --admin-surface-strong: #ffffff;
  --admin-subtle: var(--xp-tan, #ece9d8);
  --admin-input: #ffffff;
  --admin-border: var(--xp-border, #aca899);
  --admin-border-strong: var(--xp-blue, #245edc);
  --admin-text: var(--xp-text, #000000);
  --admin-muted: #3b4d63;
  --admin-faint: #606060;
  --admin-track: #d4d0c8;
  --admin-terminal: #000000;
  --admin-accent: var(--xp-blue, #245edc);
  --admin-accent-soft: rgba(36, 94, 220, 0.16);
  --admin-focus: rgba(36, 94, 220, 0.25);
  --admin-radius: 0;
  --admin-radius-sm: 0;
  --admin-shadow: inset 1px 1px 0 #ffffff, 1px 1px 0 rgba(0, 0, 0, 0.18);
  background: var(--xp-bliss, linear-gradient(to bottom, #4a86c8 0%, #8fb3dc 30%, #b8c8b0 46%, #84b348 56%, #4e8c2e 100%));
  font-family: var(--xp-font, Tahoma, Verdana, sans-serif);
}

.admin-shell.is-dark.is-legacy-blue {
  --admin-surface: #1a2030;
  --admin-surface-strong: #22283a;
  --admin-subtle: #2a2d38;
  --admin-input: #0a1020;
  --admin-muted: #8898b0;
  --admin-faint: #708098;
  --admin-track: #14182a;
  --admin-terminal: #050914;
  --admin-shadow: inset 1px 1px 0 rgba(255, 255, 255, 0.05), 1px 1px 0 rgba(0, 0, 0, 0.48);
}

.admin-shell.is-legacy-blue .admin-header {
  position: relative;
  align-items: center;
  padding: 2.25rem 0.65rem 0.65rem;
  border: 1px solid #003c9c;
  border-radius: 0;
  background: var(--xp-tan, #ece9d8);
  box-shadow: inset 1px 1px 0 #ffffff, 3px 3px 10px rgba(0, 0, 0, 0.28);
}

.admin-shell.is-dark.is-legacy-blue .admin-header {
  border-color: #001e5c;
  background: #2a2d38;
  box-shadow: inset 1px 1px 0 rgba(255, 255, 255, 0.06), 3px 3px 12px rgba(0, 0, 0, 0.55);
}

.admin-shell.is-legacy-blue .admin-header::before {
  content: '🚌  conexiuni-cluj · admin';
  position: absolute;
  top: 1px;
  left: 1px;
  right: 1px;
  height: 1.7rem;
  display: flex;
  align-items: center;
  padding: 0 0.55rem;
  background: linear-gradient(to bottom, #0058da 0%, #2e84e8 6%, #1a6cd0 14%, #1056c0 50%, #0e54be 51%, #1a66d0 95%, #0e4dac 100%);
  color: #ffffff;
  font-size: 12px;
  font-weight: 700;
  text-shadow: 1px 1px 1px rgba(0, 0, 0, 0.45);
}

.admin-shell.is-dark.is-legacy-blue .admin-header::before {
  background: linear-gradient(to bottom, #003478 0%, #1a6cd0 8%, #0f4fa8 50%, #0a3e90 51%, #1656b8 95%, #062a6c 100%);
}

.admin-shell.is-legacy-blue .admin-brand-mark,
.admin-shell.is-legacy-blue .admin-login-mark {
  position: relative;
  border: 1px solid #003c9c;
  border-radius: 0;
  background: var(--xp-orb, radial-gradient(circle at 30% 30%, #6ba1f0, #245edc 55%, #1a4fb8 100%));
  color: transparent;
  box-shadow: inset 0 1px 1px rgba(255, 255, 255, 0.55), inset 0 -2px 4px rgba(0, 0, 0, 0.2);
}

.admin-shell.is-legacy-blue .admin-brand-mark::before,
.admin-shell.is-legacy-blue .admin-login-mark::before {
  content: '🚌';
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  color: #ffffff;
  font-size: 1.05rem;
  text-shadow: 1px 1px 1px rgba(0, 0, 0, 0.55);
}

.admin-shell.is-legacy-blue .admin-login-card,
.admin-shell.is-legacy-blue .admin-card,
.admin-shell.is-legacy-blue .admin-kpi,
.admin-shell.is-legacy-blue .admin-spark {
  border: 1px solid var(--admin-border);
  border-radius: 0;
  background: var(--admin-surface);
  box-shadow: var(--admin-shadow);
}

.admin-shell.is-legacy-blue .admin-login-card {
  padding-top: 2.35rem;
  position: relative;
}

.admin-shell.is-legacy-blue .admin-login-card::before {
  content: 'Admin Login';
  position: absolute;
  top: 1px;
  left: 1px;
  right: 1px;
  height: 1.55rem;
  display: flex;
  align-items: center;
  padding-left: 0.5rem;
  background: linear-gradient(to bottom, #0058da 0%, #2e84e8 8%, #1056c0 50%, #0e54be 51%, #0e4dac 100%);
  color: #ffffff;
  font-size: 12px;
  font-weight: 700;
  text-shadow: 1px 1px 1px rgba(0, 0, 0, 0.45);
}

.admin-shell.is-legacy-blue .admin-kpi {
  padding-top: 1.95rem;
}

.admin-shell.is-legacy-blue .admin-kpi::before {
  height: 1.35rem;
  background: linear-gradient(to bottom, #4a88e8 0%, #245edc 50%, #1a52b8 100%);
  border-bottom: 1px solid #003c9c;
}

.admin-shell.is-dark.is-legacy-blue .admin-kpi::before {
  background: linear-gradient(to bottom, #4a88d8 0%, #2a66b8 50%, #1b4f90 100%);
}

.admin-shell.is-legacy-blue .admin-card-head {
  margin: -0.35rem -0.35rem 0.9rem;
  padding: 0.35rem 0.45rem;
  background: linear-gradient(to bottom, #ffffff 0%, var(--xp-tan, #ece9d8) 100%);
  border: 1px solid var(--admin-border);
  box-shadow: inset 1px 1px 0 #ffffff;
}

.admin-shell.is-dark.is-legacy-blue .admin-card-head {
  background: linear-gradient(to bottom, #2a3045 0%, #1a2030 100%);
  box-shadow: inset 1px 1px 0 rgba(255, 255, 255, 0.06);
}

.admin-shell.is-legacy-blue .admin-card-head h2 {
  color: var(--xp-blue, #245edc);
  font-family: var(--xp-font, Tahoma, Verdana, sans-serif);
  font-size: 0.86rem;
  letter-spacing: 0.02em;
}

.admin-shell.is-legacy-blue .admin-card-head h2::before {
  width: 0.65rem;
  height: 0.65rem;
  border-radius: 0;
  background: var(--xp-blue, #245edc);
  box-shadow: inset 1px 1px 0 rgba(255, 255, 255, 0.45);
}

.admin-shell.is-legacy-blue .admin-title,
.admin-shell.is-legacy-blue .admin-section,
.admin-shell.is-legacy-blue .admin-kpi-label,
.admin-shell.is-legacy-blue .admin-kpi-value,
.admin-shell.is-legacy-blue .admin-kpi-sub {
  font-family: var(--xp-font, Tahoma, Verdana, sans-serif);
}

.admin-shell.is-legacy-blue .admin-button-primary,
.admin-shell.is-legacy-blue .admin-button,
.admin-shell.is-legacy-blue .admin-button-ghost,
.admin-shell.is-legacy-blue .admin-chip {
  border: 1px solid var(--xp-border, #aca899);
  border-radius: 0;
  background: var(--xp-btn, linear-gradient(to bottom, #fdfdfb 0%, #ece9d8 50%, #d6d2c0 100%));
  color: var(--xp-text, #000000);
  box-shadow: inset 1px 1px 0 rgba(255, 255, 255, 0.75), inset -1px -1px 0 rgba(64, 64, 64, 0.35);
  font-family: var(--xp-font, Tahoma, Verdana, sans-serif);
  font-size: 12px;
  font-weight: 400;
}

.admin-shell.is-legacy-blue .admin-button-primary,
.admin-shell.is-legacy-blue .admin-chip-on {
  background: var(--xp-btn-active, linear-gradient(to bottom, #4a90e0 0%, #2470d4 50%, #1a52b8 100%));
  border-color: #003c9c;
  color: #ffffff;
  text-shadow: 1px 1px 1px rgba(0, 0, 0, 0.45);
}

.admin-shell.is-legacy-blue .admin-button:hover:not(:disabled),
.admin-shell.is-legacy-blue .admin-button-ghost:hover,
.admin-shell.is-legacy-blue .admin-chip:hover {
  background: var(--xp-btn-hover, linear-gradient(to bottom, #fff5c8 0%, #ffe07a 50%, #f3c94e 100%));
  border-color: #d08020;
  color: #000000;
  text-shadow: none;
}

.admin-shell.is-legacy-blue .admin-button-live-on {
  background: var(--xp-live, linear-gradient(to bottom, #6fb452 0%, #5baa38 50%, #3d7e22 100%));
  border-color: #3d7e22;
  color: #ffffff;
}

.admin-shell.is-legacy-blue .admin-status-pill,
.admin-shell.is-legacy-blue .admin-input,
.admin-shell.is-legacy-blue .admin-select {
  border: 1px solid #7f9db9;
  border-radius: 0;
  background: var(--admin-input);
  color: var(--admin-text);
  box-shadow: inset 1px 1px 0 rgba(0, 0, 0, 0.18), inset -1px -1px 0 rgba(255, 255, 255, 0.55);
  font-family: var(--xp-font, Tahoma, Verdana, sans-serif);
}

.admin-shell.is-legacy-blue .admin-bar-track,
.admin-shell.is-legacy-blue .admin-quota-track,
.admin-shell.is-legacy-blue .admin-kpi-meter {
  border: 1px solid #808080;
  border-radius: 0;
  background: var(--admin-track);
  box-shadow: inset 1px 1px 0 rgba(0, 0, 0, 0.22);
}

.admin-shell.is-legacy-blue .admin-bar-fill,
.admin-shell.is-legacy-blue .admin-quota-vehicles,
.admin-shell.is-legacy-blue .admin-kpi-meter span {
  background: linear-gradient(to bottom, #6ba1f0 0%, #245edc 55%, #1a4fb8 100%);
}

.admin-shell.is-legacy-blue .admin-bar-fill-latency {
  background: linear-gradient(to bottom, #ffcf63 0%, #f3a536 50%, #d56b18 100%);
}

.admin-shell.is-legacy-blue .admin-bar-fill-learning {
  background: linear-gradient(to bottom, #72d56f 0%, #239247 55%, #12652f 100%);
}

.admin-shell.is-legacy-blue .admin-quota-default,
.admin-shell.is-legacy-blue .admin-kpi-meter-alt span {
  background: linear-gradient(to bottom, #c4b5fd 0%, #7c3aed 55%, #4c1d95 100%);
}

@media (max-width: 760px) {
  .admin-dash {
    padding: 0.75rem 0.75rem 2rem;
  }

  .admin-header {
    align-items: stretch;
  }

  .admin-header-left,
  .admin-header-right {
    width: 100%;
  }

  .admin-header-right .admin-button,
  .admin-header-right .admin-button-ghost {
    flex: 1 1 auto;
  }

  .admin-status-pill,
  .admin-generated {
    order: 3;
  }

  .admin-kpis {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .admin-kpi {
    min-height: 7.4rem;
  }

  .admin-card-head,
  .admin-top-controls,
  .admin-spark-controls {
    align-items: stretch;
    width: 100%;
  }

  .admin-top-controls .admin-input-sm {
    flex: 1 1 9rem;
    width: auto;
  }

  .admin-bars li {
    grid-template-columns: 1.65rem minmax(0, 1fr) auto;
    grid-template-areas:
      "rank label count"
      ".    track track";
    row-gap: 0.35rem;
  }

  .admin-bars .admin-bar-rank {
    grid-area: rank;
    align-self: start;
  }

  .admin-bars .admin-bar-label {
    grid-area: label;
  }

  .admin-bars .admin-bar-track {
    grid-area: track;
  }

  .admin-bars .admin-bar-count {
    grid-area: count;
    align-self: start;
  }
}

@media (max-width: 420px) {
  .admin-kpis {
    grid-template-columns: 1fr;
  }

  .admin-quota-bars li {
    grid-template-columns: 3rem minmax(0, 1fr);
  }

  .admin-quota-bars .admin-bar-count {
    grid-column: 2;
  }
}
</style>
