import {onBeforeUnmount, onMounted, ref} from 'vue'
import type {Overview, Site} from '@/components/admin/types.ts'
import {currentCity} from '@/utils/city.ts'

const REFRESH_MS = 30_000
const AUTH_FLAG_KEY = 'admin:authed'
const LINE_STATS_KEY = 'admin:line-stats'

const readFlag = (key: string) => {
  try {
    return localStorage.getItem(key) === '1'
  } catch {
    return false
  }
}
const writeFlag = (key: string, on: boolean) => {
  try {
    if (on) localStorage.setItem(key, '1')
    else localStorage.removeItem(key)
  } catch {
    /* storage blocked */
  }
}

export const adminAuthed = () => readFlag(AUTH_FLAG_KEY)
export const writeAuthFlag = (on: boolean) => writeFlag(AUTH_FLAG_KEY, on)
export const lineStatsOn = () => readFlag(LINE_STATS_KEY)
export const setLineStats = (on: boolean) => writeFlag(LINE_STATS_KEY, on)

export class SignedOut extends Error {}

export async function getJSON<T>(url: string): Promise<T> {
  const res = await fetch(url, {credentials: 'same-origin'})
  if (res.status === 401) throw new SignedOut()
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return (await res.json()) as T
}

export function useAdmin() {
  const state = ref<'login' | 'loading' | 'ready'>(adminAuthed() ? 'loading' : 'login')
  const site = ref<Site | null>(null)
  const overview = ref<Overview | null>(null)
  const city = ref(currentCity.value)
  const loading = ref(false)
  const loadError = ref('')
  const clock = ref(Date.now())
  let refreshTimer: ReturnType<typeof setInterval> | null = null
  let clockTimer: ReturnType<typeof setInterval> | null = null

  function signedOut() {
    writeAuthFlag(false)
    site.value = null
    overview.value = null
    state.value = 'login'
  }

  function fail(err: unknown) {
    if (err instanceof SignedOut) return signedOut()
    loadError.value = (err as Error).message
    if (state.value === 'loading') state.value = 'ready'
  }

  async function loadCity() {
    const slug = city.value
    const next = await getJSON<Overview>(`/api/admin/overview?city=${slug}`)
    if (slug === city.value) overview.value = next
  }

  async function load() {
    if (loading.value) return
    loading.value = true
    try {
      const next = await getJSON<Site>('/api/admin/site')
      if (!next.cities.some((c) => c.slug === city.value)) city.value = next.cities[0]?.slug ?? city.value
      site.value = next
      await loadCity()
      loadError.value = ''
      state.value = 'ready'
    } catch (err) {
      fail(err)
    } finally {
      loading.value = false
    }
  }

  async function pick(slug: string) {
    if (slug === city.value) return
    city.value = slug
    overview.value = null
    try {
      await loadCity()
      loadError.value = ''
    } catch (err) {
      fail(err)
    }
  }

  async function login(token: string): Promise<string> {
    const res = await fetch('/api/admin/login', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      credentials: 'same-origin',
      body: JSON.stringify({token}),
    }).catch(() => null)
    if (!res || !res.ok) return res?.status === 401 ? 'Wrong token' : `Could not log in${res ? ` (HTTP ${res.status})` : ''}`
    writeAuthFlag(true)
    state.value = 'loading'
    await load()
    return ''
  }

  async function logout() {
    await fetch('/api/admin/logout', {method: 'POST', credentials: 'same-origin'}).catch(() => {})
    signedOut()
  }

  function onVisibility() {
    if (document.visibilityState === 'visible' && state.value === 'ready') void load()
  }

  onMounted(() => {
    if (state.value === 'loading') void load()
    refreshTimer = setInterval(() => {
      if (document.visibilityState === 'visible' && state.value === 'ready') void load()
    }, REFRESH_MS)
    clockTimer = setInterval(() => {
      clock.value = Date.now()
    }, 1000)
    document.addEventListener('visibilitychange', onVisibility)
  })

  onBeforeUnmount(() => {
    if (refreshTimer) clearInterval(refreshTimer)
    if (clockTimer) clearInterval(clockTimer)
    document.removeEventListener('visibilitychange', onVisibility)
  })

  return {state, site, overview, city, loading, loadError, clock, load, pick, login, logout}
}
