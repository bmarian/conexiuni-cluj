const inFlight = new Map<string, Promise<unknown>>()
const cache = new Map<string, { data: unknown; expiry: number }>()

export const apiRequest = async (url: string, ttlMs: number = 0): Promise<unknown> => {
  if (ttlMs > 0) {
    const cached = cache.get(url)
    if (cached && cached.expiry > Date.now()) {
      return cached.data
    }
  }

  const pending = inFlight.get(url)
  if (pending) return pending

  const promise = fetch(`/api/${url}`).then((response) => {
    if (!response.ok) throw new Error(`API request failed with status ${response.status}`)
    return response.json()
  })

  inFlight.set(url, promise)
  try {
    const data = await promise
    if (ttlMs > 0) {
      cache.set(url, { data, expiry: Date.now() + ttlMs })
    }
    return data
  } finally {
    inFlight.delete(url)
  }
}
