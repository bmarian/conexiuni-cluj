export async function apiRequest<T>(url: string): Promise<T> {
  const separator = url.includes('?') ? '&' : '?'
  const response = await fetch(`/api/${url}${separator}v=${__APP_VERSION__}`)
  if (!response.ok) {
    throw new Error(`API request failed with status ${response.status}`)
  }
  return response.json() as Promise<T>
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
