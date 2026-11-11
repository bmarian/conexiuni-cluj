import {ref} from 'vue'
import {readSaved} from '@/utils/storage.ts'
import {type HintContext, type HintId, type HintLine, markShown, openLineFromStop, pickHint, readHints, visitStop} from '@/utils/hints.ts'

const KEY = 'hints'

const state = ref(readHints(readSaved(KEY)))

window.addEventListener('storage', (e) => {
  if (e.key === KEY) state.value = readHints(readSaved(KEY))
})

function save(next: typeof state.value) {
  state.value = next
  localStorage.setItem(KEY, JSON.stringify(next))
}

const pad = (n: number) => String(n).padStart(2, '0')
export const dayOf = (at: Date) => `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())}`

export function noteStopVisit(stop: string, at = new Date()) {
  save(visitStop(state.value, stop, dayOf(at), at.getHours()))
}

export function noteLineFromStop(stop: string, line: HintLine, at = new Date()) {
  save(openLineFromStop(state.value, stop, line, dayOf(at)))
}

export const nextHint = (ctx: Omit<HintContext, 'day' | 'hour'>, at = new Date()) => pickHint(state.value, {...ctx, day: dayOf(at), hour: at.getHours()})

export function hintShown(id: HintId, at = new Date()) {
  save(markShown(state.value, id, dayOf(at)))
}
