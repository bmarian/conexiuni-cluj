import { get, set, del } from 'idb-keyval'

export const LOW_ACCURACY_SHELF_LIFE = 1000 * 60 * 60 // 1 hour
export const HIGH_ACCURACY_SHELF_LIFE = 1000 * 5 // 5 seconds

type CachedEnvelope = { timestamp: number; data: unknown }

// In-flight request deduplication: if two callers ask for the same URL
// before the first one resolves, both await the same promise instead of
// firing two network requests. Cleared as soon as the promise settles.
const inFlight = new Map<string, Promise<unknown>>()

export const apiRequest = async (url: string, shelfLife: number = LOW_ACCURACY_SHELF_LIFE): Promise<unknown> => {
  const cached = await getFromCache(url, shelfLife)
  if (cached.hit) return cached.data

  const pending = inFlight.get(url)
  if (pending) return pending

  const promise = (async () => {
    const apiUrl = `/api/${url}`
    const response = await fetch(apiUrl)
    if (!response.ok) {
      throw new Error(`API request failed with status ${response.status}`)
    }
    const data = await response.json()
    await saveToCache(url, data)
    return data
  })()

  inFlight.set(url, promise)
  try {
    return await promise
  } finally {
    inFlight.delete(url)
  }
}

const getFromCache = async (key: string, shelfLife: number): Promise<{hit: true; data: unknown} | {hit: false}> => {
  try {
    const envelope = await get(key) as CachedEnvelope | undefined
    if (!envelope) return {hit: false}

    if (Date.now() - envelope.timestamp < shelfLife) {
      // Note: data may be null/undefined/[] — those are still valid cache hits.
      return {hit: true, data: envelope.data}
    }
    await del(key)
    return {hit: false}
  } catch (err) {
    console.warn('Failed to read from cache:', err)
    return {hit: false}
  }
}

const saveToCache = async (key: string, data: unknown) => {
  try {
    await set(key, {data, timestamp: Date.now()} satisfies CachedEnvelope)
  } catch (err) {
    console.warn('Failed to save to cache:', err)
  }
}
