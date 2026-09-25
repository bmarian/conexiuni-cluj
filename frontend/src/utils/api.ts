export async function apiRequest<T>(url: string): Promise<T> {
  const separator = url.includes('?') ? '&' : '?'
  const response = await fetch(`/api/${url}${separator}v=${__APP_VERSION__}`)
  if (!response.ok) {
    throw new Error(`API request failed with status ${response.status}`)
  }
  return response.json() as Promise<T>
}

export function readCachedList<T>(key: string): T[] {
  try {
    const cached = JSON.parse(localStorage.getItem(key) ?? 'null')
    return Array.isArray(cached) ? cached : []
  } catch {
    return []
  }
}

export function writeCachedList(key: string, list: unknown[]) {
  try {
    localStorage.setItem(key, JSON.stringify(list))
  } catch { /* noop */ }
}

export async function apiPost(url: string, body: unknown): Promise<void> {
  const response = await fetch(`/api/${url}`, {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(body),
  })
  if (!response.ok) {
    throw new Error(`API request failed with status ${response.status}`)
  }
}
