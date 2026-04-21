import { get, set } from 'idb-keyval'

export const LOW_ACCURACY_SHELF_LIFE = 1000 * 60 * 60
export const HIGH_ACCURACY_SHELF_LIFE = 1000 * 5

type CachedEnvelope = { timestamp: number; data: unknown }

const inFlight = new Map<string, Promise<unknown>>()

export const apiRequest = async (url: string, shelfLife: number = LOW_ACCURACY_SHELF_LIFE): Promise<unknown> => {
  const envelope = await readEnvelope(url)
  if (envelope && Date.now() - envelope.timestamp < shelfLife) {
    return envelope.data
  }

  const pending = inFlight.get(url)
  if (pending) return pending

  const promise = (async () => {
    try {
      const response = await fetch(`/api/${url}`)
      if (!response.ok) {
        throw new Error(`API request failed with status ${response.status}`)
      }
      const data = await response.json()
      await saveToCache(url, data)
      return data
    } catch (err) {
      if (envelope) return envelope.data
      throw err
    }
  })()

  inFlight.set(url, promise)
  try {
    return await promise
  } finally {
    inFlight.delete(url)
  }
}

const readEnvelope = async (key: string): Promise<CachedEnvelope | undefined> => {
  try {
    return (await get(key)) as CachedEnvelope | undefined
  } catch (err) {
    console.warn('Failed to read from cache:', err)
    return undefined
  }
}

const saveToCache = async (key: string, data: unknown) => {
  try {
    await set(key, {data, timestamp: Date.now()} satisfies CachedEnvelope)
  } catch (err) {
    console.warn('Failed to save to cache:', err)
  }
}
