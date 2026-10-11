import {ref} from 'vue'
import {defineStore} from 'pinia'
import {
  MAX_FOLLOWS, clockAt, followId, freshProgress, isClockTime, leaveInMinutes, stepFollow,
  type FollowEnd, type FollowProgress, type FollowReading, type Followed,
} from '@/utils/follow.ts'
import {askNotifications, clearNotice, notify} from '@/utils/notify.ts'
import {translate as t} from '@/i18n.ts'
import {useSettingsStore} from '@/stores/settings.ts'

const STORAGE_KEY = 'follows.v1'

export interface FollowLive {
  minutes: number
  live: boolean
  later: number[]
  leaveAt: number | null
  walk: number | null
}

function load(): Followed[] {
  try {
    const list = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]')
    return Array.isArray(list) ? list.filter((f) => f && typeof f.id === 'string' && typeof f.tripId === 'string') : []
  } catch {
    return []
  }
}

export const minutesText = (minutes: number, nowMs: number) =>
  isClockTime(minutes) ? clockAt(nowMs, minutes) : t('etaMinutes', {n: minutes})

export const busWhen = (minutes: number, nowMs: number) =>
  isClockTime(minutes) ? t('followAtClock', {time: clockAt(nowMs, minutes)}) : t('followIn', {time: t('etaMinutes', {n: minutes})})

export function leaveText(leaveAt: number, nowMs: number): string {
  const leave = leaveInMinutes(leaveAt, nowMs)
  if (leave <= 0) return t('leaveNow')
  return isClockTime(leave) ? t('leaveAt', {time: clockAt(nowMs, leave)}) : t('leaveIn', {time: t('etaMinutes', {n: leave})})
}

export const useFollowsStore = defineStore('follows', () => {
  const settings = useSettingsStore()
  const follows = ref<Followed[]>(load())
  const live = ref<Record<string, FollowLive>>({})
  const progress: Record<string, FollowProgress> = {}

  function persist() {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(follows.value))
    } catch {}
  }

  function find(stopId: number, routeId: number, tripId: string): Followed | undefined {
    return follows.value.find((f) => f.stopId === stopId && f.routeId === routeId && f.tripId === tripId)
  }

  function remove(id: string) {
    follows.value = follows.value.filter((f) => f.id !== id)
    delete live.value[id]
    delete progress[id]
    persist()
    void clearNotice(`follow-${id}`)
  }

  async function start(followed: Omit<Followed, 'id' | 'startedAt'>) {
    const id = followId(followed.stopId, followed.routeId, followed.tripId, followed.targetAt)
    const others = follows.value.filter((f) => !(f.stopId === followed.stopId && f.routeId === followed.routeId && f.tripId === followed.tripId))
    if (others.length >= MAX_FOLLOWS) {
      settings.showToast(t('limitReached', {n: MAX_FOLLOWS}), {icon: 'bell-off'})
      return
    }
    follows.value = [...others, {...followed, id, startedAt: Date.now()}]
    progress[id] = freshProgress()
    persist()
    const permission = await askNotifications()
    if (permission === 'denied') settings.showToast(t('notificationsBlocked'), {icon: 'bell-off'})
  }

  function toggle(followed: Omit<Followed, 'id' | 'startedAt'>) {
    const existing = find(followed.stopId, followed.routeId, followed.tripId)
    if (existing) remove(existing.id)
    else void start(followed)
  }

  function end(followed: Followed, reason: FollowEnd) {
    remove(followed.id)
    const line = `${followed.line} → ${followed.destination}`
    if (reason === 'left') {
      void notify({title: t('followDone', {stop: followed.stopName}), body: line, tag: `follow-${followed.id}`, url: `/stop/${followed.stopId}`})
    } else if (reason === 'none') {
      void notify({title: t('followNoBus'), body: line, tag: `follow-${followed.id}`, url: `/stop/${followed.stopId}`})
      settings.showToast(t('followNoBus'), {body: line, icon: 'bell-off'})
    }
  }

  function onReading(id: string, reading: FollowReading | null, walk: number | null, nowMs = Date.now()) {
    const followed = follows.value.find((f) => f.id === id)
    if (!followed) return
    const step = stepFollow(progress[id] ?? freshProgress(), reading, {
      target: followed.targetAt !== undefined, walk, nowMs, startedAt: followed.startedAt,
    })
    progress[id] = step.progress
    if (step.end) return end(followed, step.end)
    if (!reading) return
    live.value = {...live.value, [id]: {minutes: reading.minutes, live: reading.live, later: reading.later, leaveAt: step.leaveAt, walk}}
    const title = `${followed.line} → ${followed.destination}`
    const url = `/stop/${followed.stopId}`
    for (const alert of step.alerts) {
      if (alert === 'leave') {
        const bus = busWhen(reading.minutes, nowMs)
        const body = walk ? t('leaveDetail', {line: followed.line, bus, walk: t('etaMinutes', {n: walk})}) : `${followed.line} ${bus}`
        void notify({title: t('leaveNow'), body, tag: `follow-${id}`, url, buzz: true})
      } else if (alert === 'soon') {
        void notify({title, body: `${t('followArrivingStop', {stop: followed.stopName})} ${busWhen(reading.minutes, nowMs)}`, tag: `follow-${id}`, url, buzz: true})
      } else {
        void notify({title, body: t('followNowStop', {stop: followed.stopName}), tag: `follow-${id}`, url, buzz: true})
      }
    }
  }

  return {follows, live, find, start, toggle, remove, onReading}
})
