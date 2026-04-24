const inFlight = new Map<string, Promise<unknown>>()

export const apiRequest = async (url: string): Promise<unknown> => {
  const pending = inFlight.get(url)
  if (pending) return pending

  const promise = fetch(`/api/${url}`).then((response) => {
    if (!response.ok) throw new Error(`API request failed with status ${response.status}`)
    return response.json()
  })

  inFlight.set(url, promise)
  try {
    return await promise
  } finally {
    inFlight.delete(url)
  }
}
