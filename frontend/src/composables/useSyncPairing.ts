import {onScopeDispose, ref, shallowRef} from 'vue'
import * as api from '@/utils/sync/api.ts'
import {isStatus, type Slot} from '@/utils/sync/api.ts'
import {
  type Bytes,
  checkNumber,
  ecdh,
  fromB64,
  type KeyPair,
  newKeyPair,
  open,
  pad,
  PAIR_PADDED,
  pairAAD,
  pairKey,
  random,
  sameBytes,
  seal,
  sha256,
  SPKI_SIZE,
  toB64,
  unpad,
  utf8,
  validPublicKey,
} from '@/utils/sync/crypto.ts'
import {type Hello, type Message, readMessage, writeMessage} from '@/utils/sync/messages.ts'
import {deviceLabel, type OverCap, overCaps} from '@/utils/sync/local.ts'
import * as engine from '@/utils/sync/engine.ts'
import {useSyncStore} from '@/stores/sync'

const POLL_MS = 2000

export type PairPhase = 'idle' | 'starting' | 'showing' | 'connecting' | 'confirm' | 'waiting' | 'done' | 'failed'
export type PairFailure = 'expired' | 'unknown' | 'limited' | 'broken' | 'both' | 'over-cap' | 'offline'

type Pairing = {
  role: 'D' | 'E'
  slot: Slot
  keys: KeyPair
  k: Bytes | null
  commit?: Bytes
  t?: Bytes
  read: number
  mine?: Hello
  giving: boolean
  confirmed: boolean
  peerReady: boolean
  sentSecret: boolean
}

export const pairLink = (code: string, k: Bytes) => `${window.location.origin}/settings?pair=${code}#k=${toB64(k)}`

