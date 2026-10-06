export type TransferLink = { code: string; secret: string; expiresIn: number }

export const TRANSFER_QUERY = 'send'

export function transferLinkUrl(code: string): string {
  return `${window.location.origin}/settings?${TRANSFER_QUERY}=${code}`
}

export async function createTransferLink(): Promise<TransferLink> {
  const response = await fetch('/api/transfer', {method: 'POST'})
  if (!response.ok) throw new Error(`transfer: status ${response.status}`)
  return response.json() as Promise<TransferLink>
}

export async function sendToTransferLink(code: string, data: unknown): Promise<'sent' | 'unknown' | 'failed'> {
  try {
    const response = await fetch(`/api/transfer/${encodeURIComponent(code)}`, {
      method: 'PUT',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(data),
    })
    if (response.ok) return 'sent'
    return response.status === 404 ? 'unknown' : 'failed'
  } catch {
    return 'failed'
  }
}

export type TransferRead = { state: 'waiting' } | { state: 'gone' } | { state: 'received'; data: unknown }

export async function readTransferLink(link: TransferLink): Promise<TransferRead> {
  const response = await fetch(`/api/transfer/${link.code}`, {headers: {'X-Link-Secret': link.secret}})
  if (response.status === 404) return {state: 'gone'}
  if (!response.ok) throw new Error(`transfer: status ${response.status}`)
  if (response.status === 204) return {state: 'waiting'}
  return {state: 'received', data: await response.json()}
}

export function deleteTransferLink(link: TransferLink) {
  void fetch(`/api/transfer/${link.code}`, {method: 'DELETE', headers: {'X-Link-Secret': link.secret}}).catch(() => {})
}
