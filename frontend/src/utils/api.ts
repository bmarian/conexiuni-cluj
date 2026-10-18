import {apiPath, cacheKey} from '@/utils/city.ts'

export async function apiRequest<T>(url: string): Promise<T> {
  const separator = url.includes('?') ? '&' : '?'
  const response = await fetch(`${apiPath(url)}${separator}v=${__APP_VERSION__}`)
  if (!response.ok) {
    throw new Error(`API request failed with status ${response.status}`)
  }
  return response.json() as Promise<T>
}

export function readCachedList<T>(key: string): T[] {
  try {
    const cached = JSON.parse(localStorage.getItem(cacheKey(key)) ?? 'null')
    return Array.isArray(cached) ? cached : []
  } catch {
    return []
  }
}

// Written when the page is idle: a big city's list is long enough to stall a map flight.
export function writeCachedList(key: string, list: unknown[]) {
  const fullKey = cacheKey(key)
  const write = () => {
    try {
      localStorage.setItem(fullKey, JSON.stringify(list))
    } catch { /* noop */ }
  }
  if (typeof requestIdleCallback === 'function') requestIdleCallback(write)
  else setTimeout(write)
}

export async function apiPost(url: string, body: unknown): Promise<void> {
  const response = await fetch(apiPath(url), {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(body),
  })
  if (!response.ok) {
    throw new Error(`API request failed with status ${response.status}`)
  }
}
