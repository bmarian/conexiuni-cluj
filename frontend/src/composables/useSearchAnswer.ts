import {onUnmounted, ref, watch, type Ref} from 'vue'
import {searchEverything, type SearchAnswer} from '@/utils/search.ts'

const DEBOUNCE_MS = 350
// Shorter queries are matched on the device alone.
const MIN_CHARS = 3

export function useSearchAnswer(query: Ref<string>, isOnline: Ref<boolean>) {
  const answer = ref<SearchAnswer | null>(null)
  const loading = ref(false)
  let timer: ReturnType<typeof setTimeout> | null = null
  let request: AbortController | null = null

  async function ask(q: string) {
    const current = new AbortController()
    request = current
    const found = await searchEverything(q, current.signal)
    if (current.signal.aborted) return
    answer.value = found
    if (request === current) {
      request = null
      loading.value = false
    }
  }

  watch(query, (q) => {
    if (timer) clearTimeout(timer)
    request?.abort()
    request = null
    answer.value = null
    const trimmed = q.trim()
    loading.value = isOnline.value && trimmed.length >= MIN_CHARS
    if (loading.value) timer = setTimeout(() => ask(trimmed), DEBOUNCE_MS)
  })

  onUnmounted(() => {
    if (timer) clearTimeout(timer)
    request?.abort()
  })

  return {answer, loading}
}
