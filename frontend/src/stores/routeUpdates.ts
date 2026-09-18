import {computed, ref, watch} from 'vue'
import {defineStore} from 'pinia'
import {apiRequest} from '@/utils/api'

const FOLLOWED_KEY = 'follows:routes'
const SEEN_KEY = 'route-changes:seen'
const MAX_SEEN_IDS = 500
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
  shifts?: { from: string; to: string }[]
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

const isString = (v: unknown): v is string => typeof v === 'string' && v.length > 0
const isNumber = (v: unknown): v is number => typeof v === 'number'

export const useRouteUpdatesStore = defineStore('routeUpdates', () => {
  const followed = ref<string[]>(readList(FOLLOWED_KEY, isString))
  const seenIds = ref(new Set(readList(SEEN_KEY, isNumber)))
  const changes = ref<RouteChange[]>([])
  const loading = ref(false)
  const error = ref(false)
  let lastFetchAt = 0
  let fetchSeq = 0

  const unseenIds = computed(() => new Set(changes.value.filter((c) => !seenIds.value.has(c.id)).map((c) => c.id)))
  const hasUnseen = computed(() => unseenIds.value.size > 0)

  function isFollowing(routeShortName: string): boolean {
    return followed.value.includes(routeShortName)
  }

  function toggleFollow(routeShortName: string): boolean {
    const idx = followed.value.indexOf(routeShortName)
    if (idx === -1) followed.value.push(routeShortName)
    else followed.value.splice(idx, 1)
    writeList(FOLLOWED_KEY, followed.value)
    return idx === -1
  }

  function importFollowed(raw: unknown) {
    followed.value = Array.isArray(raw) ? [...new Set(raw.filter(isString))] : []
    writeList(FOLLOWED_KEY, followed.value)
  }

  async function fetchChanges() {
    const seq = ++fetchSeq
    if (!followed.value.length) {
      changes.value = []
      error.value = false
      return
    }
    loading.value = true
    error.value = false
    try {
      const routes = [...followed.value].sort().map(encodeURIComponent).join(',')
      const data = await apiRequest<RouteChange[]>(`route-changes?routes=${routes}`)
      if (seq !== fetchSeq) return
      changes.value = data
      lastFetchAt = Date.now()
    } catch {
      if (seq === fetchSeq) error.value = true
    } finally {
      if (seq === fetchSeq) loading.value = false
    }
  }

  function refreshIfStale() {
    if (Date.now() - lastFetchAt > REFRESH_AFTER_MS) void fetchChanges()
  }

  function markSeen(ids: number[]) {
    const next = new Set(seenIds.value)
    ids.forEach((id) => next.add(id))
    if (next.size === seenIds.value.size) return
    seenIds.value = new Set([...next].sort((a, b) => b - a).slice(0, MAX_SEEN_IDS))
    writeList(SEEN_KEY, [...seenIds.value])
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

  watch(followed, () => void fetchChanges(), {deep: true})

  return {
    followed,
    changes,
    loading,
    error,
    unseenIds,
    hasUnseen,
    isFollowing,
    toggleFollow,
    importFollowed,
    fetchChanges,
    refreshIfStale,
    markSeen,
    markAllSeen,
    loadChange,
  }
})
