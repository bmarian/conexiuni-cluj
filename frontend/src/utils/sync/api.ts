// The /api/sync calls of backend/devicesync/protocol.md.

export class SyncHttpError extends Error {
  constructor(readonly status: number, readonly body: Record<string, unknown> = {}) {
    super(`sync: status ${status}`)
  }
}

async function call<T>(method: string, path: string, body?: unknown, headers: Record<string, string> = {}): Promise<T> {
  const response = await fetch(`/api/sync${path}`, {
    method,
    headers: body === undefined ? headers : {...headers, 'Content-Type': 'application/json'},
    body: body === undefined ? undefined : JSON.stringify(body),
    cache: 'no-store',
  })
  const text = await response.text()
  let parsed: unknown = undefined
  try {
    parsed = text ? JSON.parse(text) : undefined
  } catch { /* plain-text error page */ }
  if (!response.ok) throw new SyncHttpError(response.status, (parsed ?? {}) as Record<string, unknown>)
  return parsed as T
}

export const isStatus = (err: unknown, status: number) => err instanceof SyncHttpError && err.status === status

// Pairing.

export type Slot = {code: string; secret: string}
export type SlotRead = {pub: string | null; scanned: boolean; messages: string[]}

const pairHeader = (secret: string) => ({'X-Pair-Secret': secret})

export const openSlot = (commit: string) => call<Slot & {expiresIn: number}>('POST', '/pair', {commit})
export const claimSlot = (code: string, pub: string, scanned: boolean) =>
  call<{commit: string; secret: string}>('POST', `/pair/${code}/claim`, {pub, scanned})
export const readSlot = (slot: Slot) => call<SlotRead>('GET', `/pair/${slot.code}`, undefined, pairHeader(slot.secret))
export const revealKey = (slot: Slot, pub: string) => call<void>('PUT', `/pair/${slot.code}/pub`, {pub}, pairHeader(slot.secret))
export const sendMessage = (slot: Slot, message: string) => call<void>('POST', `/pair/${slot.code}/messages`, {message}, pairHeader(slot.secret))
export const closeSlot = (slot: Slot) => call<void>('DELETE', `/pair/${slot.code}`, undefined, pairHeader(slot.secret)).catch(() => {})

// Chains.

export type Member = {chain: string; token: string}
export type DeviceState = {id: string; kind: string; created: number; seen: number}
export type ChainState = {epoch: number; device: string; devices: DeviceState[]; records: {name: string; rev: number}[]}

const bearer = (m: Member) => ({Authorization: `Bearer ${m.token}`})
const chainPath = (m: Member, rest = '') => `/chains/${m.chain}${rest}`

export type Registered = {epoch: number; device: string; token: string}

export const createChain = (join: string, kind: string) => call<Registered & {chain: string}>('POST', '/chains', {join, kind})
export const joinChain = (chain: string, join: string, kind: string) => call<Registered>('POST', `/chains/${chain}/devices`, {join, kind})
export const chainState = (m: Member) => call<ChainState>('GET', chainPath(m), undefined, bearer(m))
export const readRecord = (m: Member, name: string) => call<{rev: number; body: string}>('GET', chainPath(m, `/records/${name}`), undefined, bearer(m))
export const writeRecord = (m: Member, name: string, epoch: number, base: number, body: string) =>
  call<{rev: number}>('PUT', chainPath(m, `/records/${name}`), {epoch, base, body}, bearer(m))
export const readWrap = (m: Member) => call<{epoch: number; body: string}>('GET', chainPath(m, '/wrap'), undefined, bearer(m))
export const rotateChain = (m: Member, rotation: {epoch: number; seen: {name: string; rev: number}[]; join: string; remove: string[]; records: {name: string; body: string}[]; wraps: {device: string; body: string}[]}) =>
  call<{epoch: number}>('POST', chainPath(m, '/rotate'), rotation, bearer(m))
export const leaveChain = (m: Member) => call<void>('DELETE', chainPath(m, '/devices/self'), undefined, bearer(m))
export const deleteChain = (m: Member) => call<void>('DELETE', chainPath(m), undefined, bearer(m))
