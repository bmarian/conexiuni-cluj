import {computed, readonly, ref} from 'vue'
import {readSaved} from '@/utils/storage.ts'

export type FirstStep = 'plan' | 'stop' | 'line'

export const FIRST_STEPS: readonly FirstStep[] = ['plan', 'stop', 'line']

const KEY = 'home.firstSteps'

type FirstStepsState = {on: boolean; done: FirstStep[]}

const isStep = (s: unknown): s is FirstStep => FIRST_STEPS.includes(s as FirstStep)

function read(): FirstStepsState | null {
  const saved = readSaved(KEY) as Partial<FirstStepsState> | null
  if (!saved || typeof saved.on !== 'boolean') return null
  return {on: saved.on, done: Array.isArray(saved.done) ? saved.done.filter(isStep) : []}
}

const state = ref<FirstStepsState | null>(read())

window.addEventListener('storage', (e) => {
  if (e.key === KEY) state.value = read()
})

function write(next: FirstStepsState) {
  state.value = next
  localStorage.setItem(KEY, JSON.stringify(next))
}

// Decided once per device: only someone who starts with nothing saved gets the card.
export function startFirstSteps(fresh: boolean) {
  if (!state.value) write({on: fresh, done: []})
}

export function markFirstStep(step: FirstStep) {
  const s = state.value
  if (s?.on && !s.done.includes(step)) write({...s, done: [...s.done, step]})
}

export function hideFirstSteps() {
  if (state.value?.on) write({...state.value, on: false})
}

export function useFirstSteps() {
  const left = computed<FirstStep[]>(() => {
    const s = state.value
    return s?.on ? FIRST_STEPS.filter((step) => !s.done.includes(step)) : []
  })
  const done = computed(() => FIRST_STEPS.length - left.value.length)
  return {state: readonly(state), left, done, total: FIRST_STEPS.length}
}
