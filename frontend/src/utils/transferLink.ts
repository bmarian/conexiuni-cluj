export type TransferLink = { code: string; secret: string; expiresIn: number }

export const TRANSFER_QUERY = 'send'
const KEY_PARAM = 'k'

// A scanned code also carries a key after the #, which browsers never send to the server,
// so what the server holds for those transfers is unreadable to it. A typed code can't
// carry one. WebCrypto only exists on https (and localhost), so elsewhere nothing is sealed.
const canSeal = () => !!globalThis.crypto?.subtle

const toBase64Url = (bytes: Uint8Array) =>
  btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
const fromBase64Url = (text: string) =>
  Uint8Array.from(atob(text.replace(/-/g, '+').replace(/_/g, '/')), (c) => c.charCodeAt(0))

export function createTransferKey(): string | null {
  return canSeal() ? toBase64Url(crypto.getRandomValues(new Uint8Array(16))) : null
}

export function transferKeyFromHash(hash: string): string | undefined {
  return new URLSearchParams(hash.replace(/^#/, '')).get(KEY_PARAM) ?? undefined
}

export function transferLinkUrl(code: string, key: string | null): string {
  const hash = key ? `#${KEY_PARAM}=${key}` : ''
  return `${window.location.origin}/settings?${TRANSFER_QUERY}=${code}${hash}`
}

type SealedTransfer = { sealed: 1; iv: string; data: string }

const isSealed = (value: unknown): value is SealedTransfer =>
  typeof value === 'object' && value !== null && (value as SealedTransfer).sealed === 1
  && typeof (value as SealedTransfer).iv === 'string' && typeof (value as SealedTransfer).data === 'string'

const importKey = (key: string, usage: KeyUsage) =>
  crypto.subtle.importKey('raw', fromBase64Url(key), 'AES-GCM', false, [usage])

async function sealTransfer(data: unknown, key: string | undefined): Promise<unknown> {
  if (!key || !canSeal()) return data
  const iv = crypto.getRandomValues(new Uint8Array(12))
  const plain = new TextEncoder().encode(JSON.stringify(data))
  const sealed = await crypto.subtle.encrypt({name: 'AES-GCM', iv}, await importKey(key, 'encrypt'), plain)
  return {sealed: 1, iv: toBase64Url(iv), data: toBase64Url(new Uint8Array(sealed))} satisfies SealedTransfer
}

// Throws when the data is sealed with a key this device doesn't have.
export async function openTransfer(payload: unknown, key: string | null): Promise<unknown> {
  if (!isSealed(payload)) return payload
  if (!key || !canSeal()) throw new Error('sealed transfer without its key')
  const plain = await crypto.subtle.decrypt(
    {name: 'AES-GCM', iv: fromBase64Url(payload.iv)}, await importKey(key, 'decrypt'), fromBase64Url(payload.data))
  return JSON.parse(new TextDecoder().decode(plain))
}

export async function createTransferLink(): Promise<TransferLink> {
  const response = await fetch('/api/transfer', {method: 'POST'})
  if (!response.ok) throw new Error(`transfer: status ${response.status}`)
  return response.json() as Promise<TransferLink>
}

export async function sendToTransferLink(
  code: string, data: unknown, key?: string,
): Promise<'sent' | 'unknown' | 'limited' | 'failed'> {
  try {
    const response = await fetch(`/api/transfer/${encodeURIComponent(code)}`, {
      method: 'PUT',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(await sealTransfer(data, key)),
    })
    if (response.ok) return 'sent'
    if (response.status === 404) return 'unknown'
    return response.status === 429 ? 'limited' : 'failed'
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
