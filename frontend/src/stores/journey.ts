import {computed, ref, shallowRef} from 'vue'
import {defineStore} from 'pinia'
import {
  JOURNEY_GIVE_UP_MS, startRun, stepJourney,
  type Fix, type Journey, type JourneyRun, type JourneyStep, type LegDeparture,
} from '@/utils/journey.ts'
import {stepDetail, stepTitle} from '@/utils/journeyText.ts'
import {askNotifications, clearNotice, notify} from '@/utils/notify.ts'
import {translate as t} from '@/i18n.ts'

const STORAGE_KEY = 'journey.v1'
const NOTICE_TAG = 'journey'

interface Saved {
  journey: Journey
  run: JourneyRun
}

function load(): Saved | null {
  try {
    const saved = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? 'null') as Saved | null
    if (!saved?.journey?.legs?.length || !saved.run?.phase) return null
    if (Date.now() - saved.journey.startedAt > JOURNEY_GIVE_UP_MS) return null
    return saved
  } catch {
    return null
  }
}

export const useJourneyStore = defineStore('journey', () => {
  const saved = load()
  const journey = shallowRef<Journey | null>(saved?.journey ?? null)
  const run = ref<JourneyRun | null>(saved?.run ?? null)
  const step = shallowRef<JourneyStep | null>(null)
  const skips = ref(0)

  const legIndex = computed(() => {
    const phase = run.value?.phase
    return phase && 'leg' in phase ? phase.leg : -1
  })
  const tripIds = computed(() => (legIndex.value >= 0 ? journey.value?.legs[legIndex.value]?.tripIds ?? [] : []))

  function persist() {
    try {
      if (journey.value && run.value) localStorage.setItem(STORAGE_KEY, JSON.stringify({journey: journey.value, run: run.value}))
      else localStorage.removeItem(STORAGE_KEY)
    } catch {}
  }

  async function start(next: Journey, fix: Fix | null) {
    journey.value = next
    run.value = startRun(next, fix)
    step.value = null
    skips.value = 0
    persist()
    await askNotifications()
  }

  function stop() {
    journey.value = null
    run.value = null
    step.value = null
    persist()
    void clearNotice(NOTICE_TAG)
  }

  function next() {
    skips.value++
  }

  function onTick(fix: Fix | null, departure: LegDeparture | null, nowMs: number, skip: boolean) {
    const current = journey.value
    const state = run.value
    if (!current || !state) return
    if (nowMs - current.startedAt > JOURNEY_GIVE_UP_MS) return stop()
    const result = stepJourney(current, state, fix, departure, nowMs, skip)
    step.value = result.step
    persist()
    const url = '/nav'
    if (result.done) {
      stop()
      void notify({title: t('journeyArrived'), body: current.destination.name, tag: NOTICE_TAG, url: '/', buzz: true})
      return
    }
    if (result.changed) {
      void notify({
        title: stepTitle(result.step),
        body: stepDetail(result.step, nowMs) ?? undefined,
        tag: NOTICE_TAG,
        url,
        buzz: result.alerted,
        silent: !result.alerted,
      })
    }
  }

  return {journey, run, step, skips, legIndex, tripIds, start, stop, next, onTick}
})
