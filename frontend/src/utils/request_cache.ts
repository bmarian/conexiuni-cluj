import { get, set, del } from 'idb-keyval'

export const LOW_ACCURACY_SHELF_LIFE = 1000 * 60 * 60 // 1 hour
export const HIGH_ACCURACY_SHELF_LIFE = 1000 * 5 // 5 seconds

export const apiRequest = async (url: string, shelfLife: number = LOW_ACCURACY_SHELF_LIFE): Promise<unknown> => {
  const cachedData = await getFromCache(url, shelfLife);
  if (cachedData) return cachedData

  const apiUrl = `/api/${url}`
  const response = await fetch(apiUrl)
  if (!response.ok) {
    throw new Error(`API request failed with status ${response.status}`)
  }

  const data = await response.json()
  await saveToCache(url, data)

  return data
};

const getFromCache = async (key: string, shelfLife: number): Promise<unknown | null> => {
  try {
    const cachedDataJson = await get(key) as { timestamp: number, data: unknown } | undefined
    if (!cachedDataJson) return null

    const now = Date.now()
    if (now - cachedDataJson.timestamp < shelfLife) {
      return cachedDataJson.data
    } else {
      await del(key)
      return null
    }
  } catch (err) {
    console.warn('Failed to read from cache:', err)
    return null
  }
}

const saveToCache = async (key: string, data: unknown) => {
  const cachedDataJson = {
    data,
    timestamp: Date.now()
  }

  try {
    await set(key, cachedDataJson)
  } catch (err) {
    console.warn('Failed to save to cache:', err)
  }
}
