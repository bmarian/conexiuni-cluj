import {computed, onScopeDispose, ref, shallowRef} from 'vue'
import {
  createTransferLink,
  deleteTransferLink,
  readTransferLink,
  type TransferLink,
  transferLinkUrl,
} from '@/utils/transferLink.ts'

const POLL_MS = 2000

// Shows a code and waits for another device to send its export to it.
export function useTransferReceiver(apply: (data: unknown) => void) {
  const state = ref<'idle' | 'loading' | 'waiting' | 'done' | 'expired' | 'failed'>('idle')
  const link = shallowRef<TransferLink | null>(null)
  let expiresAt = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let polling = false

  const url = computed(() => link.value ? transferLinkUrl(link.value.code) : '')
  const spacedCode = computed(() => link.value ? `${link.value.code.slice(0, 3)} ${link.value.code.slice(3)}` : '')

  function clearTimer() {
    if (timer) clearTimeout(timer)
    timer = undefined
  }

  function release() {
    if (link.value && state.value !== 'done') deleteTransferLink(link.value)
    link.value = null
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
      else if (result.state === 'received') receive(current, result.data)
    } catch {
      // Offline for a moment; the next poll tries again.
    } finally {
      polling = false
    }
    if (state.value === 'waiting') timer = setTimeout(poll, POLL_MS)
  }

  function receive(from: TransferLink, data: unknown) {
    try {
      apply(data)
      state.value = 'done'
    } catch {
      state.value = 'failed'
    }
    deleteTransferLink(from)
  }

  // Timers stop while the screen is off or the tab is hidden, so check as soon as it's back.
  function onVisible() {
    if (document.visibilityState === 'visible' && state.value === 'waiting') void poll()
  }

  onScopeDispose(stop)

  return {state, url, spacedCode, start, stop}
}
