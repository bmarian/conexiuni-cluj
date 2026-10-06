import {computed, onScopeDispose, ref, shallowRef} from 'vue'
import {
  createTransferKey,
  createTransferLink,
  deleteTransferLink,
  openTransfer,
  readTransferLink,
  type TransferLink,
  transferLinkUrl,
} from '@/utils/transferLink.ts'

const POLL_MS = 2000

// Shows a code, waits for another device to send its export to it, and holds what
// arrived until the person confirms it: anyone who guesses a waiting code can send too.
// `describe` returns null for anything that isn't an export.
export function useTransferReceiver<Summary>(
  describe: (data: unknown) => Summary | null,
  apply: (data: unknown) => void,
) {
  const state = ref<'idle' | 'loading' | 'waiting' | 'review' | 'done' | 'expired' | 'failed' | 'broken'>('idle')
  const link = shallowRef<TransferLink | null>(null)
  const summary = shallowRef<Summary | null>(null)
  let key: string | null = null
  let received: unknown = null
  let expiresAt = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let polling = false

  const url = computed(() => link.value ? transferLinkUrl(link.value.code, key) : '')
  const spacedCode = computed(() => link.value ? `${link.value.code.slice(0, 3)} ${link.value.code.slice(3)}` : '')

  function clearTimer() {
    if (timer) clearTimeout(timer)
    timer = undefined
  }

  function release() {
    if (link.value) deleteTransferLink(link.value)
    link.value = null
    summary.value = null
    received = null
  }

  async function start() {
    clearTimer()
    release()
    document.addEventListener('visibilitychange', onVisible)
    state.value = 'loading'
    try {
      const created = await createTransferLink()
      if (state.value !== 'loading') {
        deleteTransferLink(created)
        return
      }
      key = createTransferKey()
      link.value = created
      expiresAt = Date.now() + created.expiresIn * 1000
      state.value = 'waiting'
      timer = setTimeout(poll, POLL_MS)
    } catch {
      state.value = 'failed'
    }
  }

  function stop() {
    clearTimer()
    document.removeEventListener('visibilitychange', onVisible)
    release()
    state.value = 'idle'
  }

  async function poll() {
    clearTimer()
    const current = link.value
    if (!current || state.value !== 'waiting' || polling) return
    if (Date.now() > expiresAt) {
      state.value = 'expired'
      return
    }
    polling = true
    try {
      const result = await readTransferLink(current)
      if (link.value !== current) return
      if (result.state === 'gone') state.value = 'expired'
      else if (result.state === 'received') await receive(current, result.data)
    } catch {
      // Offline for a moment; the next poll tries again.
    } finally {
      polling = false
    }
    if (state.value === 'waiting') timer = setTimeout(poll, POLL_MS)
  }

  async function receive(from: TransferLink, payload: unknown) {
    deleteTransferLink(from)
    link.value = null
    try {
      received = await openTransfer(payload, key)
      summary.value = describe(received)
    } catch {
      summary.value = null
    }
    state.value = summary.value ? 'review' : 'broken'
  }

  function accept() {
    if (state.value !== 'review') return
    try {
      apply(received)
      state.value = 'done'
    } catch {
      state.value = 'broken'
    }
  }

  // Timers stop while the screen is off or the tab is hidden, so check as soon as it's back.
  function onVisible() {
    if (document.visibilityState === 'visible' && state.value === 'waiting') void poll()
  }

  onScopeDispose(stop)

  return {state, url, spacedCode, summary, start, stop, accept}
}
