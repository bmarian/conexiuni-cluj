// One sync run and the chain-wide steps (protocol.md §3, §5–§7), on top of this device's vault.
import * as api from '@/utils/sync/api.ts'
import {isStatus, SyncHttpError} from '@/utils/sync/api.ts'
import {
  bucketFor,
  type Bytes,
  chainKeys,
  type ChainKeys,
  fromB64,
  KEY_SIZE,
  newKeyPair,
  open,
  pad,
  random,
  recordAAD,
  recordName,
  seal,
  toB64,
  unpad,
  unwrapSecret,
  utf8,
  wrapInfo,
  wrapSecret,
} from '@/utils/sync/crypto.ts'
import {
  CITY_CAP,
  type CityRecord,
  type DeviceEntry,
  type DevicesRecord,
  emptyCity,
  emptyNews,
  type NewsRecord,
  type Read,
  readCity,
  readDevices,
  readNews,
  readSettings,
  type SettingsRecord,
  writeCity,
  writeDevices,
  writeNews,
  writeSettings,
} from '@/utils/sync/records.ts'
import {mergeCity, mergeDevices, mergeNews, mergeSettings} from '@/utils/sync/merge.ts'
import {
  applyCity,
  applyNews,
  applySettings,
  cityFromLocal,
  isEmptyCity,
  isEmptyNews,
  newsFromLocal,
  settingsFromLocal,
  syncedCities,
} from '@/utils/sync/local.ts'
import {type Base, forget, getBases, getMembership, type Membership, setBases, setMembership} from '@/utils/sync/vault.ts'

// removed: this device is out of its chain (removed, or sync was deleted); broken: something did not open or follow the rules.
export type Problem = 'removed' | 'broken' | 'update' | 'over-cap' | 'full'

export type DeviceView = {id: string; kind: string; label: string; seen: number; self: boolean}

export type Report = {devices: DeviceView[]; dropped: number; problems: Problem[]}

export class Gone extends Error {}
class EpochChanged extends Error {}

const decoder = new TextDecoder()

type Kind<T> = {
  read: (raw: string) => Read<T> | Promise<Read<T>>
  write: (value: T) => string
  local: (base: T | null) => T | null
  merge: (base: T | null, local: T, remote: T) => T
  apply: (value: T) => void
  // Not written while it is still empty and the server has none.
  empty?: (value: T) => boolean
}

type Run = {m: Membership; keys: ChainKeys; bases: Record<string, Base>; report: Report; devices: DeviceEntry[]}

const SETTINGS = 'settings'
const DEVICES = 'devices'
const cityName = (slug: string) => `city:${slug}`
const newsName = (slug: string) => `news:${slug}`

function cityKind(slug: string, run: Run): Kind<CityRecord> {
  return {
    read: readCity,
    write: writeCity,
    local: () => {
      const local = cityFromLocal(slug)
      if ([local.stops, local.lines, local.places, local.followed].some((list) => list.length > CITY_CAP)) {
        run.report.problems.push('over-cap')
        return null
      }
      return local
    },
    merge: (base, local, remote) => {
      const {merged, dropped} = mergeCity(base ?? emptyCity(), local, remote)
      run.report.dropped += dropped.stops.length + dropped.lines.length + dropped.places.length + dropped.followed.length
      return merged
    },
    apply: (value) => applyCity(slug, value),
    empty: isEmptyCity,
  }
}

function newsKind(slug: string): Kind<NewsRecord> {
  return {
    read: readNews,
    write: writeNews,
    local: () => newsFromLocal(slug),
    merge: (base, local, remote) => mergeNews(base ?? emptyNews(), local, remote),
    apply: (value) => applyNews(slug, value),
    empty: isEmptyNews,
  }
}

const settingsKind: Kind<SettingsRecord> = {
  read: readSettings,
  write: writeSettings,
  local: () => settingsFromLocal(),
  merge: mergeSettings,
  apply: applySettings,
}

function selfEntry(m: Membership, label: string): DeviceEntry {
  return {id: m.device, kind: 'web', label, pub: m.devicePub}
}

function devicesKind(run: Run, label: string): Kind<DevicesRecord> {
  return {
    read: readDevices,
    write: writeDevices,
    local: (base) => {
      const others = (base?.devices ?? []).filter((d) => d.id !== run.m.device)
      return {devices: [...others, selfEntry(run.m, label)]}
    },
    merge: (base, local, remote) => ({devices: mergeDevices(base?.devices ?? [], local.devices, remote.devices)}),
    apply: (value) => {
      run.devices = value.devices
    },
  }
}

async function fetchRecord(run: Run, name: string): Promise<{rev: number; text: string} | null> {
  const {rev, body} = await api.readRecord(run.m, name)
  const sealed = fromB64(body)
  const plain = sealed && (await open(run.keys.data, sealed, recordAAD(run.m.chain, run.m.epoch, name, rev)))
  const padded = plain && unpad(plain)
  return padded ? {rev, text: decoder.decode(padded)} : null
}

