export async function apiRequest<T>(url: string): Promise<T> {
  const response = await fetch(`/api/${url}`)
  if (!response.ok) {
    throw new Error(`API request failed with status ${response.status}`)
  }
  return response.json() as Promise<T>
}
