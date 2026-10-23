import {apiPath, cacheKey} from '@/utils/city.ts'

export class ApiError extends Error {
  constructor(readonly status: number) {
    super(`API request failed with status ${status}`)
  }
}

export async function apiRequest<T>(url: string): Promise<T> {
  const separator = url.includes('?') ? '&' : '?'
  const response = await fetch(`${apiPath(url)}${separator}v=${__APP_VERSION__}`)
  if (!response.ok) {
    throw new ApiError(response.status)
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

// Idle-time write: a big city's list stalls a map flight.
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

export async function apiPost(url: string, body: unknown, city?: string): Promise<void> {
  const response = await fetch(apiPath(url, city), {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(body),
  })
  if (!response.ok) {
    throw new Error(`API request failed with status ${response.status}`)
  }
}