async function sealRecord(keys: ChainKeys, chain: string, epoch: number, name: string, rev: number, text: string): Promise<string> {
  const plain = utf8(text)
  return toB64(await seal(keys.data, pad(plain, bucketFor(plain.length)), recordAAD(chain, epoch, name, rev)))
}

async function syncRecord<T>(run: Run, kind: Kind<T>, logical: string, name: string, remoteRev: number) {
  const {m} = run
  let rev = remoteRev
  for (let attempt = 0; attempt < 3; attempt++) {
    const base = run.bases[logical]
    const sameEpoch = base?.epoch === m.epoch
    if (sameEpoch && rev < base.rev) {
      run.report.problems.push('broken')
      return
    }
    let remoteText: string | null = null
    if (rev > 0) {
      if (sameEpoch && base.rev === rev) remoteText = base.text
      else {
        const fetched = await fetchRecord(run, name)
        if (!fetched) {
          run.report.problems.push('broken')
          return
        }
        rev = fetched.rev
        remoteText = fetched.text
      }
    }
    const remote = remoteText === null ? null : await kind.read(remoteText)
    if (remote && !remote.ok) {
      run.report.problems.push(remote.newer ? 'update' : 'broken')
      return
    }
    const baseRead = base ? await kind.read(base.text) : null
    const baseValue = baseRead?.ok ? baseRead.value : null
    const local = kind.local(remote ? baseValue : null)
    if (local === null) return
    const merged = remote ? kind.merge(baseValue, local, remote.value) : local
    const mergedText = kind.write(merged)
    if (mergedText !== kind.write(local) || logical === DEVICES) kind.apply(merged)
    if (mergedText === remoteText) {
      run.bases[logical] = {text: mergedText, rev, epoch: m.epoch}
      return
    }
    if (!remote && kind.empty?.(merged)) return
    try {
      const written = await api.writeRecord(m, name, m.epoch, rev, await sealRecord(run.keys, m.chain, m.epoch, name, rev + 1, mergedText))
      run.bases[logical] = {text: mergedText, rev: written.rev, epoch: m.epoch}
      return
    } catch (err) {
      if (!(err instanceof SyncHttpError) || err.status !== 409) throw err
      if (err.body.error === 'full') {
        run.report.problems.push('full')
        return
      }
      if (err.body.epoch !== m.epoch) throw new EpochChanged()
      rev = Number(err.body.rev)
    }
  }
}

async function state(m: Membership): Promise<api.ChainState> {
  try {
    return await api.chainState(m)
  } catch (err) {
    if (isStatus(err, 401)) {
      await forget()
      throw new Gone()
    }
    throw err
  }
}

// After another device removed someone: this device's new secret waits for it, sealed to its own key.
async function catchUp(m: Membership, epoch: number): Promise<Membership> {
  const wrap = await api.readWrap(m)
  const body = fromB64(wrap.body)
  const s = wrap.epoch === epoch && body ? await unwrapSecret(m.deviceKey, body, wrapInfo(m.chain, epoch, m.device)) : null
  if (!s) throw new SyncHttpError(409, {error: 'wrap'})
  const next = {...m, s, epoch}
  await setMembership(next)
  return next
}

function view(st: api.ChainState, devices: DeviceEntry[], self: string): DeviceView[] {
  return st.devices.map((d) => ({
    id: d.id,
    kind: d.kind,
    label: devices.find((e) => e.id === d.id)?.label ?? '',
    seen: d.seen,
    self: d.id === self,
  }))
}

export async function membership() {
  return getMembership()
}

export async function syncOnce(withSettings: boolean, label: string): Promise<Report> {
  for (let attempt = 0; ; attempt++) {
    try {
      return await syncPass(withSettings, label)
    } catch (err) {
      if (!(err instanceof EpochChanged) || attempt > 0) throw err
    }
  }
}

async function syncPass(withSettings: boolean, label: string): Promise<Report> {
  let m = await getMembership()
  if (!m) throw new Gone()
  let st = await state(m)
  if (st.epoch !== m.epoch) {
    m = await catchUp(m, st.epoch)
    st = await state(m)
  }
  const run: Run = {m, keys: await chainKeys(m.s), bases: await getBases(), report: {devices: [], dropped: 0, problems: []}, devices: []}
  const revs = new Map(st.records.map((r) => [r.name, r.rev]))
  const step =
    <T>(logical: string, kind: Kind<T>) =>
    async () => {
      const name = await recordName(run.keys.names, logical)
      await syncRecord(run, kind, logical, name, revs.get(name) ?? 0)
    }
  const steps = [
    step(DEVICES, devicesKind(run, label)),
    ...(withSettings ? [step(SETTINGS, settingsKind)] : []),
    ...syncedCities().flatMap((slug) => [step(cityName(slug), cityKind(slug, run)), step(newsName(slug), newsKind(slug))]),
  ]
  try {
    for (const sync of steps) await sync()
  } finally {
    await setBases(run.bases)
  }
  run.report.devices = view(st, run.devices, m.device)
  run.report.problems = [...new Set(run.report.problems)]
  return run.report
}

