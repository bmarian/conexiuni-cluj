// The sealing and key steps of backend/devicesync/protocol.md.

export type Bytes = Uint8Array<ArrayBuffer>

const subtle = () => globalThis.crypto.subtle

export const canSync = () => typeof globalThis.crypto?.subtle?.deriveBits === 'function'

export const random = (n: number): Bytes => globalThis.crypto.getRandomValues(new Uint8Array(n))

export const utf8 = (text: string): Bytes => new TextEncoder().encode(text) as Bytes

export function concat(...parts: Bytes[]): Bytes {
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0))
  let at = 0
  for (const part of parts) {
    out.set(part, at)
    at += part.length
  }
  return out
}

export const sameBytes = (a: Bytes, b: Bytes) => a.length === b.length && a.every((x, i) => x === b[i])

export function toB64(bytes: Bytes): string {
  let binary = ''
  for (const b of bytes) binary += String.fromCharCode(b)
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

// Strict base64url: no padding, no other alphabet, no stray bits; null unless exactly size bytes when given.
export function fromB64(text: string, size?: number): Bytes | null {
  if (typeof text !== 'string' || !/^[A-Za-z0-9_-]*$/.test(text)) return null
  let bytes: Bytes
  try {
    bytes = Uint8Array.from(atob(text.replace(/-/g, '+').replace(/_/g, '/')), (c) => c.charCodeAt(0))
  } catch {
    return null
  }
  if (toB64(bytes) !== text || (size !== undefined && bytes.length !== size)) return null
  return bytes
}

export const SPKI_SIZE = 91
export const KEY_SIZE = 32
export const ID_SIZE = 16
export const PAIR_PADDED = 512
export const WRAP_SIZE = SPKI_SIZE + 12 + KEY_SIZE + 16
export const BUCKETS = [1024, 4096, 16384, 65536]

export async function sha256(data: Bytes): Promise<Bytes> {
  return new Uint8Array(await subtle().digest('SHA-256', data))
}

// An empty salt is 32 zero bytes, as protocol.md §1 says.
export async function hkdf(ikm: Bytes, salt: Bytes, info: string | Bytes, n: number): Promise<Bytes> {
  const key = await subtle().importKey('raw', ikm, 'HKDF', false, ['deriveBits'])
  const bits = await subtle().deriveBits({
    name: 'HKDF',
    hash: 'SHA-256',
    salt: salt.length ? salt : new Uint8Array(32),
    info: typeof info === 'string' ? utf8(info) : info,
  }, key, n * 8)
  return new Uint8Array(bits)
}

export type KeyPair = {privateKey: CryptoKey; spki: Bytes}

const P256 = {name: 'ECDH', namedCurve: 'P-256'}

export async function newKeyPair(): Promise<KeyPair> {
  const pair = await subtle().generateKey(P256, false, ['deriveBits'])
  return {privateKey: pair.privateKey, spki: new Uint8Array(await subtle().exportKey('spki', pair.publicKey))}
}

export async function validPublicKey(spki: Bytes): Promise<boolean> {
  if (spki.length !== SPKI_SIZE) return false
  try {
    await subtle().importKey('spki', spki, P256, true, [])
    return true
  } catch {
    return false
  }
}

export async function ecdh(own: CryptoKey, otherSpki: Bytes): Promise<Bytes> {
  const other = await subtle().importKey('spki', otherSpki, P256, true, [])
  return new Uint8Array(await subtle().deriveBits({name: 'ECDH', public: other}, own, 256))
}

const aesKey = (raw: Bytes, usage: KeyUsage) => subtle().importKey('raw', raw, 'AES-GCM', false, [usage])

export async function seal(key: Bytes, plain: Bytes, aad: string, nonce: Bytes = random(12)): Promise<Bytes> {
  const sealed = await subtle().encrypt({name: 'AES-GCM', iv: nonce, additionalData: utf8(aad)}, await aesKey(key, 'encrypt'), plain)
  return concat(nonce, new Uint8Array(sealed))
}

// Null when the key, the associated data or the bytes are not the ones it was sealed with.
export async function open(key: Bytes, sealed: Bytes, aad: string): Promise<Bytes | null> {
  if (sealed.length < 28) return null
  try {
    const plain = await subtle().decrypt({name: 'AES-GCM', iv: sealed.slice(0, 12), additionalData: utf8(aad)}, await aesKey(key, 'decrypt'), sealed.slice(12))
    return new Uint8Array(plain)
  } catch {
    return null
  }
}

export function pad(plain: Bytes, size: number): Bytes {
  const out = new Uint8Array(size)
  new DataView(out.buffer).setUint32(0, plain.length)
  out.set(plain, 4)
  return out
}

export function unpad(padded: Bytes): Bytes | null {
  if (padded.length < 4) return null
  const n = new DataView(padded.buffer, padded.byteOffset).getUint32(0)
  if (n > padded.length - 4 || padded.subarray(4 + n).some((b) => b !== 0)) return null
  return padded.slice(4, 4 + n)
}

export const bucketFor = (n: number) => BUCKETS.find((b) => n + 4 <= b) ?? 0

export async function pairKey(shared: Bytes, k: Bytes | null, code: string, pe: Bytes, pd: Bytes): Promise<Bytes> {
  return hkdf(shared, k ?? new Uint8Array(0), concat(utf8(`cai-sync/v1/pair|${code}|`), pe, pd), 32)
}

export async function checkNumber(t: Bytes): Promise<string> {
  const bytes = await hkdf(t, new Uint8Array(0), 'cai-sync/v1/check', 4)
  return String(new DataView(bytes.buffer).getUint32(0) % 1_000_000).padStart(6, '0')
}

export const pairAAD = (code: string, sender: 'D' | 'E') => `cai-sync/v1/pair-message|${code}|${sender}`

export type ChainKeys = {join: Bytes; data: Bytes; names: Bytes}

export async function chainKeys(s: Bytes): Promise<ChainKeys> {
  const salt = utf8('cai-sync/v1')
  const [join, data, names] = await Promise.all(['join', 'data', 'names'].map((label) => hkdf(s, salt, `cai-sync/v1/${label}`, 32)))
  return {join: join!, data: data!, names: names!}
}

export async function recordName(names: Bytes, logical: string): Promise<string> {
  const key = await subtle().importKey('raw', names, {name: 'HMAC', hash: 'SHA-256'}, false, ['sign'])
  return toB64(new Uint8Array(await subtle().sign('HMAC', key, utf8(logical))).slice(0, ID_SIZE))
}

export const recordAAD = (chain: string, epoch: number, name: string, rev: number) => `cai-sync/v1/record|${chain}|${epoch}|${name}|${rev}`

export const wrapInfo = (chain: string, epoch: number, device: string) => `cai-sync/v1/wrap|${chain}|${epoch}|${device}`

export async function wrapSecret(deviceSpki: Bytes, s: Bytes, info: string, ephemeral?: KeyPair, nonce?: Bytes): Promise<Bytes> {
  const eph = ephemeral ?? await newKeyPair()
  const w = await hkdf(await ecdh(eph.privateKey, deviceSpki), eph.spki, info, 32)
  return concat(eph.spki, await seal(w, s, info, nonce))
}

export async function unwrapSecret(device: CryptoKey, body: Bytes, info: string): Promise<Bytes | null> {
  if (body.length !== WRAP_SIZE) return null
  const px = body.slice(0, SPKI_SIZE)
  try {
    const w = await hkdf(await ecdh(device, px), px, info, 32)
    return await open(w, body.slice(SPKI_SIZE), info)
  } catch {
    return null
  }
}
