import {readFileSync} from 'node:fs'
import {describe, expect, it} from 'vitest'
import {
  type Bytes,
  chainKeys,
  checkNumber,
  ecdh,
  fromB64,
  open,
  pad,
  pairKey,
  recordName,
  seal,
  sha256,
  toB64,
  unpad,
  unwrapSecret,
  utf8,
  wrapSecret,
} from '@/utils/sync/crypto.ts'
import {emptyCity, readCity, readDevices, readSettings, writeCity, writeDevices, writeSettings} from '@/utils/sync/records.ts'
import {mergeCity, mergeDevices, mergeSettings} from '@/utils/sync/merge.ts'

// The shared files every client runs: made and checked by backend/devicesync/vectors_test.go.
const testdata = (name: string) => JSON.parse(readFileSync(new URL(`../../../../../backend/devicesync/testdata/${name}`, import.meta.url), 'utf8'))
const vectors = testdata('vectors.json')
const cases = testdata('cases.json')

const b = (text: string): Bytes => fromB64(text)!

const privateKey = (pkcs8: string) =>
  crypto.subtle.importKey('pkcs8', b(pkcs8), {name: 'ECDH', namedCurve: 'P-256'}, false, ['deriveBits'])

describe('sync vectors', () => {
  it('derives the pairing channel', async () => {
    const p = vectors.pair
    const [d, e] = await Promise.all([privateKey(p.d.pkcs8), privateKey(p.e.pkcs8)])
    expect(toB64(await ecdh(d, b(p.e.spki)))).toBe(p.shared)
    expect(toB64(await ecdh(e, b(p.d.spki)))).toBe(p.shared)
    expect(toB64(await sha256(b(p.d.spki)))).toBe(p.commit)
    for (const [mode, k] of [[p.scanned, b(p.k)], [p.typed, null]] as const) {
      const t = await pairKey(b(p.shared), k, p.code, b(p.e.spki), b(p.d.spki))
      expect(toB64(t)).toBe(mode.t)
      expect(await checkNumber(t)).toBe(mode.check)
      for (const m of mode.messages) {
        expect(toB64(await seal(t, pad(utf8(m.json), m.bucket), m.aad, b(m.nonce)))).toBe(m.body)
        expect(new TextDecoder().decode(unpad((await open(t, b(m.body), m.aad))!)!)).toBe(m.json)
      }
    }
  })

  it('derives the chain keys and seals records', async () => {
    const c = vectors.chain
    const keys = await chainKeys(b(c.s))
    expect([toB64(keys.join), toB64(keys.data), toB64(keys.names)]).toEqual([c.join, c.data, c.names])
    expect(toB64(await sha256(keys.join))).toBe(c.joinHash)
    for (const [logical, stored] of Object.entries(c.recordNames)) expect(await recordName(keys.names, logical)).toBe(stored)
    for (const r of c.records) {
      expect(toB64(await seal(keys.data, pad(utf8(r.json), r.bucket), r.aad, b(r.nonce)))).toBe(r.body)
      expect(new TextDecoder().decode(unpad((await open(keys.data, b(r.body), r.aad))!)!)).toBe(r.json)
      const read = r.name === 'settings' ? readSettings(r.json) : r.name === 'devices' ? await readDevices(r.json) : readCity(r.json)
      expect(read.ok).toBe(true)
    }
  })

  it('wraps a new secret for one device', async () => {
    const w = vectors.wrap
    const ephemeral = {privateKey: await privateKey(w.ephemeral.pkcs8), spki: b(w.ephemeral.spki)}
    expect(toB64(await wrapSecret(b(w.deviceKey.spki), b(w.s), w.info, ephemeral, b(w.nonce)))).toBe(w.body)
    expect(toB64((await unwrapSecret(await privateKey(w.deviceKey.pkcs8), b(w.body), w.info))!)).toBe(w.s)
    expect(await unwrapSecret(await privateKey(vectors.pair.d.pkcs8), b(w.body), w.info)).toBeNull()
  })
})

type RecordCase = {kind: string; about: string; json: string; outcome: string}
type MergeCase = {kind: string; about: string; base: string | null; local: string; remote: string; merged: string; dropped: Record<string, unknown[]>}

async function outcome(kind: string, raw: string) {
  const read = kind === 'city' ? readCity(raw) : kind === 'devices' ? await readDevices(raw) : readSettings(raw)
  return read.ok ? 'ok' : read.newer ? 'newer' : 'refuse'
}

describe('sync cases', () => {
  it.each(cases.records as RecordCase[])('$kind: $about', async (c) => {
    expect(await outcome(c.kind, c.json)).toBe(c.outcome)
  })

  it.each(cases.merges as MergeCase[])('merge $kind: $about', async (c) => {
    const value = async <T>(raw: string | null, read: (raw: string) => {ok: boolean} | Promise<{ok: boolean}>) => {
      if (raw === null) return null
      const r = await read(raw) as {ok: true; value: T}
      expect(r.ok).toBe(true)
      return r.value
    }
    if (c.kind === 'city') {
      const [base, local, remote] = await Promise.all([c.base, c.local, c.remote].map((raw) => value<ReturnType<typeof emptyCity>>(raw, readCity)))
      const {merged, dropped} = mergeCity(base ?? emptyCity(), local!, remote!)
      expect(writeCity(merged)).toBe(c.merged)
      const nonEmpty = Object.fromEntries(Object.entries({
        stops: dropped.stops,
        lines: dropped.lines.map((l) => [l.route, l.dir]),
        places: dropped.places.map((p) => [p.name, p.label, p.lat, p.lon]),
        followed: dropped.followed,
      }).filter(([, items]) => items.length))
      expect(nonEmpty).toEqual(c.dropped)
    } else if (c.kind === 'settings') {
      const [base, local, remote] = await Promise.all([c.base, c.local, c.remote].map((raw) => value<Parameters<typeof writeSettings>[0]>(raw, readSettings)))
      expect(writeSettings(mergeSettings(base, local!, remote!))).toBe(c.merged)
    } else {
      const [base, local, remote] = await Promise.all([c.base, c.local, c.remote].map((raw) => value<Parameters<typeof writeDevices>[0]>(raw, readDevices)))
      expect(writeDevices({devices: mergeDevices(base?.devices ?? [], local!.devices, remote!.devices)})).toBe(c.merged)
    }
  })
})