async function register(s: Bytes, send: (join: string) => Promise<api.Registered & {chain?: string}>, chain?: string) {
  const keys = await chainKeys(s)
  const deviceKey = await newKeyPair()
  const registered = await send(toB64(keys.join))
  await forget()
  await setMembership({
    chain: registered.chain ?? chain!,
    epoch: registered.epoch,
    device: registered.device,
    token: registered.token,
    s,
    deviceKey: deviceKey.privateKey,
    devicePub: toB64(deviceKey.spki),
  })
}

// Starts a chain with this device alone in it; the secret it returns goes to the other device.
export async function createChain(): Promise<{chain: string; epoch: number; s: string}> {
  const s = random(KEY_SIZE)
  await register(s, (join) => api.createChain(join, 'web'))
  const m = (await getMembership())!
  return {chain: m.chain, epoch: m.epoch, s: toB64(s)}
}

export async function joinChain(chain: string, s: string) {
  const secret = fromB64(s, KEY_SIZE)
  if (!secret) throw new SyncHttpError(400)
  await register(secret, (join) => api.joinChain(chain, join, 'web'), chain)
}

// Removes another device for good: every record moves to a new secret the removed device never gets (§7).
export async function removeDevice(id: string, withSettings: boolean, label: string) {
  await syncOnce(withSettings, label)
  const m = (await getMembership())!
  const st = await state(m)
  const keys = await chainKeys(m.s)
  const logicals = [DEVICES, SETTINGS, ...syncedCities().flatMap((slug) => [cityName(slug), newsName(slug)])]
  const byName = new Map(await Promise.all(logicals.map(async (l): Promise<[string, string]> => [await recordName(keys.names, l), l])))
  const run: Run = {m, keys, bases: {}, report: {devices: [], dropped: 0, problems: []}, devices: []}

  const texts = new Map<string, string>()
  for (const record of st.records) {
    const logical = byName.get(record.name)
    if (!logical) continue
    const fetched = await fetchRecord(run, record.name)
    if (!fetched || fetched.rev !== record.rev) throw new SyncHttpError(409, {error: 'stale'})
    texts.set(logical, fetched.text)
  }
  const devicesRead = texts.has(DEVICES) ? await readDevices(texts.get(DEVICES)!) : null
  if (!devicesRead?.ok) throw new SyncHttpError(409, {error: 'devices'})
  const entries = devicesRead.value.devices
  texts.set(DEVICES, writeDevices({devices: entries.filter((d) => d.id !== id)}))
  const remaining = st.devices.filter((d) => d.id !== id && d.id !== m.device)
  const pubs = remaining.map((d) => entries.find((e) => e.id === d.id)?.pub)
  if (pubs.some((p) => !p)) throw new SyncHttpError(409, {error: 'devices'})

  const s = random(KEY_SIZE)
  const next = await chainKeys(s)
  const epoch = st.epoch + 1
  const bases: Record<string, Base> = {}
  const records = await Promise.all(
    [...texts].map(async ([logical, text]) => {
      const name = await recordName(next.names, logical)
      bases[logical] = {text, rev: 1, epoch}
      return {name, body: await sealRecord(next, m.chain, epoch, name, 1, text)}
    }),
  )
  const wraps = await Promise.all(
    remaining.map(async (d, i) => ({
      device: d.id,
      body: toB64(await wrapSecret(fromB64(pubs[i]!)!, s, wrapInfo(m.chain, epoch, d.id))),
    })),
  )
  await api.rotateChain(m, {epoch: st.epoch, seen: st.records, join: toB64(next.join), remove: [id], records, wraps})
  await setMembership({...m, s, epoch})
  await setBases({...(await getBases()), ...bases})
}

// Leaves the chain; the data on this device stays as it is.
export async function leave() {
  const m = await getMembership()
  if (!m) return
  try {
    const keys = await chainKeys(m.s)
    const name = await recordName(keys.names, DEVICES)
    const run: Run = {m, keys, bases: {}, report: {devices: [], dropped: 0, problems: []}, devices: []}
    const fetched = await fetchRecord(run, name)
    const read = fetched && (await readDevices(fetched.text))
    if (fetched && read?.ok) {
      const text = writeDevices({devices: read.value.devices.filter((d) => d.id !== m.device)})
      await api.writeRecord(m, name, m.epoch, fetched.rev, await sealRecord(keys, m.chain, m.epoch, name, fetched.rev + 1, text))
    }
    await api.leaveChain(m)
  } catch (err) {
    if (!isStatus(err, 401) && !isStatus(err, 404)) throw err
  }
  await forget()
}

export async function deleteEverything() {
  const m = await getMembership()
  if (!m) return
  try {
    await api.deleteChain(m)
  } catch (err) {
    if (!isStatus(err, 401)) throw err
  }
  await forget()
}