// Pairing of protocol.md §2: this device either shows a code (D) or enters one (E).
export function useSyncPairing() {
  const sync = useSyncStore()
  const phase = ref<PairPhase>('idle')
  const failure = ref<PairFailure | null>(null)
  const overCap = shallowRef<OverCap[]>([])
  const code = ref('')
  const url = ref('')
  const check = ref('')
  const peer = shallowRef<Hello | null>(null)
  let pairing: Pairing | null = null
  let timer: ReturnType<typeof setTimeout> | undefined
  let polling = false

  function stopTimer() {
    if (timer) clearTimeout(timer)
    timer = undefined
  }

  function reset() {
    stopTimer()
    pairing = null
    failure.value = null
    peer.value = null
    code.value = url.value = check.value = ''
  }

  function fail(why: PairFailure) {
    const current = pairing
    stopTimer()
    if (current) void api.closeSlot(current.slot)
    pairing = null
    failure.value = why
    phase.value = 'failed'
  }

  function failFrom(err: unknown) {
    if (isStatus(err, 404)) fail('unknown')
    else if (isStatus(err, 429)) fail('limited')
    else if (err instanceof TypeError) fail('offline')
    else fail('broken')
  }

  function precheck(): boolean {
    overCap.value = overCaps()
    if (overCap.value.length) fail('over-cap')
    return !overCap.value.length
  }

  async function show() {
    reset()
    if (!precheck()) return
    phase.value = 'starting'
    try {
      const keys = await newKeyPair()
      const k = random(16)
      const slot = await api.openSlot(toB64(await sha256(keys.spki)))
      pairing = {role: 'D', slot, keys, k, read: 0, giving: false, confirmed: false, peerReady: false, sentSecret: false}
      code.value = slot.code
      url.value = pairLink(slot.code, k)
      phase.value = 'showing'
      schedulePoll()
    } catch (err) {
      failFrom(err)
    }
  }

  async function enter(entered: string, key?: string) {
    reset()
    if (!precheck()) return
    const k = key ? fromB64(key, 16) : null
    phase.value = 'connecting'
    try {
      const keys = await newKeyPair()
      const claimed = await api.claimSlot(entered, toB64(keys.spki), !!k)
      const commit = fromB64(claimed.commit, 32)
      if (!commit) return fail('broken')
      pairing = {
        role: 'E',
        slot: {code: entered, secret: claimed.secret},
        keys,
        k,
        commit,
        read: 0,
        giving: false,
        confirmed: false,
        peerReady: false,
        sentSecret: false,
      }
      code.value = entered
      schedulePoll(0)
    } catch (err) {
      failFrom(err)
    }
  }

  function schedulePoll(delay = POLL_MS) {
    stopTimer()
    timer = setTimeout(() => void poll(), delay)
  }

  async function send(p: Pairing, message: Message) {
    const sealed = await seal(p.t!, pad(utf8(writeMessage(message)), PAIR_PADDED), pairAAD(p.slot.code, p.role))
    await api.sendMessage(p.slot, toB64(sealed))
  }

  async function openChannel(p: Pairing, other: Bytes, scanned: boolean) {
    const [pe, pd] = p.role === 'D' ? [other, p.keys.spki] : [p.keys.spki, other]
    p.t = await pairKey(await ecdh(p.keys.privateKey, other), scanned ? p.k : null, p.slot.code, pe, pd)
    check.value = await checkNumber(p.t)
    const member = await engine.membership()
    p.mine = {t: 'hello', kind: 'web', label: deviceLabel(), syncs: !!member, devices: member ? Math.max(sync.devices.length, 1) : 0}
    phase.value = 'connecting'
    await send(p, p.mine)
  }

  async function poll() {
    const p = pairing
    if (!p || polling) return
    polling = true
    try {
      let read: api.SlotRead
      try {
        read = await api.readSlot(p.slot)
      } catch (err) {
        if (!isStatus(err, 404)) throw err
        if (p.sentSecret) finish()
        else fail(p.t ? 'broken' : 'expired')
        return
      }
      if (!p.t && read.pub) {
        const other = fromB64(read.pub, SPKI_SIZE)
        if (!other || !(await validPublicKey(other))) return fail('broken')
        if (p.role === 'D') await api.revealKey(p.slot, toB64(p.keys.spki))
        else if (!sameBytes(await sha256(other), p.commit!)) return fail('broken')
        await openChannel(p, other, p.role === 'D' ? read.scanned : !!p.k)
      }
      for (; p.t && p.read < read.messages.length; p.read++) {
        const sealed = fromB64(read.messages[p.read]!)
        const plain = sealed && (await open(p.t, sealed, pairAAD(p.slot.code, p.role === 'D' ? 'E' : 'D')))
        const padded = plain && unpad(plain)
        const message = padded && readMessage(new TextDecoder().decode(padded))
        if (!message) return fail('broken')
        await handle(p, message)
        if (pairing !== p) return
      }
    } catch (err) {
      if (err instanceof TypeError) return schedulePoll()
      return failFrom(err)
    } finally {
      polling = false
    }
    if (pairing === p && phase.value !== 'done') schedulePoll()
  }

  async function handle(p: Pairing, message: Message) {
    if (message.t === 'hello') {
      if (peer.value) return fail('broken')
      peer.value = message
      if (p.mine!.syncs && message.syncs) return fail('both')
      p.giving = p.mine!.syncs || (!message.syncs && p.role === 'E')
      phase.value = 'confirm'
    } else if (message.t === 'ready') {
      if (!p.giving || p.peerReady) return fail('broken')
      p.peerReady = true
      await give(p)
    } else {
      if (p.giving || !p.confirmed) return fail('broken')
      await engine.joinChain(message.chain, message.s)
      void api.closeSlot(p.slot)
      await sync.joined()
      finish()
    }
  }

  async function give(p: Pairing) {
    if (!p.giving || !p.confirmed || !p.peerReady || p.sentSecret) return
    p.sentSecret = true
    const member = await engine.membership()
    let secret: {chain: string; epoch: number; s: string}
    if (member) secret = {chain: member.chain, epoch: member.epoch, s: toB64(member.s)}
    else {
      secret = await engine.createChain()
      await sync.joined()
    }
    await send(p, {t: 'secret', ...secret})
  }

  async function confirm() {
    const p = pairing
    if (!p || phase.value !== 'confirm') return
    p.confirmed = true
    phase.value = 'waiting'
    try {
      if (p.giving) await give(p)
      else await send(p, {t: 'ready'})
    } catch (err) {
      failFrom(err)
    }
  }

  function finish() {
    stopTimer()
    pairing = null
    phase.value = 'done'
    void sync.syncNow()
  }

  function cancel() {
    if (pairing) void api.closeSlot(pairing.slot)
    reset()
    phase.value = 'idle'
  }

  // Timers stop while the screen is off, so look again as soon as it is back.
  function onVisible() {
    if (document.visibilityState === 'visible' && pairing) void poll()
  }
  document.addEventListener('visibilitychange', onVisible)

  onScopeDispose(() => {
    document.removeEventListener('visibilitychange', onVisible)
    cancel()
  })

  return {phase, failure, overCap, code, url, check, peer, show, enter, confirm, cancel}
}
