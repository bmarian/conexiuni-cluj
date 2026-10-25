import {computed, ref, watch} from 'vue'
import {defineStore} from 'pinia'
import {apiRequest} from '@/utils/api'
import {cacheKey, cityInfo, currentCity} from '@/utils/city.ts'
import {CITY_CAP, newestSeen} from '@/utils/sync/records.ts'

export const FOLLOWS_KEY = 'follows:routes'
const followedKey = (city?: string) => cacheKey(FOLLOWS_KEY, city)
const seenKey = (city?: string) => cacheKey('route-changes:seen', city)
const REFRESH_AFTER_MS = 30 * 60_000

export type RouteChangeKind = 'later' | 'earlier' | 'trips_added' | 'trips_removed' | 'stops_added' | 'stops_removed'
export type RouteChangeDay = 'weekdays' | 'saturday' | 'sunday'

export interface RouteChangeItem {
  kind: RouteChangeKind
  direction: '0' | '1'
  toward?: string
  days?: RouteChangeDay[]
  since?: string
  all?: boolean
  times?: string[]
  shifts?: {from: string; to: string}[]
  stops?: string[]
}

export interface RouteChange {
  id: number
  route_short_name: string
  route_id: number
  route_color: string
  source: 'timetable' | 'stops'
  detected_at: number
  changes: RouteChangeItem[]
}

export interface NewsItem {
  title: string
  url: string
}

export type Update = {kind: 'news'; at: number; news: NewsItem} | {kind: 'change'; at: number; change: RouteChange}

function readList<T>(key: string, isItem: (v: unknown) => v is T): T[] {
  try {
    const raw = JSON.parse(localStorage.getItem(key) ?? 'null')
    return Array.isArray(raw) ? raw.filter(isItem) : []
  } catch {
    return []
  }
}

function writeList(key: string, value: unknown[]) {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {}
}

export const MAX_FOLLOWED_LINES = CITY_CAP

const isString = (v: unknown): v is string => typeof v === 'string' && v.length > 0
const isNumber = (v: unknown): v is number => typeof v === 'number'

export const useRouteUpdatesStore = defineStore('routeUpdates', () => {
  const followed = ref<string[]>(readList(followedKey(), isString).slice(0, MAX_FOLLOWED_LINES))
  const seenIds = ref(new Set(readList(seenKey(), isNumber)))
  const updates = ref<Update[]>([])
  const changes = computed(() => updates.value.flatMap((u) => (u.kind === 'change' ? [u.change] : [])))
  const hasFeed = computed(() => followed.value.length > 0 || !!cityInfo()?.news)
  const loading = ref(false)
  const error = ref(false)
  let lastFetchAt = 0
  let fetchSeq = 0

  // Also when another tab changed what is saved.
  function reload() {
    followed.value = readList(followedKey(), isString).slice(0, MAX_FOLLOWED_LINES)
    seenIds.value = new Set(readList(seenKey(), isNumber))
  }

  watch(
    currentCity,
    () => {
      reload()
      updates.value = []
      lastFetchAt = 0
    },
    {flush: 'sync'},
  )

  const unseenIds = computed(() => new Set(changes.value.filter((c) => !seenIds.value.has(c.id)).map((c) => c.id)))
  const hasUnseen = computed(() => unseenIds.value.size > 0)

  function isFollowing(routeShortName: string): boolean {
    return followed.value.includes(routeShortName)
  }

  /** null when the limit is reached */
  function toggleFollow(routeShortName: string): boolean | null {
    const idx = followed.value.indexOf(routeShortName)
    if (idx === -1 && followed.value.length >= MAX_FOLLOWED_LINES) return null
    if (idx === -1) followed.value.push(routeShortName)
    else followed.value.splice(idx, 1)
    writeList(followedKey(), followed.value)
    return idx === -1
  }

  const followedIn = (city: string) => (city === currentCity.value ? followed.value : readList(followedKey(city), isString))

  function importFollowed(raw: unknown, city = currentCity.value) {
    const list = Array.isArray(raw) ? [...new Set(raw.filter(isString))].slice(0, MAX_FOLLOWED_LINES) : []
    writeList(followedKey(city), list)
    if (city === currentCity.value) followed.value = list
  }

  async function fetchUpdates() {
    const seq = ++fetchSeq
    if (!hasFeed.value) {
      updates.value = []
      error.value = false
      return
    }
    loading.value = true
    error.value = false
    try {
      const routes = [...followed.value].sort().map(encodeURIComponent).join(',')
      const data = await apiRequest<Update[]>(routes ? `updates?routes=${routes}` : 'updates')
      if (seq !== fetchSeq) return
      updates.value = data
      lastFetchAt = Date.now()
    } catch {
      if (seq === fetchSeq) error.value = true
    } finally {
      if (seq === fetchSeq) loading.value = false
    }
  }

  function refreshIfStale() {
    if (Date.now() - lastFetchAt > REFRESH_AFTER_MS) void fetchUpdates()
  }

  function markSeen(ids: number[]) {
    const next = new Set(seenIds.value)
    ids.forEach((id) => next.add(id))
    if (next.size === seenIds.value.size) return
    seenIds.value = new Set(newestSeen([...next]))
    writeList(seenKey(), [...seenIds.value])
  }

  const seenIn = (city: string) => (city === currentCity.value ? [...seenIds.value] : readList(seenKey(city), isNumber))

  function importSeen(ids: number[], city = currentCity.value) {
    writeList(seenKey(city), ids)
    if (city === currentCity.value) seenIds.value = new Set(ids)
  }

  function markAllSeen() {
    markSeen(changes.value.map((c) => c.id))
  }

  async function loadChange(id: number): Promise<RouteChange | null> {
    const known = changes.value.find((c) => c.id === id)
    if (known) return known
    try {
      const data = await apiRequest<RouteChange[]>(`route-changes?id=${id}`)
      return data[0] ?? null
    } catch {
      return null
    }
  }

  watch(followed, () => void fetchUpdates(), {deep: true})

  return {
    followed,
    updates,
    changes,
    hasFeed,
    loading,
    error,
    seenIds,
    unseenIds,
    hasUnseen,
    isFollowing,
    toggleFollow,
    followedIn,
    importFollowed,
    reload,
    fetchUpdates,
    refreshIfStale,
    markSeen,
    markAllSeen,
    seenIn,
    importSeen,
    loadChange,
  }
})
