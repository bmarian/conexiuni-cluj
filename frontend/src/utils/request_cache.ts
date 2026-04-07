export const LOW_ACCURACY_SHELF_LIFE = 1000 * 60 * 60 // 1 hour
export const HIGH_ACCURACY_SHELF_LIFE = 1000 * 10 // 10 seconds

export const apiRequest = async (url: string, shelfLife: number = LOW_ACCURACY_SHELF_LIFE): Promise<unknown> => {
  const cachedData = getFromCache(url, shelfLife);
  if (cachedData) return cachedData

  const apiUrl = `/api/${url}`
  const response = await fetch(apiUrl)
  if (!response.ok) {
    throw new Error(`API request failed with status ${response.status}`)
  }

  const data = await response.json()
  saveToCache(url, data, shelfLife)

  return data
};

const getFromCache = (key: string, shelfLife: number): unknown => {
  const cachedData = localStorage.getItem(key)
  if (!cachedData) return null

  const cachedDataJson = JSON.parse(cachedData)
  const now = Date.now()
  if (now - cachedDataJson.timestamp < shelfLife) {
    return cachedDataJson.data
  } else {
    localStorage.removeItem(key)
    return null
  }
}

const saveToCache = (key: string, data: unknown, shelfLife: number) => {
  const cachedDataJson = {
    data,
    timestamp: Date.now()
  }
  localStorage.setItem(key, JSON.stringify(cachedDataJson))
}
