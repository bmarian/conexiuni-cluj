// Pairing messages (protocol.md §2.6), held to the same strict, canonical rules as records.
import {DEVICE_CAP, type DeviceKind, validText} from '@/utils/sync/records.ts'
import {fromB64, ID_SIZE, KEY_SIZE} from '@/utils/sync/crypto.ts'

export type Hello = {t: 'hello'; kind: DeviceKind; label: string; syncs: boolean; devices: number}
export type Ready = {t: 'ready'}
export type Secret = {t: 'secret'; chain: string; epoch: number; s: string}
export type Message = Hello | Ready | Secret

export function writeMessage(m: Message): string {
  switch (m.t) {
    case 'hello':
      return JSON.stringify({t: m.t, kind: m.kind, label: m.label, syncs: m.syncs, devices: m.devices})
    case 'ready':
      return JSON.stringify({t: m.t})
    case 'secret':
      return JSON.stringify({t: m.t, chain: m.chain, epoch: m.epoch, s: m.s})
  }
}

function check(m: Record<string, unknown>): Message | null {
  const keys = Object.keys(m)
  if (m.t === 'ready' && keys.length === 1) return {t: 'ready'}
  if (m.t === 'hello' && keys.length === 5
    && (m.kind === 'web' || m.kind === 'android' || m.kind === 'wear')
    && validText(m.label, 1, 40) && typeof m.syncs === 'boolean'
    && Number.isInteger(m.devices) && (m.syncs ? (m.devices as number) >= 1 && (m.devices as number) <= DEVICE_CAP : m.devices === 0)) {
    return m as Hello
  }
  if (m.t === 'secret' && keys.length === 4 && typeof m.chain === 'string' && fromB64(m.chain, ID_SIZE)
    && Number.isInteger(m.epoch) && (m.epoch as number) >= 1 && typeof m.s === 'string' && fromB64(m.s, KEY_SIZE)) {
    return m as Secret
  }
  return null
}

export function readMessage(raw: string): Message | null {
  try {
    const parsed: unknown = JSON.parse(raw)
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) return null
    const message = check(parsed as Record<string, unknown>)
    return message && writeMessage(message) === raw ? message : null
  } catch {
    return null
  }
}
